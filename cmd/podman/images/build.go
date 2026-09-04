package images

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	buildahCLI "go.podman.io/buildah/pkg/cli"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/storage/pkg/archive"
)

var (
	// Command: podman _diff_ Object_ID
	buildDescription = "Builds an OCI or Docker image using instructions from one or more Containerfiles and a specified build context directory."
	buildCmd         = &cobra.Command{
		Use:               "build [options] [CONTEXT]",
		Short:             "Build an image using instructions from Containerfiles",
		Long:              buildDescription,
		Args:              cobra.MaximumNArgs(1),
		RunE:              build,
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman build .
podman build --creds=username:password -t imageName -f Containerfile.simple .
podman build --layers --force-rm --tag imageName .`,
	}

	imageBuildCmd = &cobra.Command{
		Args:              buildCmd.Args,
		Use:               buildCmd.Use,
		Short:             buildCmd.Short,
		Long:              buildCmd.Long,
		RunE:              buildCmd.RunE,
		ValidArgsFunction: buildCmd.ValidArgsFunction,
		Example: `podman image build .
podman image build --creds=username:password -t imageName -f Containerfile.simple .
podman image build --layers --force-rm --tag imageName .`,
	}

	buildxBuildCmd = &cobra.Command{
		Args:              buildCmd.Args,
		Use:               buildCmd.Use,
		Short:             buildCmd.Short,
		Long:              buildCmd.Long,
		RunE:              buildCmd.RunE,
		ValidArgsFunction: buildCmd.ValidArgsFunction,
		Example: `podman buildx build .
podman buildx build --creds=username:password -t imageName -f Containerfile.simple .
podman buildx build --layers --force-rm --tag imageName .`,
	}

	buildOpts = common.BuildFlagsWrapper{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildCmd,
	})
	buildFlags(buildCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageBuildCmd,
		Parent:  imageCmd,
	})
	buildFlags(imageBuildCmd)
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildxBuildCmd,
		Parent:  buildxCmd,
	})
	buildFlags(buildxBuildCmd)
}

func buildFlags(cmd *cobra.Command) {
	common.DefineBuildFlags(cmd, &buildOpts, false)
}

// build executes the build command.
func build(cmd *cobra.Command, args []string) error {
	apiBuildOpts, err := common.ParseBuildOpts(cmd, args, &buildOpts)
	if err != nil {
		return err
	}
	// Close the logFile if one was created based on the flag
	if apiBuildOpts.LogFileToClose != nil {
		defer apiBuildOpts.LogFileToClose.Close()
	}
	if apiBuildOpts.TmpDirToClose != "" {
		// We had to download the context directory.
		// Delete it later.
		defer func() {
			if err = os.RemoveAll(apiBuildOpts.TmpDirToClose); err != nil {
				logrus.Errorf("Removing temporary directory %q: %v", apiBuildOpts.ContextDirectory, err)
			}
		}()
	}
	if len(apiBuildOpts.BuildOutputs) > 0 && registry.IsRemote() {
		for _, buildOutput := range apiBuildOpts.BuildOutputs {
			if _, err := getBuildOutput(buildOutput); err != nil {
				registry.SetExitCode(125)
				return err
			}
		}
	}
	report, err := registry.ImageEngine().Build(registry.Context(), apiBuildOpts.ContainerFiles, *apiBuildOpts)
	if err != nil {
		exitCode := buildahCLI.ExecErrorCodeGeneric
		if registry.IsRemote() {
			// errors from server does not contain ExitCode
			// so parse exit code from error message
			remoteExitCode, parseErr := utils.ExitCodeFromBuildError(err.Error())
			if parseErr == nil {
				exitCode = remoteExitCode
			}
		}

		exitError := &exec.ExitError{}
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}

		registry.SetExitCode(exitCode)
		return err
	}

	if len(apiBuildOpts.BuildOutputs) > 0 && registry.IsRemote() {
		if err := processBuildOutputs(registry.Context(), report.ID, apiBuildOpts.BuildOutputs); err != nil {
			registry.SetExitCode(125)
			return err
		}
	}

	if cmd.Flag("iidfile").Changed {
		if err := os.WriteFile(buildOpts.Iidfile, []byte("sha256:"+report.ID), 0o644); err != nil {
			return err
		}
	}
	if cmd.Flag("iidfile-raw").Changed {
		if err := os.WriteFile(buildOpts.IidfileRaw, []byte(report.ID), 0o644); err != nil {
			return err
		}
	}

	return nil
}

type BuildOutputType int

const (
	BuildOutputInvalid  BuildOutputType = 0
	BuildOutputStdout   BuildOutputType = 1 // stream tar to stdout
	BuildOutputLocalDir BuildOutputType = 2
	BuildOutputTar      BuildOutputType = 3
)

type BuildOutputOption struct {
	Type BuildOutputType
	Path string
}

func getBuildOutput(buildOutput string) (BuildOutputOption, error) {
	if !strings.Contains(buildOutput, ",") && !strings.Contains(buildOutput, "=") {
		if buildOutput == "-" {
			return BuildOutputOption{Type: BuildOutputStdout}, nil
		}
		return BuildOutputOption{Type: BuildOutputLocalDir, Path: buildOutput}, nil
	}

	typeSelected := BuildOutputInvalid
	pathSelected := ""
	for option := range strings.SplitSeq(buildOutput, ",") {
		key, value, found := strings.Cut(option, "=")
		if !found {
			return BuildOutputOption{}, fmt.Errorf("invalid build output options %q, expected format key=value", buildOutput)
		}
		switch key {
		case "type":
			if typeSelected != BuildOutputInvalid {
				return BuildOutputOption{}, fmt.Errorf("duplicate %q not supported", key)
			}
			switch value {
			case "local":
				typeSelected = BuildOutputLocalDir
			case "tar":
				typeSelected = BuildOutputTar
			default:
				return BuildOutputOption{}, fmt.Errorf("invalid type %q selected for build output options %q", value, buildOutput)
			}
		case "dest":
			if pathSelected != "" {
				return BuildOutputOption{}, fmt.Errorf("duplicate %q not supported", key)
			}
			pathSelected = value
		default:
			return BuildOutputOption{}, fmt.Errorf("unrecognized key %q in build output option: %q", key, buildOutput)
		}
	}
	if typeSelected == BuildOutputInvalid {
		return BuildOutputOption{}, fmt.Errorf("missing required key %q in build output option: %q", "type", buildOutput)
	}
	if typeSelected == BuildOutputLocalDir || typeSelected == BuildOutputTar {
		if pathSelected == "" {
			return BuildOutputOption{}, fmt.Errorf("missing required key %q in build output option: %q", "dest", buildOutput)
		}
	} else {
		pathSelected = ""
	}
	if pathSelected == "-" {
		if typeSelected == BuildOutputTar {
			typeSelected = BuildOutputStdout
			pathSelected = ""
		} else {
			return BuildOutputOption{}, fmt.Errorf("invalid build output option %q, only 'type=tar' can be used with 'dest=-'", buildOutput)
		}
	}
	return BuildOutputOption{Type: typeSelected, Path: pathSelected}, nil
}

func makeTarDirsWritable(r io.Reader) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		tr := tar.NewReader(r)
		tw := tar.NewWriter(pw)
		defer tw.Close()
		for {
			hdr, err := tr.Next()
			if err != nil {
				return
			}
			if hdr.Typeflag == tar.TypeDir {
				hdr.Mode |= 0o200
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return
			}
			if _, err := io.Copy(tw, tr); err != nil {
				return
			}
		}
	}()
	return pr
}

func processBuildOutputs(ctx context.Context, imageID string, buildOutputs []string) error {
	for _, buildOutput := range buildOutputs {
		buildOutputOption, err := getBuildOutput(buildOutput)
		if err != nil {
			return err
		}

		s := specgen.NewSpecGenerator(imageID, false)
		s.Command = []string{"/bin/sh"}
		createResponse, err := registry.ContainerEngine().ContainerCreate(ctx, s)
		if err != nil {
			return fmt.Errorf("failed to create temporary container for output: %w", err)
		}

		defer func(id string) {
			_, _ = registry.ContainerEngine().ContainerRm(ctx, []string{id}, entities.RmOptions{Force: true})
		}(createResponse.Id)

		switch buildOutputOption.Type {
		case BuildOutputStdout:
			if err := registry.ContainerEngine().ContainerExport(ctx, createResponse.Id, entities.ContainerExportOptions{Output: os.Stdout}); err != nil {
				return fmt.Errorf("failed to export temporary container: %w", err)
			}
		case BuildOutputTar:
			outFile, err := os.Create(buildOutputOption.Path)
			if err != nil {
				return err
			}
			if err := registry.ContainerEngine().ContainerExport(ctx, createResponse.Id, entities.ContainerExportOptions{Output: outFile}); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to export temporary container: %w", err)
			}
			outFile.Close()
		case BuildOutputLocalDir:
			pr, pw := io.Pipe()
			errChan := make(chan error, 1)

			go func() {
				defer pw.Close()
				errChan <- registry.ContainerEngine().ContainerExport(ctx, createResponse.Id, entities.ContainerExportOptions{Output: pw})
			}()

			err = archive.Untar(makeTarDirsWritable(pr), buildOutputOption.Path, &archive.TarOptions{
				NoLchown:          true,
				IgnoreChownErrors: true,
			})
			if err != nil {
				pr.Close()
				return err
			}

			if err := <-errChan; err != nil {
				return fmt.Errorf("failed to export temporary container: %w", err)
			}
		}
	}
	return nil
}
