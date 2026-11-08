package configs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var createCmd = &cobra.Command{
	Use:   "create [options] NAME FILE|-",
	Short: "Create a new config",
	Long:  "Create a config. Input can be a path to a file or \"-\" (read from stdin). Config drivers \"file\" (default), \"pass\", and \"shell\" are available.",
	RunE:  create,
	Args:  cobra.ExactArgs(2),
	Example: `podman config create myconfig /path/to/config
printf "configdata" | podman config create myconfig -`,
	ValidArgsFunction: common.AutocompleteConfigCreate,
}

var (
	createOpts = entities.ConfigCreateOptions{}
	env        = false
	labels     []string
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCmd,
		Parent:  configCmd,
	})
	flags := createCmd.Flags()

	driverFlagName := "driver"
	flags.StringVarP(&createOpts.Driver, driverFlagName, "d", "file", "Specify config driver")
	_ = createCmd.RegisterFlagCompletionFunc(driverFlagName, completion.AutocompleteNone)
	flags.StringToStringVar(&createOpts.DriverOpts, "driver-opts", nil, "Specify driver-specific options")
	_ = createCmd.RegisterFlagCompletionFunc("driver-opts", completion.AutocompleteNone)

	envFlagName := "env"
	flags.BoolVar(&env, envFlagName, false, "Read config data from environment variable")

	flags.BoolVar(&createOpts.Replace, "replace", false, "If a config with the same name exists, replace it")

	flags.BoolVar(&createOpts.Ignore, "ignore", false, "If a config with the same name exists, ignore and do not create a new config")

	labelFlagName := "label"
	flags.StringArrayVarP(&labels, labelFlagName, "l", nil, "Specify labels on the config")
	_ = createCmd.RegisterFlagCompletionFunc(labelFlagName, completion.AutocompleteNone)
}

func create(cmd *cobra.Command, args []string) error {
	name := args[0]

	if createOpts.Ignore && createOpts.Replace {
		return errors.New("cannot use --ignore and --replace flags together")
	}

	var err error
	path := args[1]

	var reader io.Reader
	switch {
	case env:
		envValue := os.Getenv(path)
		if envValue == "" {
			return fmt.Errorf("cannot create store config data: environment variable %s is not set", path)
		}
		reader = strings.NewReader(envValue)
	case path == "-" || path == "/dev/stdin":
		reader = os.Stdin
	default:
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}

	createOpts.Labels, err = parse.GetAllLabels([]string{}, labels)
	if err != nil {
		return fmt.Errorf("unable to process labels: %w", err)
	}

	report, err := registry.ContainerEngine().ConfigCreate(cmd.Context(), name, reader, createOpts)
	if err != nil {
		return err
	}
	fmt.Println(report.ID)
	return nil
}
