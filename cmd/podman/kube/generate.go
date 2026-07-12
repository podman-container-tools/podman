package kube

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/generate"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/storage/pkg/fileutils"
)

var (
	generateOptions     = entities.GenerateKubeOptions{}
	generateFile        = ""
	generateDescription = `Command generates Kubernetes Pod, Service or PersistentVolumeClaim YAML (v1 specification) from Podman containers, pods or volumes.

  Whether the input is for a container or pod, Podman will always generate the specification as a pod.`

	kubeGenerateCmd = &cobra.Command{
		Use:               "generate [options] {CONTAINER...|POD...|VOLUME...}",
		Short:             "Generate Kubernetes YAML from containers, pods or volumes.",
		Long:              generateDescription,
		RunE:              generateKube,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteForGenerate,
		Example: `podman kube generate ctrID
podman kube generate podID
podman kube generate --service podID
podman kube generate volumeName
podman kube generate ctrID podID volumeName --service`,
	}

	generateKubeCmd = &cobra.Command{
		Use:               "kube [options] {CONTAINER...|POD...|VOLUME...}",
		Short:             kubeGenerateCmd.Short,
		Long:              kubeGenerateCmd.Long,
		RunE:              kubeGenerateCmd.RunE,
		Args:              kubeGenerateCmd.Args,
		ValidArgsFunction: kubeGenerateCmd.ValidArgsFunction,
		Example:           kubeGenerateCmd.Example,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: generateKubeCmd,
		Parent:  generate.GenerateCmd,
	})
	generateFlags(generateKubeCmd, registry.PodmanConfig())

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: kubeGenerateCmd,
		Parent:  kubeCmd,
	})
	generateFlags(kubeGenerateCmd, registry.PodmanConfig())
}

func generateFlags(cmd *cobra.Command, podmanConfig *entities.PodmanConfig) {
	flags := cmd.Flags()
	flags.BoolVarP(&generateOptions.Service, "service", "s", false, "Generate YAML for a Kubernetes service object")

	filenameFlagName := "filename"
	flags.StringVarP(&generateFile, filenameFlagName, "f", "", "Write output to the specified path")
	_ = cmd.RegisterFlagCompletionFunc(filenameFlagName, completion.AutocompleteDefault)

	typeFlagName := "type"
	// If remote, don't read the client's containers.conf file
	defaultGenerateType := ""
	if !registry.IsRemote() {
		defaultGenerateType = podmanConfig.ContainersConfDefaultsRO.Engine.KubeGenerateType
	}
	flags.StringVarP(&generateOptions.Type, typeFlagName, "t", defaultGenerateType, "Generate YAML for the given Kubernetes kind")
	_ = cmd.RegisterFlagCompletionFunc(typeFlagName, completion.AutocompleteNone)

	replicasFlagName := "replicas"
	flags.Int32VarP(&generateOptions.Replicas, replicasFlagName, "r", 1, "Set the replicas number for Deployment kind")
	_ = cmd.RegisterFlagCompletionFunc(replicasFlagName, completion.AutocompleteNone)

	noTruncAnnotationsFlagName := "no-trunc"
	flags.BoolVar(&generateOptions.UseLongAnnotations, noTruncAnnotationsFlagName, false, "Don't truncate annotations to Kubernetes length (63 chars)")
	_ = flags.MarkHidden(noTruncAnnotationsFlagName)

	podmanOnlyFlagName := "podman-only"
	flags.BoolVar(&generateOptions.PodmanOnly, podmanOnlyFlagName, false, "Add podman-only reserved annotations to the generated YAML file (Cannot be used by Kubernetes)")

	flags.SetNormalizeFunc(utils.AliasFlags)
}

func generateKube(cmd *cobra.Command, args []string) error {
	report, err := registry.ContainerEngine().GenerateKube(registry.Context(), args, generateOptions)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(report.Reader)
	if err != nil {
		return err
	}
	if r, ok := report.Reader.(io.ReadCloser); ok {
		defer r.Close()
	}

	if cmd.Flags().Changed("filename") {
		if err := fileutils.Exists(generateFile); err == nil {
			return fmt.Errorf("cannot write to %q; file exists", generateFile)
		}
		if err := os.WriteFile(generateFile, content, 0o644); err != nil {
			return fmt.Errorf("cannot write to %q: %w", generateFile, err)
		}
		return nil
	}

	fmt.Println(string(content))
	return nil
}
