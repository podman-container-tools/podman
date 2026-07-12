package containers

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	portDescription = `List port mappings for the CONTAINER, or look up the public-facing port that is NAT-ed to the PRIVATE_PORT
`
	portCommand = &cobra.Command{
		Use:   "port [options] CONTAINER [PORT]",
		Short: "List port mappings or a specific mapping for the container",
		Long:  portDescription,
		RunE:  port,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, true, "")
		},
		ValidArgsFunction: common.AutocompleteContainerOneArg,
		Example: `podman port --all
podman port ctrID 80/tcp`,
	}

	containerPortCommand = &cobra.Command{
		Use:   "port [options] CONTAINER [PORT]",
		Short: portCommand.Short,
		Long:  portDescription,
		RunE:  portCommand.RunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, true, "")
		},
		ValidArgsFunction: portCommand.ValidArgsFunction,
		Example: `podman container port --all
podman container port CTRID 80`,
	}
)

var portOpts entities.ContainerPortOptions

func portFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&portOpts.All, "all", "a", false, "Display port information for all containers")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: portCommand,
	})
	portFlags(portCommand.Flags())
	validate.AddLatestFlag(portCommand, &portOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerPortCommand,
		Parent:  containerCmd,
	})
	portFlags(containerPortCommand.Flags())
	validate.AddLatestFlag(containerPortCommand, &portOpts.Latest)
}

func port(_ *cobra.Command, args []string) error {
	var (
		container string
		err       error
		userPort  uint16
		userProto string
	)

	if len(args) == 0 && !portOpts.Latest && !portOpts.All {
		return errors.New("you must supply a running container name or id")
	}
	if !portOpts.Latest && len(args) >= 1 {
		container = strings.TrimPrefix(args[0], "/")
	}
	port := ""
	if len(args) > 2 {
		return errors.New("`port` accepts at most 2 arguments")
	}
	if len(args) > 1 && !portOpts.Latest {
		port = args[1]
	}
	if len(args) == 1 && portOpts.Latest {
		port = args[0]
	}
	if len(port) > 0 {
		fields := strings.Split(port, "/")
		if len(fields) > 2 || len(fields) < 1 {
			return fmt.Errorf("port formats are port/protocol. '%s' is invalid", port)
		}
		if len(fields) == 1 {
			fields = append(fields, "tcp")
		}

		portNum, err := strconv.ParseUint(fields[0], 10, 16)
		if err != nil {
			return err
		}
		userPort = uint16(portNum)
		userProto = fields[1]
	}

	reports, err := registry.ContainerEngine().ContainerPort(registry.Context(), container, portOpts)
	if err != nil {
		return err
	}
	var found bool
	// Iterate mappings
	for _, report := range reports {
		allPrefix := ""
		if portOpts.All {
			allPrefix = report.Id[:12] + "\t"
		}
		for _, v := range report.Ports {
			hostIP := v.HostIP
			// Set host IP to 0.0.0.0 if blank
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}
			for protocol := range strings.SplitSeq(v.Protocol, ",") {
				// If not searching by port or port/proto, then dump what we see
				if port == "" {
					for i := uint16(0); i < v.Range; i++ {
						fmt.Printf("%s%d/%s -> %s:%d\n", allPrefix, v.ContainerPort+i, protocol, hostIP, v.HostPort+i)
					}
					continue
				}
				// check if the proto matches and if the port is in the range
				// this is faster than looping over the range for no reason
				if v.Protocol == userProto &&
					v.ContainerPort <= userPort &&
					v.ContainerPort+v.Range > userPort {
					// we have to add the current range to the host port
					hostPort := v.HostPort + userPort - v.ContainerPort
					fmt.Printf("%s%s:%d\n", allPrefix, hostIP, hostPort)
					found = true
					break
				}
			}
		}
		if !found && port != "" {
			return fmt.Errorf("failed to find published port %q", port)
		}
	}
	return nil
}
