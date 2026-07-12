package network

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/libnetwork/util"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	networkCreateDescription = `Create networks for containers and pods`
	networkCreateCommand     = &cobra.Command{
		Use:               "create [options] [NAME]",
		Short:             "Create networks for containers and pods",
		Long:              networkCreateDescription,
		RunE:              networkCreate,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman network create podman1`,
	}
)

var (
	networkCreateOptions entities.NetworkCreateOptions
	labels               []string
	opts                 []string
	ipamDriverFlagName   = "ipam-driver"
	ipamDriver           string
)

func networkCreateFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	driverFlagName := "driver"
	flags.StringVarP(&networkCreateOptions.Driver, driverFlagName, "d", types.DefaultNetworkDriver, "driver to manage the network")
	_ = cmd.RegisterFlagCompletionFunc(driverFlagName, common.AutocompleteNetworkDriver)

	optFlagName := "opt"
	flags.StringArrayVarP(&opts, optFlagName, "o", nil, "Set driver specific options (default [])")
	_ = cmd.RegisterFlagCompletionFunc(optFlagName, completion.AutocompleteNone)

	gatewayFlagName := "gateway"
	flags.IPSliceVar(&networkCreateOptions.Gateways, gatewayFlagName, nil, "IPv4 or IPv6 gateway for the subnet")
	_ = cmd.RegisterFlagCompletionFunc(gatewayFlagName, completion.AutocompleteNone)

	flags.BoolVar(&networkCreateOptions.Internal, "internal", false, "restrict external access from this network")

	ipRangeFlagName := "ip-range"
	flags.StringArrayVar(&networkCreateOptions.Ranges, ipRangeFlagName, nil, "allocate container IP from range")
	_ = cmd.RegisterFlagCompletionFunc(ipRangeFlagName, completion.AutocompleteNone)

	labelFlagName := "label"
	flags.StringArrayVar(&labels, labelFlagName, nil, "set metadata on a network")
	_ = cmd.RegisterFlagCompletionFunc(labelFlagName, completion.AutocompleteNone)

	flags.StringVar(&ipamDriver, ipamDriverFlagName, "", "IP Address Management Driver")
	_ = cmd.RegisterFlagCompletionFunc(ipamDriverFlagName, common.AutocompleteNetworkIPAMDriver)

	flags.BoolVar(&networkCreateOptions.IPv6, "ipv6", false, "enable IPv6 networking")

	subnetFlagName := "subnet"
	flags.StringArrayVar(&networkCreateOptions.Subnets, subnetFlagName, nil, "subnets in CIDR format")
	_ = cmd.RegisterFlagCompletionFunc(subnetFlagName, completion.AutocompleteNone)

	routeFlagName := "route"
	flags.StringArrayVar(&networkCreateOptions.Routes, routeFlagName, nil, "Static routes for this network. Format: <destination>,<gateway>[,<metric>] or <destination>,<type>[,<metric>] where type is blackhole, unreachable, or prohibit")
	_ = cmd.RegisterFlagCompletionFunc(routeFlagName, completion.AutocompleteNone)

	interfaceFlagName := "interface-name"
	flags.StringVar(&networkCreateOptions.InterfaceName, interfaceFlagName, "", "interface name which is used by the driver")
	_ = cmd.RegisterFlagCompletionFunc(interfaceFlagName, common.AutocompleteNetworkInterfaceNames)

	flags.BoolVar(&networkCreateOptions.DisableDNS, "disable-dns", false, "disable dns plugin")

	flags.BoolVar(&networkCreateOptions.IgnoreIfExists, "ignore", false, "Don't fail if network already exists")
	dnsserverFlagName := "dns"
	flags.StringSliceVar(&networkCreateOptions.NetworkDNSServers, dnsserverFlagName, nil, "DNS servers this network will use")
	_ = cmd.RegisterFlagCompletionFunc(dnsserverFlagName, completion.AutocompleteNone)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkCreateCommand,
		Parent:  networkCmd,
	})
	networkCreateFlags(networkCreateCommand)
}

func networkCreate(cmd *cobra.Command, args []string) error {
	var name string
	if len(args) > 0 {
		name = args[0]
	}
	var err error
	networkCreateOptions.Labels, err = parse.GetAllLabels([]string{}, labels)
	if err != nil {
		return fmt.Errorf("failed to parse labels: %w", err)
	}
	networkCreateOptions.Options, err = parse.GetAllLabels([]string{}, opts)
	if err != nil {
		return fmt.Errorf("unable to parse options: %w", err)
	}

	network := types.Network{
		Name:              name,
		Driver:            networkCreateOptions.Driver,
		Options:           networkCreateOptions.Options,
		Labels:            networkCreateOptions.Labels,
		IPv6Enabled:       networkCreateOptions.IPv6,
		DNSEnabled:        !networkCreateOptions.DisableDNS,
		NetworkDNSServers: networkCreateOptions.NetworkDNSServers,
		Internal:          networkCreateOptions.Internal,
		NetworkInterface:  networkCreateOptions.InterfaceName,
	}

	if cmd.Flags().Changed(ipamDriverFlagName) {
		network.IPAMOptions = map[string]string{
			types.Driver: ipamDriver,
		}
	}

	if networkCreateOptions.Driver == types.MacVLANNetworkDriver || networkCreateOptions.Driver == types.IPVLANNetworkDriver {
		// new -d macvlan --opt parent=... syntax
		if parent, ok := network.Options["parent"]; ok {
			network.NetworkInterface = parent
			delete(network.Options, "parent")
		}
	}

	if len(networkCreateOptions.Subnets) > 0 {
		if len(networkCreateOptions.Gateways) > len(networkCreateOptions.Subnets) {
			return errors.New("cannot set more gateways than subnets")
		}
		if len(networkCreateOptions.Ranges) > len(networkCreateOptions.Subnets) {
			return errors.New("cannot set more ranges than subnets")
		}

		for i := range networkCreateOptions.Subnets {
			subnet, err := types.ParseCIDR(networkCreateOptions.Subnets[i])
			if err != nil {
				return err
			}
			s := types.Subnet{
				Subnet: subnet,
			}
			if len(networkCreateOptions.Ranges) > i {
				leaseRange, err := parseRange(networkCreateOptions.Ranges[i])
				if err != nil {
					return err
				}
				s.LeaseRange = leaseRange
			}
			if len(networkCreateOptions.Gateways) > i {
				s.Gateway = networkCreateOptions.Gateways[i]
			}
			network.Subnets = append(network.Subnets, s)
		}
	} else if len(networkCreateOptions.Ranges) > 0 || len(networkCreateOptions.Gateways) > 0 {
		return errors.New("cannot set gateway or range without subnet")
	}

	for i := range networkCreateOptions.Routes {
		route, err := parseRoute(networkCreateOptions.Routes[i])
		if err != nil {
			return err
		}

		network.Routes = append(network.Routes, *route)
	}

	extraCreateOptions := types.NetworkCreateOptions{
		IgnoreIfExists: networkCreateOptions.IgnoreIfExists,
	}

	response, err := registry.ContainerEngine().NetworkCreate(registry.Context(), network, &extraCreateOptions)
	if err != nil {
		return err
	}
	fmt.Println(response.Name)
	return nil
}

func parseRoute(routeStr string) (*types.Route, error) {
	s := strings.Split(routeStr, ",")

	if len(s) < 2 || len(s) > 3 {
		return nil, fmt.Errorf("invalid route: %s\nFormat: --route <destination>,<gateway>[,<metric>] or --route <destination>,<type>[,<metric>] where type is blackhole, unreachable, or prohibit", routeStr)
	}

	destination, err := types.ParseCIDR(s[0])
	if err != nil {
		return nil, fmt.Errorf("invalid route destination %s: %w", s[0], err)
	}

	route := &types.Route{
		Destination: destination,
	}

	// Check if second field is a route type (blackhole, unreachable, prohibit)
	secondField := strings.ToLower(s[1])
	switch types.RouteType(secondField) {
	case types.RouteTypeBlackhole, types.RouteTypeUnreachable, types.RouteTypeProhibit:
		route.RouteType = types.RouteType(secondField)
		// Parse optional metric from position 2
		if len(s) >= 3 {
			mtr, err := strconv.ParseUint(s[2], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid route metric %s", s[2])
			}
			x := uint32(mtr)
			route.Metric = &x
		}
		return route, nil
	case types.RouteTypeUnicast:
		return nil, fmt.Errorf("unicast route requires a gateway, format: --route <destination>,<gateway>[,<metric>]")
	}

	// Regular unicast route with gateway
	gateway := net.ParseIP(s[1])
	if gateway == nil {
		return nil, fmt.Errorf("invalid route gateway %s", s[1])
	}
	route.Gateway = gateway
	route.RouteType = types.RouteTypeUnicast // Default

	// Parse optional metric
	if len(s) >= 3 {
		mtr, err := strconv.ParseUint(s[2], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid route metric %s", s[2])
		}
		x := uint32(mtr)
		route.Metric = &x
	}
	return route, nil
}

func parseRange(iprange string) (*types.LeaseRange, error) {
	startIPString, endIPString, hasDash := strings.Cut(iprange, "-")
	if hasDash {
		// range contains dash so assume form is start-end
		start := net.ParseIP(startIPString)
		if start == nil {
			return nil, fmt.Errorf("range start ip %q is not a ip address", startIPString)
		}
		end := net.ParseIP(endIPString)
		if end == nil {
			return nil, fmt.Errorf("range end ip %q is not a ip address", endIPString)
		}
		return &types.LeaseRange{
			StartIP: start,
			EndIP:   end,
		}, nil
	}
	// no dash, so assume CIDR is given
	_, subnet, err := net.ParseCIDR(iprange)
	if err != nil {
		return nil, err
	}

	startIP, err := util.FirstIPInSubnet(subnet)
	if err != nil {
		return nil, fmt.Errorf("failed to get first ip in range: %w", err)
	}
	lastIP, err := util.LastIPInSubnet(subnet)
	if err != nil {
		return nil, fmt.Errorf("failed to get last ip in range: %w", err)
	}
	return &types.LeaseRange{
		StartIP: startIP,
		EndIP:   lastIP,
	}, nil
}
