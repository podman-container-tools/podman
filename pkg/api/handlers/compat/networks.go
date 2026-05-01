//go:build !remote && (linux || freebsd)

package compat

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"

	nettypes "go.podman.io/common/libnetwork/types"
	netutil "go.podman.io/common/libnetwork/util"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/util"

	dockerNetwork "github.com/moby/moby/api/types/network"
	"github.com/sirupsen/logrus"
)

// leaseRangeToDockerIPRange maps Libpod LeaseRange back to Docker IPAM IPRange when it is exactly
// the usable span of some CIDR (same semantics as FirstIPInSubnet/LastIPInSubnet used when creating from Docker API).
func leaseRangeToDockerIPRange(lr *nettypes.LeaseRange) netip.Prefix {
	if lr == nil || lr.StartIP == nil || lr.EndIP == nil {
		return netip.Prefix{}
	}
	start := append(net.IP(nil), lr.StartIP...)
	end := append(net.IP(nil), lr.EndIP...)
	netutil.NormalizeIP(&start)
	netutil.NormalizeIP(&end)

	switch {
	case start.To4() != nil && end.To4() != nil:
		return leaseSpanToPrefix(start.To4(), end.To4(), 32)
	case len(start) == net.IPv6len && len(end) == net.IPv6len:
		s := start.To16()
		e := end.To16()
		return leaseSpanToPrefix(s, e, 128)
	default:
		return netip.Prefix{}
	}
}

func leaseSpanToPrefix(start, end net.IP, ipLen int) netip.Prefix {
	if bytes.Compare(start, end) > 0 {
		return netip.Prefix{}
	}
	for ones := ipLen; ones >= 0; ones-- {
		mask := net.CIDRMask(ones, ipLen)
		ipNet := &net.IPNet{
			IP:   start.Mask(mask),
			Mask: mask,
		}
		first, err1 := netutil.FirstIPInSubnet(ipNet)
		last, err2 := netutil.LastIPInSubnet(ipNet)
		if err1 != nil || err2 != nil {
			continue
		}
		netutil.NormalizeIP(&first)
		netutil.NormalizeIP(&last)
		if first.Equal(start) && last.Equal(end) {
			pfx, err := netip.ParsePrefix(ipNet.String())
			if err != nil {
				continue
			}
			return pfx
		}
	}
	return netip.Prefix{}
}

func normalizeNetworkName(rt *libpod.Runtime, name string) (string, bool) {
	if name == nettypes.BridgeNetworkDriver {
		return rt.Network().DefaultNetworkName(), true
	}
	return name, false
}

func InspectNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	// scope is only used to see if the user passes any illegal value, verbose is not used but implemented
	// for compatibility purposes only.
	query := struct {
		scope   string `schema:"scope"`
		verbose bool   `schema:"verbose"`
	}{
		scope: "local",
	}
	decoder := utils.GetDecoder(r)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if query.scope != "local" {
		utils.Error(w, http.StatusBadRequest, define.ErrInvalidArg)
		return
	}
	name, changed := normalizeNetworkName(runtime, utils.GetName(r))
	net, err := runtime.Network().NetworkInspect(name)
	if err != nil {
		utils.NetworkNotFound(w, name, err)
		return
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	statuses, err := ic.GetContainerNetStatuses()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	report, err := convertLibpodNetworktoDockerNetwork(runtime, statuses, &net, changed)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func convertLibpodNetworktoDockerNetwork(runtime *libpod.Runtime, statuses []abi.ContainerNetStatus, network *nettypes.Network, changeDefaultName bool) (*dockerNetwork.Inspect, error) {
	containerEndpoints := make(map[string]dockerNetwork.EndpointResource, len(statuses))
	for _, st := range statuses {
		if netData, ok := st.Status[network.Name]; ok {
			var err error
			var ipv4_pfx netip.Prefix
			var ipv6_pfx netip.Prefix
			macAddr := nettypes.HardwareAddr{}
			for _, dev := range netData.Interfaces {
				for _, subnet := range dev.Subnets {
					// Note the docker API really wants the full CIDR subnet not just a single ip.
					// https://github.com/containers/podman/pull/12328
					if netutil.IsIPv4(subnet.IPNet.IP) {
						ipv4_pfx, err = netip.ParsePrefix(subnet.IPNet.String())
						if err != nil {
							return nil, fmt.Errorf("invalid IPv4Address %q: %w", subnet.IPNet.String(), err)
						}
					} else {
						ipv6_pfx, err = netip.ParsePrefix(subnet.IPNet.String())
						if err != nil {
							return nil, fmt.Errorf("invalid IPv6Address %q: %w", subnet.IPNet.String(), err)
						}
					}
				}
				macAddr = dev.MacAddress
				break
			}

			containerEndpoint := dockerNetwork.EndpointResource{
				Name:        st.Name,
				MacAddress:  dockerNetwork.HardwareAddr(net.HardwareAddr(macAddr)),
				IPv4Address: ipv4_pfx,
				IPv6Address: ipv6_pfx,
			}
			containerEndpoints[st.ID] = containerEndpoint
		}
	}
	ipamConfigs := make([]dockerNetwork.IPAMConfig, 0, len(network.Subnets))
	for _, sub := range network.Subnets {
		subnet, err := netip.ParsePrefix(sub.Subnet.String())
		if err != nil {
			return nil, err
		}
		gateway, ok := netip.AddrFromSlice(sub.Gateway)
		if !ok {
			return nil, fmt.Errorf("invalid gateway IP %v", sub.Gateway)
		}
		ipamConfig := dockerNetwork.IPAMConfig{
			Subnet:   subnet,
			Gateway:  gateway,
			IPRange:  leaseRangeToDockerIPRange(sub.LeaseRange),
		}
		ipamConfigs = append(ipamConfigs, ipamConfig)
	}
	ipamDriver := network.IPAMOptions["driver"]
	if ipamDriver == nettypes.HostLocalIPAMDriver {
		ipamDriver = "default"
	}
	ipam := dockerNetwork.IPAM{
		Driver:  ipamDriver,
		Options: network.IPAMOptions,
		Config:  ipamConfigs,
	}

	name := network.Name
	if changeDefaultName && name == runtime.Network().DefaultNetworkName() {
		name = nettypes.BridgeNetworkDriver
	}

	report := dockerNetwork.Inspect{
		Network: dockerNetwork.Network{
			Name:       name,
			ID:         network.ID,
			Driver:     network.Driver,
			Created:    network.Created,
			Scope:      "local",
			EnableIPv4: true,
			EnableIPv6: network.IPv6Enabled,
			IPAM:       ipam,
			Internal:   network.Internal,
			Attachable: false,
			Ingress:    false,
			ConfigFrom: dockerNetwork.ConfigReference{},
			ConfigOnly: false,
			Options:    network.Options,
			Labels:     network.Labels,
			Peers:      nil,
		},
		Containers: containerEndpoints,
		Services:   nil,
	}
	return &report, nil
}

func ListNetworks(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	options := entities.NetworkListOptions{
		Filters: *filterMap,
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	nets, err := ic.NetworkList(r.Context(), options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	statuses, err := ic.GetContainerNetStatuses()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	reports := make([]*dockerNetwork.Summary, 0, len(nets))
	for _, net := range nets {
		report, err := convertLibpodNetworktoDockerNetwork(runtime, statuses, &net, true)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		reports = append(reports, &dockerNetwork.Summary{
			Network: report.Network,
		})
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func CreateNetwork(w http.ResponseWriter, r *http.Request) {
	var (
		networkCreate   dockerNetwork.CreateRequest
		network         nettypes.Network
		responseWarning string
	)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	if err := utils.ReadJSONFromBody(r, &networkCreate); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	network.Name = networkCreate.Name
	if networkCreate.Driver == "" {
		networkCreate.Driver = nettypes.DefaultNetworkDriver
	}
	network.Driver = networkCreate.Driver
	network.Labels = networkCreate.Labels
	network.Internal = networkCreate.Internal
	if networkCreate.EnableIPv6 != nil {
		network.IPv6Enabled = *networkCreate.EnableIPv6
	}

	network.Options = make(map[string]string)

	for opt, optVal := range networkCreate.Options {
		switch opt {
		case nettypes.MTUOption:
			fallthrough
		case "com.docker.network.driver.mtu":
			network.Options[nettypes.MTUOption] = optVal
		case "com.docker.network.bridge.name":
			if network.Driver == nettypes.BridgeNetworkDriver {
				network.NetworkInterface = optVal
			}
		case nettypes.ModeOption:
			if network.Driver == nettypes.MacVLANNetworkDriver || network.Driver == nettypes.IPVLANNetworkDriver {
				network.Options[opt] = optVal
			}
		case "parent":
			if network.Driver == nettypes.MacVLANNetworkDriver || network.Driver == nettypes.IPVLANNetworkDriver {
				network.NetworkInterface = optVal
			}
		default:
			responseWarning = "\"" + opt + ": " + optVal + "\" is not a recognized option"
		}
	}

	// dns is only enabled for the bridge driver
	if network.Driver == nettypes.BridgeNetworkDriver {
		network.DNSEnabled = true
	}

	if networkCreate.IPAM != nil && len(networkCreate.IPAM.Config) > 0 {
		for _, conf := range networkCreate.IPAM.Config {
			s := nettypes.Subnet{}
			if conf.Subnet.IsValid() {
				pfx := conf.Subnet.Masked()
				addr := pfx.Addr()

				s.Subnet = nettypes.IPNet{
					IPNet: net.IPNet{
						IP:   net.IP(addr.AsSlice()),
						Mask: net.CIDRMask(pfx.Bits(), addr.BitLen()),
					},
				}
			}
			if conf.Gateway.IsValid() {
				s.Gateway = net.IP(conf.Gateway.AsSlice())
			}
			if conf.IPRange.IsValid() {
				_, net, err := net.ParseCIDR(conf.IPRange.String())
				if err != nil {
					utils.InternalServerError(w, fmt.Errorf("failed to parse ip range: %w", err))
					return
				}
				startIP, err := netutil.FirstIPInSubnet(net)
				if err != nil {
					utils.InternalServerError(w, fmt.Errorf("failed to get first ip in range: %w", err))
					return
				}
				lastIP, err := netutil.LastIPInSubnet(net)
				if err != nil {
					utils.InternalServerError(w, fmt.Errorf("failed to get last ip in range: %w", err))
					return
				}
				s.LeaseRange = &nettypes.LeaseRange{
					StartIP: startIP,
					EndIP:   lastIP,
				}
			}
			network.Subnets = append(network.Subnets, s)
		}
		// FIXME can we use the IPAM driver and options?
	}

	opts := nettypes.NetworkCreateOptions{
		// networkCreate.CheckDuplicate is deprecated since API v1.44,
		// but it defaults to true when sent by the client package to
		// older daemons.
		IgnoreIfExists: false,
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	newNetwork, err := ic.NetworkCreate(r.Context(), network, &opts)
	if err != nil {
		if errors.Is(err, nettypes.ErrNetworkExists) {
			utils.Error(w, http.StatusConflict, err)
		} else {
			utils.InternalServerError(w, err)
		}
		return
	}

	body := struct {
		ID      string `json:"Id"`
		Warning string `json:"Warning"`
	}{
		ID:      newNetwork.ID,
		Warning: responseWarning,
	}
	utils.WriteResponse(w, http.StatusCreated, body)
}

func RemoveNetwork(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}

	query := struct {
		Force   bool  `schema:"force"`
		Timeout *uint `schema:"timeout"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	decoder := utils.GetDecoder(r)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	options := entities.NetworkRmOptions{
		Force:   query.Force,
		Timeout: query.Timeout,
	}

	name, _ := normalizeNetworkName(runtime, utils.GetName(r))
	reports, err := ic.NetworkRm(r.Context(), []string{name}, options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if len(reports) == 0 {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("internal error"))
		return
	}
	report := reports[0]
	if report.Err != nil {
		if errors.Is(report.Err, define.ErrNoSuchNetwork) {
			utils.Error(w, http.StatusNotFound, define.ErrNoSuchNetwork)
			return
		}
		utils.InternalServerError(w, report.Err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, nil)
}

// Connect adds a container to a network
func Connect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	var netConnect dockerNetwork.ConnectRequest
	if err := utils.ReadJSONFromBody(r, &netConnect); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	netOpts := nettypes.PerNetworkOptions{}

	name, _ := normalizeNetworkName(runtime, utils.GetName(r))
	if netConnect.EndpointConfig != nil {
		if netConnect.EndpointConfig.Aliases != nil {
			netOpts.Aliases = netConnect.EndpointConfig.Aliases
		}

		if netConnect.EndpointConfig.IPAMConfig != nil {
			ipam := netConnect.EndpointConfig.IPAMConfig

			// IPv4
			if ipam.IPv4Address.IsValid() {
				netOpts.StaticIPs = append(
					netOpts.StaticIPs,
					net.IP(ipam.IPv4Address.AsSlice()),
				)
			}

			// IPv6
			if ipam.IPv6Address.IsValid() {
				netOpts.StaticIPs = append(
					netOpts.StaticIPs,
					net.IP(ipam.IPv6Address.AsSlice()),
				)
			}
		}

		// If MAC address is provided
		if len(netConnect.EndpointConfig.MacAddress) != 0 {
			netOpts.StaticMAC = nettypes.HardwareAddr(
				net.HardwareAddr(netConnect.EndpointConfig.MacAddress),
			)
		}
	}
	err := runtime.ConnectContainerToNetwork(netConnect.Container, name, netOpts)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, netConnect.Container, err)
			return
		}
		if errors.Is(err, define.ErrNoSuchNetwork) {
			utils.Error(w, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, define.ErrNetworkConnected) {
			utils.Error(w, http.StatusForbidden, err)
			return
		}
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "OK")
}

// Disconnect removes a container from a network
func Disconnect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	var netDisconnect dockerNetwork.DisconnectRequest
	if err := utils.ReadJSONFromBody(r, &netDisconnect); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	name, _ := normalizeNetworkName(runtime, utils.GetName(r))
	err := runtime.DisconnectContainerFromNetwork(netDisconnect.Container, name, netDisconnect.Force)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.Error(w, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, define.ErrNoSuchNetwork) {
			utils.Error(w, http.StatusNotFound, err)
			return
		}
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "OK")
}

// Prune removes unused networks
func Prune(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("Decode(): %w", err))
		return
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	pruneOptions := entities.NetworkPruneOptions{
		Filters: *filterMap,
	}
	pruneReports, err := ic.NetworkPrune(r.Context(), pruneOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	type response struct {
		NetworksDeleted []string
	}
	prunedNetworks := []string{}
	for _, pr := range pruneReports {
		if pr.Error != nil {
			logrus.Error(pr.Error)
			continue
		}
		prunedNetworks = append(prunedNetworks, pr.Name)
	}
	utils.WriteResponse(w, http.StatusOK, response{NetworksDeleted: prunedNetworks})
}
