//go:build linux || freebsd

package netavark

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/pasta"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
)

type netavarkOptions struct {
	types.NetworkOptions
	Networks map[string]*types.Network `json:"network_info"`
}

func (n *netavarkNetwork) execUpdate(networkName string, networkDNSServers []string) error {
	retErr := n.execNetavark([]string{"update", networkName, "--network-dns-servers", strings.Join(networkDNSServers, ",")}, false, nil, nil)
	return retErr
}

func (n *netavarkNetwork) validateSetupOptions(namespacePath string, options types.SetupOptions) error {
	if namespacePath == "" {
		return errors.New("namespacePath is empty")
	}
	if options.ContainerID == "" {
		return errors.New("ContainerID is empty")
	}
	if len(options.Networks) == 0 {
		return errors.New("must specify at least one network")
	}
	for _, net := range options.Networks {
		network, ok := n.networks[net.Name]
		if !ok {
			return fmt.Errorf("unable to find network with name %s: %w", net.Name, types.ErrNoSuchNetwork)
		}

		err := validatePerNetworkOpts(network, &net.PerNetworkOptions)
		if err != nil {
			return err
		}
	}
	return nil
}

// validatePerNetworkOpts checks that all given static ips are in a subnet on this network.
func validatePerNetworkOpts(network *types.Network, netOpts *types.PerNetworkOptions) error {
	if netOpts.InterfaceName == "" {
		return fmt.Errorf("interface name on network %s is empty", network.Name)
	}
	if network.IPAMOptions[types.Driver] == types.HostLocalIPAMDriver {
	outer:
		for _, ip := range netOpts.StaticIPs {
			for _, s := range network.Subnets {
				if s.Subnet.Contains(ip) {
					continue outer
				}
			}
			return fmt.Errorf("requested static ip %s not in any subnet on network %s", ip.String(), network.Name)
		}
	}
	return nil
}

// Setup will setup the container network namespace. It returns
// a map of StatusBlocks, the key is the network name.
func (n *netavarkNetwork) Setup(namespacePath string, options types.SetupOptions) (_ map[string]types.StatusBlock, retErr error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return nil, err
	}

	err = n.validateSetupOptions(namespacePath, options)
	if err != nil {
		return nil, err
	}

	// allocate IPs in the IPAM db
	err = n.allocIPs(&options.NetworkOptions)
	if err != nil {
		return nil, err
	}
	defer func() {
		// In case the setup failed for whatever reason podman will not start the
		// container so we must free the allocated ips again to not leak them.
		if retErr != nil {
			if err := n.deallocIPs(&options.NetworkOptions); err != nil {
				logrus.Error(err)
			}
		}
	}()

	netavarkOpts, needPlugin, err := n.convertNetOpts(options.NetworkOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to convert net opts: %w", err)
	}

	// Warn users if one or more networks have dns enabled
	// but aardvark-dns binary is not configured
	for _, network := range netavarkOpts.Networks {
		if network != nil && network.DNSEnabled && n.aardvarkBinary == "" {
			// this is not a fatal error we can still use container without dns
			logrus.Warnf("aardvark-dns binary not found, container dns will not be enabled")
			break
		}
	}

	// trace output to get the json
	if logrus.IsLevelEnabled(logrus.TraceLevel) {
		b, err := json.Marshal(&netavarkOpts)
		if err != nil {
			return nil, err
		}
		// show the full netavark command so we can easily reproduce errors from the cli
		logrus.Tracef("netavark command: printf '%s' | %s setup %s", string(b), n.netavarkBinary, namespacePath)
	}

	result := map[string]types.StatusBlock{}
	setup := func() error {
		return n.execNetavark([]string{"setup", namespacePath}, needPlugin, netavarkOpts, &result)
	}

	if n.rootlessNetns != nil {
		err = n.rootlessNetns.Setup(len(options.Networks), setup)
	} else {
		err = setup()
	}
	if err != nil {
		return nil, err
	}

	// make sure that the result makes sense
	if len(result) != len(options.Networks) {
		logrus.Errorf("unexpected netavark result: %v", result)
		return nil, fmt.Errorf("unexpected netavark result length, want (%d), got (%d) networks", len(options.Networks), len(result))
	}

	if n.rootlessPortForwarder == config.RootlessPortForwarderPasta && n.networkRootless && len(options.NetworkOptions.PortMappings) > 0 {
		opts := options.NetworkOptions

		if len(opts.NetworkOrder) == 0 {
			opts.NetworkOrder = make([]string, 0, len(opts.Networks))
			for _, net := range opts.Networks {
				opts.NetworkOrder = append(opts.NetworkOrder, net.Name)
			}
		}
		if err := n.pestoSetup(opts, result); err != nil {
			return nil, err
		}
	}

	return result, err
}

// pestoSetup publishes port mappings via pesto with target address mapping.
// PestoAddPorts is idempotent so this is safe to call on every Setup.
func (n *netavarkNetwork) pestoSetup(opts types.NetworkOptions, newResult map[string]types.StatusBlock) error {
	if n.rootlessNetns == nil {
		return nil
	}

	// First check if the ports are already published on an existing network.
	// Because a port can only bound once on the host we need to skip the adding the ports several times
	// on things like podman network connect.
	oldIPv4, _, oldIPv6, _ := firstIPsFromStatus(opts.NetworkStatus, opts.NetworkOrder)
	if oldIPv4 != "" && oldIPv6 != "" {
		// We are already connected to an v4 and v6 address, do nothing here.
		return nil
	}
	ipv4, _, ipv6, _ := firstIPsFromStatus(newResult, opts.NetworkOrder)
	if oldIPv4 != "" {
		// Already connected to a v4 address, we cannot connect more than once so skip it.
		ipv4 = ""
	}
	if oldIPv6 != "" {
		// Same but for ipv6.
		ipv6 = ""
	}

	if ipv4 == "" && ipv6 == "" {
		// Still no new ips to forward so do nothing.
		return nil
	}

	return pasta.PestoAddPorts(n.config, n.PestoSocketPath(), opts.PortMappings, ipv4, ipv6)
}

// pestoTeardown handles pesto port mapping removal on network disconnect.
// It derives the active forwarding IPs from the caller-provided NetworkStatus
// (the podman db state) rather than maintaining a separate state file.
//   - If NetworkStatus is empty: nothing to do.
//   - If this is the last network: delete all mappings.
//   - If the disconnected network supplied the active IP: delete old mappings,
//     pick a replacement IP from the remaining networks, and re-add.
//   - Otherwise: no port changes needed.
func (n *netavarkNetwork) pestoTeardown(opts types.NetworkOptions) error {
	if n.rootlessNetns == nil {
		return nil
	}

	if len(opts.NetworkStatus) == 0 {
		return nil
	}

	// Determine current active IPs from the full set of connected networks,
	// respecting connection-time ordering from NetworkOrder.
	activeIPv4, activeIPv4Net, activeIPv6, activeIPv6Net := firstIPsFromStatus(opts.NetworkStatus, opts.NetworkOrder)

	// Build the set of networks being disconnected and the remaining status.
	remaining := make(map[string]types.StatusBlock, len(opts.NetworkStatus))
	disconnectedNets := make(map[string]struct{}, len(opts.Networks))
	for _, namedNet := range opts.Networks {
		disconnectedNets[namedNet.Name] = struct{}{}
	}
	for name, sb := range opts.NetworkStatus {
		if _, removed := disconnectedNets[name]; !removed {
			remaining[name] = sb
		}
	}

	// Last network: remove all port mappings.
	if len(remaining) == 0 {
		return pasta.PestoDeletePorts(n.config, n.PestoSocketPath(), opts.PortMappings,
			activeIPv4, activeIPv6)
	}

	// Check whether any active IP came from a network being disconnected.
	_, v4Lost := disconnectedNets[activeIPv4Net]
	v4Lost = v4Lost && activeIPv4Net != ""
	_, v6Lost := disconnectedNets[activeIPv6Net]
	v6Lost = v6Lost && activeIPv6Net != ""

	if !v4Lost && !v6Lost {
		return nil
	}

	// Remap: delete old mappings, pick new IPs from remaining networks, re-add.
	if err := pasta.PestoDeletePorts(n.config, n.PestoSocketPath(), opts.PortMappings,
		activeIPv4, activeIPv6); err != nil {
		return fmt.Errorf("deleting port mappings for remap: %w", err)
	}

	newIPv4, _, newIPv6, _ := firstIPsFromStatus(remaining, opts.NetworkOrder)
	if newIPv4 != "" || newIPv6 != "" {
		if err := pasta.PestoAddPorts(n.config, n.PestoSocketPath(), opts.PortMappings,
			newIPv4, newIPv6); err != nil {
			return fmt.Errorf("re-adding port mappings after remap: %w", err)
		}
	}
	return nil
}

// Teardown will teardown the container network namespace.
func (n *netavarkNetwork) Teardown(namespacePath string, options types.TeardownOptions) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return err
	}

	// get IPs from the IPAM db
	err = n.getAssignedIPs(&options.NetworkOptions)
	if err != nil {
		// when there is an error getting the ips we should still continue
		// to call teardown for netavark to prevent leaking network interfaces
		logrus.Error(err)
	}

	if n.rootlessPortForwarder == config.RootlessPortForwarderPasta && n.networkRootless && len(options.NetworkOptions.PortMappings) > 0 {
		opts := options.NetworkOptions
		if len(opts.NetworkOrder) == 0 {
			opts.NetworkOrder = make([]string, 0, len(opts.Networks))
			for _, net := range opts.Networks {
				opts.NetworkOrder = append(opts.NetworkOrder, net.Name)
			}
		}
		if err := n.pestoTeardown(opts); err != nil {
			logrus.Errorf("pesto: %v", err)
		}
	}

	netavarkOpts, needPlugin, err := n.convertNetOpts(options.NetworkOptions)
	if err != nil {
		return fmt.Errorf("failed to convert net opts: %w", err)
	}

	var retErr error
	teardown := func() error {
		return n.execNetavark([]string{"teardown", namespacePath}, needPlugin, netavarkOpts, nil)
	}

	if n.rootlessNetns != nil {
		retErr = n.rootlessNetns.Teardown(len(options.Networks), teardown)
	} else {
		retErr = teardown()
	}

	// when netavark returned an error we still free the used ips
	// otherwise we could end up in a state where block the ips forever
	err = n.deallocIPs(&netavarkOpts.NetworkOptions)
	if err != nil {
		if retErr != nil {
			logrus.Error(err)
		} else {
			retErr = err
		}
	}

	return retErr
}

func (n *netavarkNetwork) getCommonNetavarkOptions(needPlugin bool) []string {
	opts := []string{"--config", n.networkRunDir, "--rootless=" + strconv.FormatBool(n.networkRootless), "--aardvark-binary=" + n.aardvarkBinary}
	// to allow better backwards compat we only add the new netavark option when really needed
	if needPlugin {
		// Note this will require a netavark with https://github.com/containers/netavark/pull/509
		for _, dir := range n.pluginDirs {
			opts = append(opts, "--plugin-directory", dir)
		}
	}
	return opts
}

func (n *netavarkNetwork) convertNetOpts(opts types.NetworkOptions) (*netavarkOptions, bool, error) {
	netavarkOptions := netavarkOptions{
		NetworkOptions: opts,
		Networks:       make(map[string]*types.Network, len(opts.Networks)),
	}

	needsPlugin := false

	foundNetwork := make(map[string]struct{}, len(opts.Networks))

	for _, network := range opts.Networks {
		if _, ok := foundNetwork[network.Name]; ok {
			return nil, false, fmt.Errorf("network %s passed twice in NetworkOptions", network.Name)
		}
		foundNetwork[network.Name] = struct{}{}

		net, err := n.getNetwork(network.Name)
		if err != nil {
			return nil, false, err
		}
		netavarkOptions.Networks[network.Name] = net
		if !slices.Contains(builtinDrivers, net.Driver) {
			needsPlugin = true
		}
	}
	return &netavarkOptions, needsPlugin, nil
}

func (n *netavarkNetwork) RunInRootlessNetns(toRun func() error) error {
	if n.rootlessNetns == nil {
		return types.ErrNotRootlessNetns
	}
	return n.rootlessNetns.Run(n.lock, toRun)
}

func (n *netavarkNetwork) RootlessNetnsInfo() (*types.RootlessNetnsInfo, error) {
	if n.rootlessNetns == nil {
		return nil, types.ErrNotRootlessNetns
	}
	return n.rootlessNetns.Info(), nil
}

func (n *netavarkNetwork) PestoSocketPath() string {
	if n.rootlessNetns == nil {
		logrus.Debug("PestoSocketPath: rootlessNetns is nil")
		return ""
	}
	return n.rootlessNetns.PestoSocketPath()
}

// firstIPsFromStatus picks the first IPv4 and IPv6 container addresses from a
// set of network StatusBlocks, iterating in the order given by networkOrder
// (connection time). Returns the IP strings and the owning network name.
func firstIPsFromStatus(status map[string]types.StatusBlock, networkOrder []string) (ipv4, ipv4Net, ipv6, ipv6Net string) {
	for _, name := range networkOrder {
		sb, ok := status[name]
		if !ok {
			continue
		}
		for _, netInt := range sb.Interfaces {
			for _, netAddr := range netInt.Subnets {
				ip := netAddr.IPNet.IP
				if ip.To4() != nil && ipv4 == "" {
					ipv4 = ip.String()
					ipv4Net = name
				} else if ip.To4() == nil && ipv6 == "" {
					ipv6 = ip.String()
					ipv6Net = name
				}
			}
		}
		if ipv4 != "" && ipv6 != "" {
			break
		}
	}
	return
}
