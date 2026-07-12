//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"go.podman.io/common/libnetwork/pasta"
	"go.podman.io/common/libnetwork/types"
	netutil "go.podman.io/common/libnetwork/util"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) NetworkUpdate(_ context.Context, netName string, options entities.NetworkUpdateOptions) error {
	var networkUpdateOptions types.NetworkUpdateOptions
	networkUpdateOptions.AddDNSServers = options.AddDNSServers
	networkUpdateOptions.RemoveDNSServers = options.RemoveDNSServers
	err := ic.Libpod.Network().NetworkUpdate(netName, networkUpdateOptions)
	if err != nil {
		return err
	}
	return nil
}

func (ic *ContainerEngine) NetworkList(_ context.Context, options entities.NetworkListOptions) ([]types.Network, error) {
	// dangling filter is not provided by netutil
	var wantDangling bool

	val, filterDangling := options.Filters["dangling"]
	if filterDangling {
		switch len(val) {
		case 0:
			return nil, fmt.Errorf("got no values for filter key \"dangling\"")
		case 1:
			var err error
			wantDangling, err = strconv.ParseBool(val[0])
			if err != nil {
				return nil, fmt.Errorf("invalid dangling filter value \"%v\"", val[0])
			}
			delete(options.Filters, "dangling")
		default:
			return nil, fmt.Errorf("got more than one value for filter key \"dangling\"")
		}
	}

	filters, err := netutil.GenerateNetworkFilters(options.Filters)
	if err != nil {
		return nil, err
	}

	if filterDangling {
		danglingFilterFunc, err := ic.createDanglingFilterFunc(wantDangling)
		if err != nil {
			return nil, err
		}

		filters = append(filters, danglingFilterFunc)
	}
	nets, err := ic.Libpod.Network().NetworkList(filters...)
	return nets, err
}

func (ic *ContainerEngine) NetworkInspect(_ context.Context, namesOrIds []string, _ entities.InspectOptions) ([]entities.NetworkInspectReport, []error, error) {
	var errs []error
	statuses, err := ic.GetContainerNetStatuses()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get network status for containers: %w", err)
	}
	networks := make([]entities.NetworkInspectReport, 0, len(namesOrIds))
	for _, name := range namesOrIds {
		net, err := ic.Libpod.Network().NetworkInspect(name)
		if err != nil {
			if errors.Is(err, define.ErrNoSuchNetwork) {
				errs = append(errs, fmt.Errorf("network %s: %w", name, err))
				continue
			} else {
				return nil, nil, fmt.Errorf("inspecting network %s: %w", name, err)
			}
		}
		containerMap := make(map[string]entities.NetworkContainerInfo)
		for _, st := range statuses {
			// Make sure to only show the info for the correct network
			if sb, ok := st.Status[net.Name]; ok {
				containerMap[st.ID] = entities.NetworkContainerInfo{
					Name:       st.Name,
					Interfaces: sb.Interfaces,
				}
			}
		}

		netReport := entities.NetworkInspectReport{
			Network:    net,
			Containers: containerMap,
		}
		networks = append(networks, netReport)
	}
	return networks, errs, nil
}

func (ic *ContainerEngine) NetworkReload(_ context.Context, names []string, options entities.NetworkReloadOptions) ([]*entities.NetworkReloadReport, error) {
	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: names})
	if err != nil {
		return nil, err
	}

	reports := make([]*entities.NetworkReloadReport, 0, len(containers))
	for _, ctr := range containers {
		report := new(entities.NetworkReloadReport)
		report.Id = ctr.ID()
		report.Err = ctr.ReloadNetwork()
		// ignore errors for invalid ctr state and network mode when --all is used
		if options.All && (errors.Is(report.Err, define.ErrCtrStateInvalid) ||
			errors.Is(report.Err, define.ErrNetworkModeInvalid)) {
			continue
		}
		reports = append(reports, report)
	}

	return reports, nil
}

func (ic *ContainerEngine) NetworkRm(ctx context.Context, namesOrIds []string, options entities.NetworkRmOptions) ([]*entities.NetworkRmReport, error) {
	reports := make([]*entities.NetworkRmReport, 0, len(namesOrIds))
	for _, name := range namesOrIds {
		report := entities.NetworkRmReport{Name: name}
		containers, err := ic.Libpod.GetAllContainers()
		if err != nil {
			return reports, err
		}
		// We need to iterate containers looking to see if they belong to the given network
		for _, c := range containers {
			networks, err := c.Networks()
			// if container vanished or network does not exist, go to next container
			if errors.Is(err, define.ErrNoSuchNetwork) || errors.Is(err, define.ErrNoSuchCtr) {
				continue
			}
			if err != nil {
				return reports, err
			}
			if slices.Contains(networks, name) {
				// if user passes force, we nuke containers and pods
				if !options.Force {
					// Without the force option, we return an error
					return reports, fmt.Errorf("%q has associated containers with it. Use -f to forcibly delete containers and pods: %w", name, define.ErrNetworkInUse)
				}
				if c.IsInfra() {
					// if we have an infra container we need to remove the pod
					pod, err := ic.Libpod.GetPod(c.PodID())
					if err != nil {
						return reports, err
					}
					if _, err := ic.Libpod.RemovePod(ctx, pod, true, true, options.Timeout); err != nil {
						return reports, err
					}
				} else if err := ic.Libpod.RemoveContainer(ctx, c, true, true, options.Timeout); err != nil && !errors.Is(err, define.ErrNoSuchCtr) {
					return reports, err
				}
			}
		}
		net, err := ic.Libpod.Network().NetworkInspect(name)
		if err != nil && !errors.Is(err, define.ErrNoSuchNetwork) {
			return reports, err
		}
		if err := ic.Libpod.Network().NetworkRemove(name); err != nil {
			if options.Ignore && errors.Is(err, define.ErrNoSuchNetwork) {
				continue
			}
			report.Err = err
		}
		if len(net.Name) != 0 {
			ic.Libpod.NewNetworkEvent(events.Remove, net.Name, net.ID, net.Driver)
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) NetworkCreate(_ context.Context, network types.Network, createOptions *types.NetworkCreateOptions) (*types.Network, error) {
	if slices.Contains([]string{"none", "host", "bridge", "private", "slirp4netns", pasta.BinaryName, "container", "ns", "default"}, network.Name) {
		return nil, fmt.Errorf("cannot create network with name %q because it conflicts with a valid network mode", network.Name)
	}
	network, err := ic.Libpod.Network().NetworkCreate(network, createOptions)
	if err != nil {
		return nil, err
	}
	ic.Libpod.NewNetworkEvent(events.Create, network.Name, network.ID, network.Driver)
	return &network, nil
}

// NetworkDisconnect removes a container from a given network
func (ic *ContainerEngine) NetworkDisconnect(_ context.Context, networkname string, options entities.NetworkDisconnectOptions) error {
	return ic.Libpod.DisconnectContainerFromNetwork(options.Container, networkname, options.Force)
}

func (ic *ContainerEngine) NetworkConnect(_ context.Context, networkname string, options entities.NetworkConnectOptions) error {
	return ic.Libpod.ConnectContainerToNetwork(options.Container, networkname, options.PerNetworkOptions)
}

// NetworkExists checks if the given network exists
func (ic *ContainerEngine) NetworkExists(_ context.Context, networkname string) (*entities.BoolReport, error) {
	_, err := ic.Libpod.Network().NetworkInspect(networkname)
	exists := true
	// if err is ErrNoSuchNetwork do not return it
	if errors.Is(err, define.ErrNoSuchNetwork) {
		exists = false
	} else if err != nil {
		return nil, err
	}
	return &entities.BoolReport{
		Value: exists,
	}, nil
}

// Network prune removes unused networks
func (ic *ContainerEngine) NetworkPrune(_ context.Context, options entities.NetworkPruneOptions) ([]*entities.NetworkPruneReport, error) {
	// get all filters
	filters, err := netutil.GenerateNetworkPruneFilters(options.Filters)
	if err != nil {
		return nil, err
	}
	danglingFilterFunc, err := ic.createDanglingFilterFunc(true)
	if err != nil {
		return nil, err
	}
	filters = append(filters, danglingFilterFunc)
	nets, err := ic.Libpod.Network().NetworkList(filters...)
	if err != nil {
		return nil, err
	}

	pruneReport := make([]*entities.NetworkPruneReport, 0, len(nets))
	for _, net := range nets {
		pruneReport = append(pruneReport, &entities.NetworkPruneReport{
			Name:  net.Name,
			Error: ic.Libpod.Network().NetworkRemove(net.Name),
		})
	}
	return pruneReport, nil
}

// danglingFilter function is special and not implemented in libnetwork filters
func (ic *ContainerEngine) createDanglingFilterFunc(wantDangling bool) (types.FilterFunc, error) {
	cons, err := ic.Libpod.GetAllContainers()
	if err != nil {
		return nil, err
	}
	// Gather up all the non-default networks that the
	// containers want
	networksToKeep := make(map[string]bool)
	for _, c := range cons {
		nets, err := c.Networks()
		if err != nil {
			return nil, err
		}
		for _, n := range nets {
			networksToKeep[n] = true
		}
	}
	// ignore the default network, this one cannot be deleted
	networksToKeep[ic.Libpod.GetDefaultNetworkName()] = true

	return func(net types.Network) bool {
		for network := range networksToKeep {
			if network == net.Name {
				return !wantDangling
			}
		}
		return wantDangling
	}, nil
}

type ContainerNetStatus struct {
	// Name of the container
	Name string
	// ID of the container
	ID string
	// Status contains the net status, the key is the network name
	Status map[string]types.StatusBlock
}

func (ic *ContainerEngine) GetContainerNetStatuses() ([]ContainerNetStatus, error) {
	cons, err := ic.Libpod.GetAllContainers()
	if err != nil {
		return nil, err
	}
	statuses := make([]ContainerNetStatus, 0, len(cons))
	for _, con := range cons {
		status, err := con.GetNetworkStatus()
		if err != nil {
			if errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrRemoved) {
				continue
			}
			return nil, err
		}

		statuses = append(statuses, ContainerNetStatus{
			ID:     con.ID(),
			Name:   con.Name(),
			Status: status,
		})
	}
	return statuses, nil
}
