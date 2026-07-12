package kube

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	applyOptions     = entities.ApplyOptions{}
	applyDescription = `Command applies a podman container, pod, volume, or kube yaml to a Kubernetes cluster when a kubeconfig file is given.`

	applyCmd = &cobra.Command{
		Use:               "apply [options] [CONTAINER...|POD...|VOLUME...]",
		Short:             "Deploy a podman container, pod, volume, or Kubernetes yaml to a Kubernetes cluster",
		Long:              applyDescription,
		RunE:              apply,
		ValidArgsFunction: common.AutocompleteForKube,
		Example: `podman kube apply ctrName volName
podman kube apply --namespace project -f fileName`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: applyCmd,
		Parent:  kubeCmd,
	})
	applyFlags(applyCmd)
}

func applyFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.SetNormalizeFunc(utils.AliasFlags)

	kubeconfigFlagName := "kubeconfig"
	flags.StringVarP(&applyOptions.Kubeconfig, kubeconfigFlagName, "k", os.Getenv("KUBECONFIG"), "Path to the kubeconfig file for the Kubernetes cluster")
	_ = cmd.RegisterFlagCompletionFunc(kubeconfigFlagName, completion.AutocompleteDefault)

	namespaceFlagName := "ns"
	flags.StringVarP(&applyOptions.Namespace, namespaceFlagName, "", "", "The namespace to deploy the workload to on the Kubernetes cluster")
	_ = cmd.RegisterFlagCompletionFunc(namespaceFlagName, completion.AutocompleteNone)

	caCertFileFlagName := "ca-cert-file"
	flags.StringVarP(&applyOptions.CACertFile, caCertFileFlagName, "", "", "Path to the CA cert file for the Kubernetes cluster.")
	_ = cmd.RegisterFlagCompletionFunc(caCertFileFlagName, completion.AutocompleteDefault)

	fileFlagName := "file"
	flags.StringVarP(&applyOptions.File, fileFlagName, "f", "", "Path to the Kubernetes yaml file to deploy.")
	_ = cmd.RegisterFlagCompletionFunc(fileFlagName, completion.AutocompleteDefault)

	serviceFlagName := "service"
	flags.BoolVarP(&applyOptions.Service, serviceFlagName, "s", false, "Create a service object for the container being deployed.")
}

func apply(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("file") && cmd.Flags().Changed("service") {
		return errors.New("cannot set --service and --file at the same time")
	}

	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}
	if kubeconfig == "" {
		return errors.New("kubeconfig not given, unable to connect to cluster")
	}

	var reader io.Reader
	if cmd.Flags().Changed("file") {
		yamlFile := applyOptions.File
		if yamlFile == "-" {
			yamlFile = os.Stdin.Name()
		}

		f, err := os.Open(yamlFile)
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
	} else {
		generateOptions.Service = applyOptions.Service
		report, err := registry.ContainerEngine().GenerateKube(registry.Context(), args, generateOptions)
		if err != nil {
			return err
		}
		if r, ok := report.Reader.(io.ReadCloser); ok {
			defer r.Close()
		}
		reader = report.Reader
	}

	fmt.Println("Deploying to cluster...")

	if err = registry.ContainerEngine().KubeApply(registry.Context(), reader, applyOptions); err != nil {
		return err
	}

	fmt.Println("Successfully deployed workloads to cluster!")

	return nil
}
