package common

import (
	"errors"
	"fmt"
	"net"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgenutil"
)

func DefineNetFlags(cmd *cobra.Command) {
	netFlags := cmd.Flags()

	addHostFlagName := "add-host"
	netFlags.StringSlice(
		addHostFlagName, []string{},
		"Add a custom host-to-IP mapping (host:ip) (default [])",
	)
	_ = cmd.RegisterFlagCompletionFunc(addHostFlagName, completion.AutocompleteNone)

	hostsFileFlagName := "hosts-file"
	netFlags.String(
		hostsFileFlagName, "",
		`Base file to create the /etc/hosts file inside the container, or one of the special values. ("image"|"none")`,
	)
	_ = cmd.RegisterFlagCompletionFunc(hostsFileFlagName, AutocompleteHostsFile)

	dnsFlagName := "dns"
	netFlags.StringSlice(
		dnsFlagName, podmanConfig.ContainersConf.DNSServers(),
		"Set custom DNS servers",
	)
	_ = cmd.RegisterFlagCompletionFunc(dnsFlagName, completion.AutocompleteNone)

	dnsOptFlagName := "dns-option"
	netFlags.StringSlice(
		dnsOptFlagName, podmanConfig.ContainersConf.DNSOptions(),
		"Set custom DNS options",
	)
	_ = cmd.RegisterFlagCompletionFunc(dnsOptFlagName, completion.AutocompleteNone)
	dnsSearchFlagName := "dns-search"
	netFlags.StringSlice(
		dnsSearchFlagName, podmanConfig.ContainersConf.DNSSearches(),
		"Set custom DNS search domains",
	)
	_ = cmd.RegisterFlagCompletionFunc(dnsSearchFlagName, completion.AutocompleteNone)

	ipFlagName := "ip"
	netFlags.String(
		ipFlagName, "",
		"Specify a static IPv4 address for the container",
	)
	_ = cmd.RegisterFlagCompletionFunc(ipFlagName, completion.AutocompleteNone)

	ip6FlagName := "ip6"
	netFlags.String(
		ip6FlagName, "",
		"Specify a static IPv6 address for the container",
	)
	_ = cmd.RegisterFlagCompletionFunc(ip6FlagName, completion.AutocompleteNone)

	macAddressFlagName := "mac-address"
	netFlags.String(
		macAddressFlagName, "",
		"Container MAC address (e.g. 92:d0:c6:0a:29:33)",
	)
	_ = cmd.RegisterFlagCompletionFunc(macAddressFlagName, completion.AutocompleteNone)

	networkFlagName := "network"
	netFlags.StringArray(
		networkFlagName, nil,
		"Connect a container to a network",
	)
	_ = cmd.RegisterFlagCompletionFunc(networkFlagName, AutocompleteNetworkFlag)

	networkAliasFlagName := "network-alias"
	netFlags.StringSlice(
		networkAliasFlagName, []string{},
		"Add network-scoped alias for the container",
	)
	_ = cmd.RegisterFlagCompletionFunc(networkAliasFlagName, completion.AutocompleteNone)

	publishFlagName := "publish"
	netFlags.StringSliceP(
		publishFlagName, "p", []string{},
		"Publish a container's port, or a range of ports, to the host (default [])",
	)
	_ = cmd.RegisterFlagCompletionFunc(publishFlagName, completion.AutocompleteNone)

	netFlags.Bool(
		"no-hostname", false, "Do not create /etc/hostname within the container, instead use the version from the image",
	)

	netFlags.Bool(
		"no-hosts", podmanConfig.ContainersConfDefaultsRO.Containers.NoHosts,
		"Do not create /etc/hosts within the container, instead use the version from the image",
	)
}

// NetFlagsToNetOptions parses the network flags for the given cmd.
func NetFlagsToNetOptions(opts *entities.NetOptions, flags pflag.FlagSet) (*entities.NetOptions, error) {
	var err error
	if opts == nil {
		opts = &entities.NetOptions{}
	}

	if flags.Changed("add-host") {
		opts.AddHosts, err = flags.GetStringSlice("add-host")
		if err != nil {
			return nil, err
		}
		// Verify the additional hosts are in correct format
		for _, host := range opts.AddHosts {
			if _, err := parse.ValidateExtraHost(host); err != nil {
				return nil, err
			}
		}
	}

	if flags.Changed("hosts-file") {
		opts.HostsFile, err = flags.GetString("hosts-file")
		if err != nil {
			return nil, err
		}
	}

	if flags.Changed("dns") {
		servers, err := flags.GetStringSlice("dns")
		if err != nil {
			return nil, err
		}
		for _, d := range servers {
			if d == "none" {
				opts.UseImageResolvConf = true
				if len(servers) > 1 {
					return nil, fmt.Errorf("%s is not allowed to be specified with other DNS ip addresses", d)
				}
				break
			}
			dns := net.ParseIP(d)
			if dns == nil {
				return nil, fmt.Errorf("%s is not an ip address", d)
			}
			opts.DNSServers = append(opts.DNSServers, dns)
		}
	}

	if flags.Changed("dns-option") {
		options, err := flags.GetStringSlice("dns-option")
		if err != nil {
			return nil, err
		}
		opts.DNSOptions = options
	}

	if flags.Changed("dns-search") {
		dnsSearches, err := flags.GetStringSlice("dns-search")
		if err != nil {
			return nil, err
		}
		// Validate domains are good
		for _, dom := range dnsSearches {
			if dom == "." {
				if len(dnsSearches) > 1 {
					return nil, errors.New("cannot pass additional search domains when also specifying '.'")
				}
				continue
			}
			if _, err := parse.ValidateDomain(dom); err != nil {
				return nil, err
			}
		}
		opts.DNSSearch = dnsSearches
	}

	if flags.Changed("publish") {
		inputPorts, err := flags.GetStringSlice("publish")
		if err != nil {
			return nil, err
		}
		if len(inputPorts) > 0 {
			opts.PublishPorts, err = specgenutil.CreatePortBindings(inputPorts)
			if err != nil {
				return nil, err
			}
		}
	}

	opts.NoHostname, err = flags.GetBool("no-hostname")
	if err != nil {
		return nil, err
	}

	opts.NoHosts, err = flags.GetBool("no-hosts")
	if err != nil {
		return nil, err
	}

	// parse the network only when network was changed
	// otherwise we send default to server so that the server
	// can pick the correct default instead of the client
	if flags.Changed("network") {
		network, err := flags.GetStringArray("network")
		if err != nil {
			return nil, err
		}

		ns, networks, networkOrder, options, err := specgen.ParseNetworkFlag(network)
		if err != nil {
			return nil, err
		}

		opts.NetworkOptions = options
		opts.Network = ns
		opts.Networks = networks
		opts.NetworkOrder = networkOrder
	}

	if flags.Changed("ip") || flags.Changed("ip6") || flags.Changed("mac-address") || flags.Changed("network-alias") {
		// if there is no network we add the default
		if len(opts.Networks) == 0 {
			opts.Networks = map[string]types.PerNetworkOptions{
				"default": {},
			}
		}

		for _, ipFlagName := range []string{"ip", "ip6"} {
			ip, err := flags.GetString(ipFlagName)
			if err != nil {
				return nil, err
			}
			if ip != "" {
				// if pod create --infra=false
				if infra, err := flags.GetBool("infra"); err == nil && !infra {
					return nil, fmt.Errorf("cannot set --%s without infra container: %w", ipFlagName, define.ErrInvalidArg)
				}

				staticIP := net.ParseIP(ip)
				if staticIP == nil {
					return nil, fmt.Errorf("%q is not an ip address", ip)
				}
				if !opts.Network.IsBridge() && !opts.Network.IsDefault() {
					return nil, fmt.Errorf("--%s can only be set when the network mode is bridge: %w", ipFlagName, define.ErrInvalidArg)
				}
				if len(opts.Networks) != 1 {
					return nil, fmt.Errorf("--%s can only be set for a single network: %w", ipFlagName, define.ErrInvalidArg)
				}
				for name, netOpts := range opts.Networks {
					netOpts.StaticIPs = append(netOpts.StaticIPs, staticIP)
					opts.Networks[name] = netOpts
				}
			}
		}

		m, err := flags.GetString("mac-address")
		if err != nil {
			return nil, err
		}
		if len(m) > 0 {
			// if pod create --infra=false
			if infra, err := flags.GetBool("infra"); err == nil && !infra {
				return nil, fmt.Errorf("cannot set --mac without infra container: %w", define.ErrInvalidArg)
			}
			mac, err := net.ParseMAC(m)
			if err != nil {
				return nil, err
			}
			if !opts.Network.IsBridge() && !opts.Network.IsDefault() {
				return nil, fmt.Errorf("--mac-address can only be set when the network mode is bridge: %w", define.ErrInvalidArg)
			}
			if len(opts.Networks) != 1 {
				return nil, fmt.Errorf("--mac-address can only be set for a single network: %w", define.ErrInvalidArg)
			}
			for name, netOpts := range opts.Networks {
				netOpts.StaticMAC = types.HardwareAddr(mac)
				opts.Networks[name] = netOpts
			}
		}

		aliases, err := flags.GetStringSlice("network-alias")
		if err != nil {
			return nil, err
		}
		if len(aliases) > 0 {
			// if pod create --infra=false
			if infra, err := flags.GetBool("infra"); err == nil && !infra {
				return nil, fmt.Errorf("cannot set --network-alias without infra container: %w", define.ErrInvalidArg)
			}
			if !opts.Network.IsBridge() && !opts.Network.IsDefault() {
				return nil, fmt.Errorf("--network-alias can only be set when the network mode is bridge: %w", define.ErrInvalidArg)
			}
			for name, netOpts := range opts.Networks {
				netOpts.Aliases = aliases
				opts.Networks[name] = netOpts
			}
		}
	}

	return opts, err
}
