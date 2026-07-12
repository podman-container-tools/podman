package images

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

type buildNode struct {
	Name            string
	Endpoint        string
	Status          string
	BuildkitVersion string
	Platforms       []string
}

type buildxInspectOutput struct {
	builderName string
	driverName  string
	Nodes       []buildNode
}

var buildxInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspects build capabilities",
	Long:  "Displays information about the current builder instance (compatibility with Docker buildx inspect)",
	RunE:  runBuildxInspect,
	Example: `podman buildx inspect
podman buildx inspect --bootstrap`,
}

func init() {
	buildxInspectCmd.Flags().Bool("bootstrap", false, "Currently a No Op for podman")
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildxInspectCmd,
		Parent:  buildxCmd,
	})
}

func runBuildxInspect(_ *cobra.Command, _ []string) error {
	info, err := registry.ContainerEngine().Info(registry.Context())
	if err != nil {
		return fmt.Errorf("retrieving podman information: %w", err)
	}

	nativePlatform := fmt.Sprintf("%s/%s", info.Host.OS, info.Host.Arch)

	// Constants are based on default values for Docker buildx inspect.
	defaultNode := buildNode{
		Name:            "default",
		Endpoint:        "default",
		Status:          "running",
		BuildkitVersion: "N/A",
		Platforms:       []string{nativePlatform},
	}

	defaultNode.Platforms = append(defaultNode.Platforms, info.Host.EmulatedArchitectures...)

	out := buildxInspectOutput{
		builderName: "default",
		driverName:  "podman",
		Nodes:       []buildNode{defaultNode},
	}

	fmt.Printf("Name:   %s\n", out.builderName)
	fmt.Printf("Driver: %s\n", out.driverName)
	fmt.Println()

	fmt.Println("Nodes:")
	fmt.Printf("Name:             %s\n", out.Nodes[0].Name)
	fmt.Printf("Endpoint:         %s\n", out.Nodes[0].Endpoint)
	fmt.Printf("Status:           %s\n", out.Nodes[0].Status)
	fmt.Printf("Buildkit version: %s\n", out.Nodes[0].BuildkitVersion)

	fmt.Printf("Platforms:        %s\n", strings.Join(out.Nodes[0].Platforms, ", "))
	fmt.Println("Labels: ")
	return nil
}
