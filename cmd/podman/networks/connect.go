package network

import (
	"net"

	"github.com/spf13/cobra"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	networkConnectDescription = `Add container to a network`
	networkConnectCommand     = &cobra.Command{
		Use:               "connect [options] NETWORK CONTAINER",
		Short:             "Add container to a network",
		Long:              networkConnectDescription,
		RunE:              networkConnect,
		Example:           `podman network connect web secondary`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteNetworkConnectCmd,
	}
)

var (
	networkConnectOptions entities.NetworkConnectOptions
	ipv4                  net.IP
	ipv6                  net.IP
	macAddress            string
)

func networkConnectFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	aliasFlagName := "alias"
	flags.StringSliceVar(&networkConnectOptions.Aliases, aliasFlagName, nil, "network scoped alias for container")
	_ = cmd.RegisterFlagCompletionFunc(aliasFlagName, completion.AutocompleteNone)

	ipAddressFlagName := "ip"
	flags.IPVar(&ipv4, ipAddressFlagName, nil, "set a static ipv4 address for this container network")
	_ = cmd.RegisterFlagCompletionFunc(ipAddressFlagName, completion.AutocompleteNone)

	ipv6AddressFlagName := "ip6"
	flags.IPVar(&ipv6, ipv6AddressFlagName, nil, "set a static ipv6 address for this container network")
	_ = cmd.RegisterFlagCompletionFunc(ipv6AddressFlagName, completion.AutocompleteNone)

	macAddressFlagName := "mac-address"
	flags.StringVar(&macAddress, macAddressFlagName, "", "set a static mac address for this container network")
	_ = cmd.RegisterFlagCompletionFunc(macAddressFlagName, completion.AutocompleteNone)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkConnectCommand,
		Parent:  networkCmd,
	})
	networkConnectFlags(networkConnectCommand)
}

func networkConnect(_ *cobra.Command, args []string) error {
	networkConnectOptions.Container = args[1]
	if macAddress != "" {
		mac, err := net.ParseMAC(macAddress)
		if err != nil {
			return err
		}
		networkConnectOptions.StaticMAC = types.HardwareAddr(mac)
	}
	for _, ip := range []net.IP{ipv4, ipv6} {
		if ip != nil {
			networkConnectOptions.StaticIPs = append(networkConnectOptions.StaticIPs, ip)
		}
	}

	return registry.ContainerEngine().NetworkConnect(registry.Context(), args[0], networkConnectOptions)
}
