package containers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	buildahCopiah "go.podman.io/buildah/copier"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/copy"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/storage/pkg/archive"
	"go.podman.io/storage/pkg/idtools"
)

var (
	cpDescription = `Copy the contents of SRC_PATH to the DEST_PATH.

  You can copy from the container's file system to the local machine or the reverse, from the local filesystem to the container. If "-" is specified for either the SRC_PATH or DEST_PATH, you can also stream a tar archive from STDIN or to STDOUT. The CONTAINER can be a running or stopped container. The SRC_PATH or DEST_PATH can be a file or a directory.
`
	cpCommand = &cobra.Command{
		Use:               "cp [options] [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
		Short:             "Copy files/folders between a container and the local filesystem",
		Long:              cpDescription,
		Args:              cobra.ExactArgs(2),
		RunE:              cp,
		ValidArgsFunction: common.AutocompleteCpCommand,
		Example:           "podman cp [options] [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
	}

	containerCpCommand = &cobra.Command{
		Use:               cpCommand.Use,
		Short:             cpCommand.Short,
		Long:              cpCommand.Long,
		Args:              cpCommand.Args,
		RunE:              cpCommand.RunE,
		ValidArgsFunction: cpCommand.ValidArgsFunction,
		Example:           "podman container cp [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
	}
)

var (
	cpOpts entities.ContainerCpOptions
	chown  bool
)

func cpFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVar(&cpOpts.OverwriteDirNonDir, "overwrite", false, "Allow to overwrite directories with non-directories and vice versa")
	flags.BoolVarP(&chown, "archive", "a", true, `Chown copied files to the primary uid/gid of the destination container.`)

	// Deprecated flags (both are NOPs): exist for backwards compat
	flags.BoolVar(&cpOpts.Extract, "extract", false, "Deprecated...")
	_ = flags.MarkHidden("extract")
	flags.BoolVar(&cpOpts.Pause, "pause", true, "Deprecated")
	_ = flags.MarkHidden("pause")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: cpCommand,
	})
	cpFlags(cpCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerCpCommand,
		Parent:  containerCmd,
	})
	cpFlags(containerCpCommand)
}

func cp(_ *cobra.Command, args []string) error {
	// Parse user input.
	sourceContainerStr, sourcePath, destContainerStr, destPath, err := copy.ParseSourceAndDestination(args[0], args[1])
	if err != nil {
		return err
	}

	if len(sourceContainerStr) > 0 && len(destContainerStr) > 0 {
		return copyContainerToContainer(sourceContainerStr, sourcePath, destContainerStr, destPath)
	} else if len(sourceContainerStr) > 0 {
		return copyFromContainer(sourceContainerStr, sourcePath, destPath)
	}

	return copyToContainer(destContainerStr, destPath, sourcePath)
}

// containerMustExist returns an error if the specified container does not
// exist.
func containerMustExist(container string) error {
	exists, err := registry.ContainerEngine().ContainerExists(registry.Context(), container, entities.ContainerExistsOptions{})
	if err != nil {
		return err
	}
	if !exists.Value {
		return fmt.Errorf("container %q does not exist", container)
	}
	return nil
}

// doCopy executes the two functions in parallel to copy data from A to B and
// joins the errors if any.
func doCopy(funcA func() error, funcB func() error) error {
	errChan := make(chan error)
	go func() {
		errChan <- funcA()
	}()
	copyErrors := []error{
		funcB(),
		<-errChan,
	}
	return errorhandling.JoinErrors(copyErrors)
}

func copyContainerToContainer(sourceContainer string, sourcePath string, destContainer string, destPath string) error {
	if err := containerMustExist(sourceContainer); err != nil {
		return err
	}

	if err := containerMustExist(destContainer); err != nil {
		return err
	}

	sourceContainerInfo, err := registry.ContainerEngine().ContainerStat(registry.Context(), sourceContainer, sourcePath)
	if err != nil {
		return fmt.Errorf("%q could not be found on container %s: %w", sourcePath, sourceContainer, err)
	}

	destContainerBaseName, destContainerInfo, destResolvedToParentDir, err := resolvePathOnDestinationContainer(destContainer, destPath, false)
	if err != nil {
		return err
	}

	if sourceContainerInfo.IsDir && !destContainerInfo.IsDir {
		return errors.New("destination must be a directory when copying a directory")
	}

	sourceContainerTarget := sourceContainerInfo.LinkTarget
	destContainerTarget := destContainerInfo.LinkTarget
	if !destContainerInfo.IsDir {
		destContainerTarget = path.Dir(destPath)
	}

	// If we copy a directory via the "." notation and the container path
	// does not exist, we need to make sure that the destination on the
	// container gets created; otherwise the contents of the source
	// directory will be written to the destination's parent directory.
	//
	// Hence, whenever "." is the source and the destination does not
	// exist, we copy the source's parent and let the copier package create
	// the destination via the Rename option.
	if destResolvedToParentDir && sourceContainerInfo.IsDir && path.Base(sourcePath) == "." {
		sourceContainerTarget = path.Dir(sourceContainerTarget)
	}

	reader, writer := io.Pipe()

	sourceContainerCopy := func() error {
		defer writer.Close()
		copyFunc, err := registry.ContainerEngine().ContainerCopyToArchive(registry.Context(), sourceContainer, sourceContainerTarget, writer)
		if err != nil {
			return err
		}
		if err := copyFunc(); err != nil {
			return fmt.Errorf("copying from container: %w", err)
		}
		return nil
	}

	destContainerCopy := func() error {
		defer reader.Close()

		copyOptions := entities.CopyOptions{Chown: chown, NoOverwriteDirNonDir: !cpOpts.OverwriteDirNonDir}
		if (!sourceContainerInfo.IsDir && !destContainerInfo.IsDir) || destResolvedToParentDir {
			// If we're having a file-to-file copy, make sure to
			// rename accordingly.
			copyOptions.Rename = map[string]string{path.Base(sourceContainerTarget): destContainerBaseName}
		}

		copyFunc, err := registry.ContainerEngine().ContainerCopyFromArchive(registry.Context(), destContainer, destContainerTarget, reader, copyOptions)
		if err != nil {
			return err
		}
		if err := copyFunc(); err != nil {
			return fmt.Errorf("copying to container: %w", err)
		}
		return nil
	}

	return doCopy(sourceContainerCopy, destContainerCopy)
}

// copyFromContainer copies from the containerPath on the container to hostPath.
func copyFromContainer(container string, containerPath string, hostPath string) error {
	if err := containerMustExist(container); err != nil {
		return err
	}

	isStdout := false
	if hostPath == "-" {
		isStdout = true
		hostPath = os.Stdout.Name()
	}

	containerInfo, err := registry.ContainerEngine().ContainerStat(registry.Context(), container, containerPath)
	if err != nil {
		return fmt.Errorf("%q could not be found on container %s: %w", containerPath, container, err)
	}

	var hostBaseName string
	var resolvedToHostParentDir bool
	hostInfo, hostInfoErr := copy.ResolveHostPath(hostPath)
	if hostInfoErr != nil {
		if strings.HasSuffix(hostPath, "/") {
			return fmt.Errorf("%q could not be found on the host: %w", hostPath, hostInfoErr)
		}
		// If it doesn't exist, then let's have a look at the parent dir.
		parentDir := filepath.Dir(hostPath)
		hostInfo, err = copy.ResolveHostPath(parentDir)
		if err != nil {
			return fmt.Errorf("%q could not be found on the host: %w", hostPath, hostInfoErr)
		}
		// If the specified path does not exist, we need to assume that
		// it'll be created while copying.  Hence, we use it as the
		// base path.
		hostBaseName = filepath.Base(hostPath)
		resolvedToHostParentDir = true
	} else {
		// If the specified path exists on the host, we must use its
		// base path as it may have changed due to symlink evaluations.
		hostBaseName = filepath.Base(hostInfo.LinkTarget)
	}

	if !isStdout {
		if err := validateFileInfo(hostInfo); err != nil {
			return fmt.Errorf("invalid destination: %w", err)
		}
	}

	// If we copy a directory via the "." notation and the host path does
	// not exist, we need to make sure that the destination on the host
	// gets created; otherwise the contents of the source directory will be
	// written to the destination's parent directory.
	//
	// While we could cut it short on the host and do create the directory
	// ourselves, we would run into problems trying to that the other way
	// around when copying into a container.  Instead, to keep both
	// implementations symmetrical, we need to massage the code a bit to
	// let Buildah's copier package create the destination.
	//
	// Hence, whenever "." is the source and the destination does not exist,
	// we copy the source's parent and let the copier package create the
	// destination via the Rename option.
	containerTarget := containerInfo.LinkTarget
	if resolvedToHostParentDir && containerInfo.IsDir && path.Base(containerTarget) == "." {
		containerTarget = path.Dir(containerTarget)
	}

	if !isStdout && containerInfo.IsDir && !hostInfo.IsDir {
		return errors.New("destination must be a directory when copying a directory")
	}

	reader, writer := io.Pipe()
	hostCopy := func() error {
		defer reader.Close()
		if isStdout {
			_, err := io.Copy(os.Stdout, reader)
			return err
		}

		groot, err := user.Current()
		if err != nil {
			return err
		}

		// Set the {G,U}ID.  Let's be tolerant towards the different
		// operating systems and only log the errors, so we can debug
		// if necessary.
		idPair := idtools.IDPair{}
		if i, err := strconv.Atoi(groot.Uid); err == nil {
			idPair.UID = i
		} else {
			logrus.Debugf("Error converting UID %q to int: %v", groot.Uid, err)
		}
		if i, err := strconv.Atoi(groot.Gid); err == nil {
			idPair.GID = i
		} else {
			logrus.Debugf("Error converting GID %q to int: %v", groot.Gid, err)
		}

		putOptions := buildahCopiah.PutOptions{
			ChownDirs:            &idPair,
			ChownFiles:           &idPair,
			IgnoreDevices:        true,
			NoOverwriteDirNonDir: !cpOpts.OverwriteDirNonDir,
			NoOverwriteNonDirDir: !cpOpts.OverwriteDirNonDir,
		}
		if (!containerInfo.IsDir && !hostInfo.IsDir) || resolvedToHostParentDir {
			// If we're having a file-to-file copy, make sure to
			// rename accordingly.
			putOptions.Rename = map[string]string{path.Base(containerTarget): hostBaseName}
		}
		dir := hostInfo.LinkTarget
		if !hostInfo.IsDir {
			dir = filepath.Dir(dir)
		}
		if err := buildahCopiah.Put(dir, "", putOptions, reader); err != nil {
			return fmt.Errorf("copying to host: %w", err)
		}
		return nil
	}

	containerCopy := func() error {
		defer writer.Close()
		copyFunc, err := registry.ContainerEngine().ContainerCopyToArchive(registry.Context(), container, containerTarget, writer)
		if err != nil {
			return err
		}
		if err := copyFunc(); err != nil {
			return fmt.Errorf("copying from container: %w", err)
		}
		return nil
	}
	return doCopy(containerCopy, hostCopy)
}

// copyToContainer copies the hostPath to containerPath on the container.
func copyToContainer(container string, containerPath string, hostPath string) error {
	if err := containerMustExist(container); err != nil {
		return err
	}

	hostInfo := &copy.FileInfo{}
	var err error

	isStdin := false
	if hostPath == "-" {
		isStdin = true
	} else {
		// Make sure that host path exists if not copying from stdin.
		hostInfo, err = copy.ResolveHostPath(hostPath)
		if err != nil {
			return fmt.Errorf("%q could not be found on the host: %w", hostPath, err)
		}
	}

	containerBaseName, containerInfo, containerResolvedToParentDir, err := resolvePathOnDestinationContainer(container, containerPath, isStdin)
	if err != nil {
		return err
	}

	// If we copy a directory via the "." notation and the container path
	// does not exist, we need to make sure that the destination on the
	// container gets created; otherwise the contents of the source
	// directory will be written to the destination's parent directory.
	//
	// Hence, whenever "." is the source and the destination does not
	// exist, we copy the source's parent and let the copier package create
	// the destination via the Rename option.
	hostTarget := hostInfo.LinkTarget
	if containerResolvedToParentDir && hostInfo.IsDir && filepath.Base(hostTarget) == "." {
		hostTarget = filepath.Dir(hostTarget)
	}

	var stdinFile string
	if isStdin {
		if !containerInfo.IsDir {
			return errors.New("destination must be a directory when copying from stdin")
		}

		// Copy from stdin to a temporary file *before* throwing it
		// over the wire.  This allows for proper client-side error
		// reporting.
		tmpFile, err := os.CreateTemp("", "")
		if err != nil {
			return err
		}
		defer os.Remove(tmpFile.Name())

		_, err = io.Copy(tmpFile, os.Stdin)
		if err != nil {
			_ = tmpFile.Close()
			return err
		}
		if err = tmpFile.Close(); err != nil {
			return err
		}

		if !archive.IsArchivePath(tmpFile.Name()) {
			return errors.New("source must be a (compressed) tar archive when copying from stdin")
		}
		stdinFile = tmpFile.Name()
	}

	if hostInfo.IsDir && !containerInfo.IsDir {
		return errors.New("destination must be a directory when copying a directory")
	}

	reader, writer := io.Pipe()
	hostCopy := func() error {
		defer writer.Close()
		if isStdin {
			stream, err := os.Open(stdinFile)
			if err != nil {
				return err
			}
			defer stream.Close()
			_, err = io.Copy(writer, stream)
			return err
		}

		getOptions := buildahCopiah.GetOptions{
			// Unless the specified path points to ".", we want to
			// copy the base directory.
			KeepDirectoryNames: hostInfo.IsDir && filepath.Base(hostTarget) != ".",
		}
		if (!hostInfo.IsDir && !containerInfo.IsDir) || containerResolvedToParentDir {
			// If we're having a file-to-file copy, make sure to
			// rename accordingly.
			getOptions.Rename = map[string]string{filepath.Base(hostTarget): containerBaseName}
		}

		// On Windows, the root path needs to be <drive>:\, while otherwise
		// it needs to be /. Combining filepath.VolumeName() + string(os.PathSeparator)
		// gives us the correct path for the current OS.
		if err := buildahCopiah.Get(filepath.VolumeName(hostTarget)+string(os.PathSeparator), "", getOptions, []string{hostTarget}, writer); err != nil {
			return fmt.Errorf("copying from host: %w", err)
		}
		return nil
	}

	containerCopy := func() error {
		defer reader.Close()
		target := containerInfo.FileInfo.LinkTarget
		if !containerInfo.IsDir {
			target = path.Dir(target)
		}

		copyFunc, err := registry.ContainerEngine().ContainerCopyFromArchive(registry.Context(), container, target, reader, entities.CopyOptions{Chown: chown, NoOverwriteDirNonDir: !cpOpts.OverwriteDirNonDir})
		if err != nil {
			return err
		}
		if err := copyFunc(); err != nil {
			return fmt.Errorf("copying to container: %w", err)
		}
		return nil
	}

	return doCopy(hostCopy, containerCopy)
}

// resolvePathOnDestinationContainer resolves the specified path on the
// container.  If the path does not exist, it attempts to use the parent
// directory.
func resolvePathOnDestinationContainer(container string, containerPath string, isStdin bool) (baseName string, containerInfo *entities.ContainerStatReport, resolvedToParentDir bool, err error) {
	containerInfo, err = registry.ContainerEngine().ContainerStat(registry.Context(), container, containerPath)
	if err == nil {
		baseName = path.Base(containerInfo.LinkTarget)
		return baseName, containerInfo, resolvedToParentDir, err //nolint: nilerr
	}

	if strings.HasSuffix(containerPath, "/") {
		err = fmt.Errorf("%q could not be found on container %s: %w", containerPath, container, err)
		return baseName, containerInfo, resolvedToParentDir, err
	}
	if isStdin {
		err = errors.New("destination must be a directory when copying from stdin")
		return baseName, containerInfo, resolvedToParentDir, err
	}

	// NOTE: containerInfo may actually be set.  That happens when
	// the container path is a symlink into nirvana.  In that case,
	// we must use the symlinked path instead.
	parentPath := containerPath
	if containerInfo != nil {
		baseName = path.Base(containerInfo.LinkTarget)
		parentPath = containerInfo.LinkTarget
	} else {
		baseName = path.Base(containerPath)
	}

	parentDir, err := containerParentDir(container, parentPath)
	if err != nil {
		err = fmt.Errorf("could not determine parent dir of %q on container %s: %w", parentPath, container, err)
		return baseName, containerInfo, resolvedToParentDir, err
	}

	containerInfo, err = registry.ContainerEngine().ContainerStat(registry.Context(), container, parentDir)
	if err != nil {
		err = fmt.Errorf("%q could not be found on container %s: %w", containerPath, container, err)
		return baseName, containerInfo, resolvedToParentDir, err
	}

	resolvedToParentDir = true
	return baseName, containerInfo, resolvedToParentDir, nil
}

// containerParentDir returns the parent directory of the specified path on the
// container.  If the path is relative, it will be resolved relative to the
// container's working directory (or "/" if the work dir isn't set).
func containerParentDir(container string, containerPath string) (string, error) {
	// This is specifically a path in the (linux) container, so we need to intentionally use
	// path instead of filepath to ensure we don't try to parse container paths using the
	// host OS conventions.
	if path.IsAbs(containerPath) {
		return path.Dir(containerPath), nil
	}
	inspectData, _, err := registry.ContainerEngine().ContainerInspect(registry.Context(), []string{container}, entities.InspectOptions{})
	if err != nil {
		return "", err
	}
	if len(inspectData) != 1 {
		return "", fmt.Errorf("inspecting container %q: expected 1 data item but got %d", container, len(inspectData))
	}
	workDir := path.Join("/", inspectData[0].Config.WorkingDir)
	workDir = path.Join(workDir, containerPath)
	return path.Dir(workDir), nil
}

// validateFileInfo returns an error if the specified FileInfo doesn't point to
// a directory or a regular file.
func validateFileInfo(info *copy.FileInfo) error {
	if info.Mode.IsDir() || info.Mode.IsRegular() {
		return nil
	}
	return fmt.Errorf("%q must be a directory or a regular file", info.LinkTarget)
}
