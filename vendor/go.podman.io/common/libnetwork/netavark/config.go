//go:build linux || freebsd

package netavark

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"

	internalutil "go.podman.io/common/libnetwork/internal/util"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
	"go.podman.io/storage/pkg/stringid"
)

func sliceRemoveDuplicates(strList []string) []string {
	list := make([]string, 0, len(strList))
	for _, item := range strList {
		if !slices.Contains(list, item) {
			list = append(list, item)
		}
	}
	return list
}

func (n *netavarkNetwork) commitNetwork(network *types.Network) error {
	if err := os.MkdirAll(n.networkConfigDir, 0o755); err != nil {
		return nil
	}
	confPath := filepath.Join(n.networkConfigDir, network.Name+".json")
	f, err := os.Create(confPath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "     ")
	err = enc.Encode(network)
	if err != nil {
		return err
	}
	return nil
}

func (n *netavarkNetwork) NetworkUpdate(name string, options types.NetworkUpdateOptions) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return err
	}
	network, err := n.getNetwork(name)
	if err != nil {
		return err
	}
	// Nameservers must be IP Addresses.
	for _, dnsServer := range options.AddDNSServers {
		if net.ParseIP(dnsServer) == nil {
			return fmt.Errorf("unable to parse ip %s specified in AddDNSServer: %w", dnsServer, types.ErrInvalidArg)
		}
	}
	for _, dnsServer := range options.RemoveDNSServers {
		if net.ParseIP(dnsServer) == nil {
			return fmt.Errorf("unable to parse ip %s specified in RemoveDNSServer: %w", dnsServer, types.ErrInvalidArg)
		}
	}
	networkDNSServersBefore := network.NetworkDNSServers
	networkDNSServersAfter := []string{}
	for _, server := range networkDNSServersBefore {
		if slices.Contains(options.RemoveDNSServers, server) {
			continue
		}
		networkDNSServersAfter = append(networkDNSServersAfter, server)
	}
	networkDNSServersAfter = append(networkDNSServersAfter, options.AddDNSServers...)
	networkDNSServersAfter = sliceRemoveDuplicates(networkDNSServersAfter)
	network.NetworkDNSServers = networkDNSServersAfter
	if slices.Equal(networkDNSServersBefore, networkDNSServersAfter) {
		return nil
	}
	err = n.commitNetwork(network)
	if err != nil {
		return err
	}

	return n.execUpdate(network.Name, network.NetworkDNSServers)
}

// NetworkCreate will take a partial filled Network and fill the
// missing fields. It creates the Network and returns the full Network.
func (n *netavarkNetwork) NetworkCreate(net types.Network, options *types.NetworkCreateOptions) (types.Network, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return types.Network{}, err
	}

	if options != nil && options.IgnoreIfExists {
		if network, ok := n.networks[net.Name]; ok {
			return *network, nil
		}
	}
	network, err := n.networkCreate(&net, false)
	if err != nil {
		return types.Network{}, err
	}
	// add the new network to the map
	n.networks[network.Name] = network
	return *network, nil
}

func (n *netavarkNetwork) networkCreate(newNetwork *types.Network, defaultNet bool) (*types.Network, error) {
	// if no driver is set use the default one
	if newNetwork.Driver == "" {
		newNetwork.Driver = types.DefaultNetworkDriver
	}
	if !defaultNet {
		// FIXME: Should we use a different type for network create without the ID field?
		// the caller is not allowed to set a specific ID
		if newNetwork.ID != "" {
			return nil, fmt.Errorf("ID can not be set for network create: %w", types.ErrInvalidArg)
		}

		// generate random network ID
		var i int
		for i = range 1000 {
			id := stringid.GenerateNonCryptoID()
			if _, err := n.getNetwork(id); err != nil {
				newNetwork.ID = id
				break
			}
		}
		if i == 1000 {
			return nil, errors.New("failed to create random network ID")
		}
	}
	if newNetwork.Name == "" {
		name, err := internalutil.GetFreeDeviceName(n)
		if err != nil {
			return nil, err
		}
		newNetwork.Name = name
	}

	usedSubnets, err := internalutil.GetUsedSubnets(n)
	if err != nil {
		return nil, err
	}

	usedNames := make(map[string]string, len(n.networks))
	for name, network := range n.networks {
		usedNames[name] = network.ID
	}

	type Used struct {
		Interfaces []string          `json:"interfaces"`
		Names      map[string]string `json:"names"`
		Subnets    []types.IPNet     `json:"subnets"`
	}

	type ConfigOpts struct {
		SubnetPools          []config.SubnetPool `json:"subnet_pools"`
		DefaultInterfaceName string              `json:"default_interface_name"`
		CheckUsedSubnets     bool                `json:"check_used_subnets"`
	}

	type CreateConfigOptions struct {
		Network types.Network `json:"network"`
		Used    Used          `json:"used"`
		Options ConfigOpts    `json:"options"`
	}

	subnets := make([]types.IPNet, len(usedSubnets))
	for i, subnet := range usedSubnets {
		subnets[i] = types.IPNet{IPNet: *subnet}
	}

	opts := CreateConfigOptions{
		Network: *newNetwork,
		Used: Used{
			Interfaces: internalutil.GetBridgeInterfaceNames(n),
			Names:      usedNames,
			Subnets:    subnets,
		},
		Options: ConfigOpts{
			SubnetPools:          n.defaultsubnetPools,
			DefaultInterfaceName: n.DefaultInterfaceName(),
			CheckUsedSubnets:     !defaultNet,
		},
	}
	var needsPlugin bool
	if !slices.Contains(builtinDrivers, newNetwork.Driver) {
		needsPlugin = true
	}
	var result *types.Network
	err = n.execNetavark([]string{"create"}, needsPlugin, &opts, &result)
	if err != nil {
		if strings.Contains(err.Error(), "network already exists") {
			return nil, fmt.Errorf("%w%.0w", err, types.ErrNetworkExists)
		}
		return nil, err
	}

	// normalize network fields
	if err := parseNetwork(result); err != nil {
		return nil, err
	}

	if !defaultNet {
		err := n.commitNetwork(result)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// NetworkRemove will remove the Network with the given name or ID.
// It does not ensure that the network is unused.
func (n *netavarkNetwork) NetworkRemove(nameOrID string) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return err
	}

	network, err := n.getNetwork(nameOrID)
	if err != nil {
		return err
	}

	// Removing the default network is not allowed.
	if network.Name == n.defaultNetwork {
		return fmt.Errorf("default network %s cannot be removed", n.defaultNetwork)
	}

	// remove the ipam bucket for this network
	if err := n.removeNetworkIPAMBucket(network); err != nil {
		return err
	}

	file := filepath.Join(n.networkConfigDir, network.Name+".json")
	// make sure to not error for ErrNotExist
	if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(n.networks, network.Name)
	return nil
}

// NetworkList will return all known Networks. Optionally you can
// supply a list of filter functions. Only if a network matches all
// functions it is returned.
func (n *netavarkNetwork) NetworkList(filters ...types.FilterFunc) ([]types.Network, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return nil, err
	}

	networks := make([]types.Network, 0, len(n.networks))
outer:
	for _, net := range n.networks {
		for _, filter := range filters {
			// All filters have to match, if one does not match we can skip to the next network.
			if !filter(*net) {
				continue outer
			}
		}
		networks = append(networks, *net)
	}
	return networks, nil
}

// NetworkInspect will return the Network with the given name or ID.
func (n *netavarkNetwork) NetworkInspect(nameOrID string) (types.Network, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return types.Network{}, err
	}

	network, err := n.getNetwork(nameOrID)
	if err != nil {
		return types.Network{}, err
	}
	return *network, nil
}

func getAllPlugins(dirs []string) []string {
	var plugins []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if !slices.Contains(plugins, name) {
					plugins = append(plugins, name)
				}
			}
		}
	}
	return plugins
}
