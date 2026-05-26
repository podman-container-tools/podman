//go:build !remote && (linux || freebsd)

package filters

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/filters"
	"go.podman.io/common/pkg/util"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities/types"
	"go.podman.io/storage"
)

// GenerateContainerFilterFuncs return ContainerFilter functions based of filter.
func GenerateContainerFilterFuncs(filter string, filterValues []string, r *libpod.Runtime) (func(container *libpod.Container) bool, error) {
	switch filter {
	case "id":
		return func(c *libpod.Container) bool {
			return filters.FilterID(c.ID(), filterValues)
		}, nil
	case "label":
		// we have to match that all given labels exits on that container
		return func(c *libpod.Container) bool {
			return filters.MatchLabelFilters(filterValues, c.Labels())
		}, nil
	case "label!":
		return func(c *libpod.Container) bool {
			return filters.MatchNegatedLabelFilters(filterValues, c.Labels())
		}, nil
	case "annotation":
		return func(c *libpod.Container) bool {
			return filters.MatchLabelFilters(filterValues, c.ConfigNoCopy().Spec.Annotations)
		}, nil
	case "annotation!":
		return func(c *libpod.Container) bool {
			return filters.MatchNegatedLabelFilters(filterValues, c.ConfigNoCopy().Spec.Annotations)
		}, nil
	case "name":
		// we only have to match one name
		return func(c *libpod.Container) bool {
			filters := make([]string, 0, len(filterValues))
			for _, f := range filterValues {
				filters = append(filters, strings.ReplaceAll(f, "/", ""))
			}
			return util.StringMatchRegexSlice(c.Name(), filters)
		}, nil
	case "exited":
		var exitCodes []int32
		for _, exitCode := range filterValues {
			ec, err := strconv.ParseInt(exitCode, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("exited code out of range %q: %w", ec, err)
			}
			exitCodes = append(exitCodes, int32(ec))
		}
		return func(c *libpod.Container) bool {
			ec, exited, err := c.ExitCode()
			if err == nil && exited {
				if slices.Contains(exitCodes, ec) {
					return true
				}
			}
			return false
		}, nil
	case "status":
		for _, filterValue := range filterValues {
			if _, err := define.StringToContainerStatus(filterValue); err != nil {
				return nil, err
			}
		}
		return func(c *libpod.Container) bool {
			status, err := c.State()
			if err != nil {
				return false
			}
			state := status.String()
			if status == define.ContainerStateStopped {
				state = "exited"
			}
			for _, filterValue := range filterValues {
				if filterValue == "stopped" {
					filterValue = "exited"
				}
				if state == filterValue {
					return true
				}
			}
			return false
		}, nil
	case "ancestor":
		// This needs to refine to match docker
		// - ancestor=(<image-name>[:tag]|<image-id>| ⟨image@digest⟩) - containers created from an image or a descendant.
		return func(c *libpod.Container) bool {
			for _, filterValue := range filterValues {
				rootfsImageID, rootfsImageName := c.Image()
				var imageTag string
				var imageNameWithoutTag string
				// Compare with ImageID, ImageName
				// Will match ImageName if running image has tag latest for other tags exact complete filter must be given
				name, tag, hasColon := strings.Cut(rootfsImageName, ":")
				if hasColon {
					imageNameWithoutTag = name
					imageTag = tag
				}

				// Check for substring match on image ID (Docker compatibility)
				if strings.Contains(rootfsImageID, filterValue) {
					return true
				}

				// Check for regex match (advanced use cases)
				if util.StringMatchRegexSlice(rootfsImageName, filterValues) ||
					(util.StringMatchRegexSlice(imageNameWithoutTag, filterValues) && imageTag == "latest") {
					return true
				}
			}
			return false
		}, nil
	case "before":
		var createTime time.Time
		for _, filterValue := range filterValues {
			ctr, err := r.LookupContainer(filterValue)
			if err != nil {
				return nil, err
			}
			if createTime.IsZero() || createTime.After(ctr.CreatedTime()) {
				createTime = ctr.CreatedTime()
			}
		}
		return func(c *libpod.Container) bool {
			return createTime.After(c.CreatedTime())
		}, nil
	case "since":
		var createTime time.Time
		for _, filterValue := range filterValues {
			ctr, err := r.LookupContainer(filterValue)
			if err != nil {
				return nil, err
			}
			if createTime.IsZero() || createTime.After(ctr.CreatedTime()) {
				createTime = ctr.CreatedTime()
			}
		}
		return func(c *libpod.Container) bool {
			return createTime.Before(c.CreatedTime())
		}, nil
	case "volume":
		//- volume=(<volume-name>|<mount-point-destination>)
		return func(c *libpod.Container) bool {
			containerConfig := c.ConfigNoCopy()
			for _, filterValue := range filterValues {
				source, dest, _ := strings.Cut(filterValue, ":")
				for _, mount := range containerConfig.Spec.Mounts {
					if dest != "" && (mount.Source == source && mount.Destination == dest) {
						return true
					}
					if dest == "" && mount.Destination == source {
						return true
					}
				}
				for _, vname := range containerConfig.NamedVolumes {
					if dest != "" && (vname.Name == source && vname.Dest == dest) {
						return true
					}
					if dest == "" && vname.Name == source {
						return true
					}
				}
			}
			return false
		}, nil
	case "health":
		return func(c *libpod.Container) bool {
			hcStatus, err := c.HealthCheckStatus()
			if err != nil {
				logrus.Warnf("Healthcheck status for %v can't be determined, status won't be matched: %v", c.ID(), err)
				return false
			}
			return slices.Contains(filterValues, hcStatus)
		}, nil
	case "until":
		return prepareUntilFilterFunc(filterValues)
	case "pod":
		var pods []*libpod.Pod
		for _, podNameOrID := range filterValues {
			p, err := r.LookupPod(podNameOrID)
			if err != nil {
				if errors.Is(err, define.ErrNoSuchPod) {
					continue
				}
				return nil, err
			}
			pods = append(pods, p)
		}
		return func(c *libpod.Container) bool {
			// if no pods match, quick out
			if len(pods) < 1 {
				return false
			}
			// if the container has no pod id, quick out
			if len(c.PodID()) < 1 {
				return false
			}
			for _, p := range pods {
				// we already looked up by name or id, so id match
				// here is ok
				if p.ID() == c.PodID() {
					return true
				}
			}
			return false
		}, nil
	case "network":
		var inputNetNames []string
		for _, val := range filterValues {
			net, err := r.Network().NetworkInspect(val)
			if err != nil {
				if errors.Is(err, define.ErrNoSuchNetwork) {
					continue
				}
				return nil, err
			}
			inputNetNames = append(inputNetNames, net.Name)
		}
		return func(c *libpod.Container) bool {
			networkMode := c.NetworkMode()
			// support docker like `--filter network=container:<IDorName>`
			// check if networkMode is configured as `container:<ctr>`
			// perform a match against filter `container:<IDorName>`
			// networks is already going to be empty if `container:<ctr>` is configured as Mode
			if networkModeContainerID, ok := strings.CutPrefix(networkMode, "container:"); ok {
				for _, val := range filterValues {
					if idOrName, ok := strings.CutPrefix(val, "container:"); ok {
						filterNetworkModeIDorName := idOrName
						filterID, err := r.LookupContainerID(filterNetworkModeIDorName)
						if err != nil {
							return false
						}
						if filterID == networkModeContainerID {
							return true
						}
					}
				}
				return false
			}

			networks, err := c.Networks()
			// if err or no networks, quick out
			if err != nil || len(networks) == 0 {
				return false
			}
			for _, net := range networks {
				if slices.Contains(inputNetNames, net) {
					return true
				}
			}
			return false
		}, nil
	case "restart-policy":
		invalidPolicyNames := []string{}
		for _, policy := range filterValues {
			if _, ok := define.RestartPolicyMap[policy]; !ok {
				invalidPolicyNames = append(invalidPolicyNames, policy)
			}
		}
		var filterValueError error
		if len(invalidPolicyNames) > 0 {
			errPrefix := "invalid restart policy"
			if len(invalidPolicyNames) > 1 {
				errPrefix = "invalid restart policies"
			}
			filterValueError = fmt.Errorf("%s %s", strings.Join(invalidPolicyNames, ", "), errPrefix)
		}
		return func(c *libpod.Container) bool {
			for _, policy := range filterValues {
				if policy == "none" && c.RestartPolicy() == define.RestartPolicyNone {
					return true
				}
				if c.RestartPolicy() == policy {
					return true
				}
			}
			return false
		}, filterValueError
	case "command":
		return func(c *libpod.Container) bool {
			return util.StringMatchRegexSlice(c.Command()[0], filterValues)
		}, nil
	case "should-start-on-boot":
		wantRestart := false
		var err error
		for _, fv := range filterValues {
			wantRestart, err = strconv.ParseBool(fv)
			if err != nil {
				return nil, err
			}
		}
		return func(c *libpod.Container) bool {
			return c.ShouldStartOnBoot() == wantRestart
		}, nil
	}
	return nil, fmt.Errorf("%s is an invalid filter", filter)
}

// GeneratePruneContainerFilterFuncs return ContainerFilter functions based of filter for prune operation
func GeneratePruneContainerFilterFuncs(filter string, filterValues []string, _ *libpod.Runtime) (func(container *libpod.Container) bool, error) {
	switch filter {
	case "label":
		return func(c *libpod.Container) bool {
			return filters.MatchLabelFilters(filterValues, c.Labels())
		}, nil
	case "label!":
		return func(c *libpod.Container) bool {
			return filters.MatchNegatedLabelFilters(filterValues, c.Labels())
		}, nil
	case "annotation":
		return func(c *libpod.Container) bool {
			return filters.MatchLabelFilters(filterValues, c.ConfigNoCopy().Spec.Annotations)
		}, nil
	case "annotation!":
		return func(c *libpod.Container) bool {
			return filters.MatchNegatedLabelFilters(filterValues, c.ConfigNoCopy().Spec.Annotations)
		}, nil
	case "until":
		return prepareUntilFilterFunc(filterValues)
	}
	return nil, fmt.Errorf("%s is an invalid filter", filter)
}

func prepareUntilFilterFunc(filterValues []string) (func(container *libpod.Container) bool, error) {
	until, err := filters.ComputeUntilTimestamp(filterValues)
	if err != nil {
		return nil, err
	}
	return func(c *libpod.Container) bool {
		if !until.IsZero() && c.CreatedTime().Before(until) {
			return true
		}
		return false
	}, nil
}

// GenerateExternalContainerFilterFuncs return ContainerFilter functions for external containers
func GenerateExternalContainerFilterFuncs(filter string, filterValues []string, r *libpod.Runtime) (func(listContainer *types.ListContainer) bool, error) {
	switch filter {
	case "id":
		return func(listContainer *types.ListContainer) bool {
			return filters.FilterID(listContainer.ID, filterValues)
		}, nil
	case "name":
		// we only have to match one name
		return func(listContainer *types.ListContainer) bool {
			namesList := listContainer.Names

			for _, f := range filterValues {
				f = strings.ReplaceAll(f, "/", "")
				if util.StringMatchRegexSlice(f, namesList) {
					return true
				}
			}

			return false
		}, nil
	case "command":
		return func(listContainer *types.ListContainer) bool {
			return util.StringMatchRegexSlice(listContainer.Command[0], filterValues)
		}, nil
	case "ancestor":
		// This needs to refine to match docker
		// - ancestor=(<image-name>[:tag]|<image-id>| ⟨image@digest⟩) - containers created from an image or a descendant.
		return func(listContainer *types.ListContainer) bool {
			for _, filterValue := range filterValues {
				var imageTag string
				var imageNameWithoutTag string
				// Compare with ImageID, ImageName
				// Will match ImageName if running image has tag latest for other tags exact complete filter must be given
				name, tag, hasColon := strings.Cut(listContainer.Image, ":")
				if hasColon {
					imageNameWithoutTag = name
					imageTag = tag
				}

				// Check for substring match on image ID (Docker compatibility)
				if strings.Contains(listContainer.ImageID, filterValue) {
					return true
				}

				// Check for regex match (advanced use cases)
				if util.StringMatchRegexSlice(listContainer.Image, filterValues) ||
					(util.StringMatchRegexSlice(imageNameWithoutTag, filterValues) && imageTag == "latest") {
					return true
				}
			}
			return false
		}, nil
	case "before":
		var createTime time.Time
		var externCons []storage.Container
		externCons, err := r.StorageContainers()
		if err != nil {
			return nil, err
		}

		for _, filterValue := range filterValues {
			for _, ctr := range externCons {
				if slices.Contains(ctr.Names, filterValue) {
					if createTime.IsZero() || createTime.After(ctr.Created) {
						createTime = ctr.Created
					}
				}
			}
		}

		return func(listContainer *types.ListContainer) bool {
			return createTime.After(listContainer.Created)
		}, nil
	case "since":
		var createTime time.Time
		var externCons []storage.Container
		externCons, err := r.StorageContainers()
		if err != nil {
			return nil, err
		}

		for _, filterValue := range filterValues {
			for _, ctr := range externCons {
				if slices.Contains(ctr.Names, filterValue) {
					if createTime.IsZero() || createTime.After(ctr.Created) {
						createTime = ctr.Created
					}
				}
			}
		}

		return func(listContainer *types.ListContainer) bool {
			return createTime.Before(listContainer.Created)
		}, nil
	case "until":
		until, err := filters.ComputeUntilTimestamp(filterValues)
		if err != nil {
			return nil, err
		}
		return func(listContainer *types.ListContainer) bool {
			if !until.IsZero() && listContainer.Created.Before(until) {
				return true
			}
			return false
		}, nil
	case "status":
		for _, filterValue := range filterValues {
			if _, err := define.StringToContainerStatus(filterValue); err != nil {
				return nil, err
			}
		}
		return func(listContainer *types.ListContainer) bool {
			state := listContainer.State
			if state == define.ContainerStateStopped.String() {
				state = "exited"
			}
			for _, filterValue := range filterValues {
				if filterValue == "stopped" {
					filterValue = "exited"
				}
				if state == filterValue {
					return true
				}
			}
			return false
		}, nil
	case "exited":
		var exitCodes []int32
		for _, exitCode := range filterValues {
			ec, err := strconv.ParseInt(exitCode, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("exited code out of range %q: %w", ec, err)
			}
			exitCodes = append(exitCodes, int32(ec))
		}
		return func(listContainer *types.ListContainer) bool {
			ec := listContainer.ExitCode
			exited := listContainer.Exited
			if exited {
				if slices.Contains(exitCodes, ec) {
					return true
				}
			}
			return false
		}, nil
	case "label":
		return func(listContainer *types.ListContainer) bool {
			return filters.MatchLabelFilters(filterValues, listContainer.Labels)
		}, nil
	case "label!":
		return func(listContainer *types.ListContainer) bool {
			return filters.MatchNegatedLabelFilters(filterValues, listContainer.Labels)
		}, nil
	case "pod":
		var pods []*libpod.Pod
		for _, podNameOrID := range filterValues {
			p, err := r.LookupPod(podNameOrID)
			if err != nil {
				if errors.Is(err, define.ErrNoSuchPod) {
					continue
				}
				return nil, err
			}
			pods = append(pods, p)
		}
		return func(listContainer *types.ListContainer) bool {
			// if no pods match, quick out
			if len(pods) < 1 {
				return false
			}
			// if the container has no pod id, quick out
			if len(listContainer.ID) < 1 {
				return false
			}
			for _, p := range pods {
				// we already looked up by name or id, so id match
				// here is ok
				if p.ID() == listContainer.ID {
					return true
				}
			}
			return false
		}, nil
	case "network":
		var inputNetNames []string
		for _, val := range filterValues {
			net, err := r.Network().NetworkInspect(val)
			if err != nil {
				if errors.Is(err, define.ErrNoSuchNetwork) {
					continue
				}
				return nil, err
			}
			inputNetNames = append(inputNetNames, net.Name)
		}
		return func(listContainer *types.ListContainer) bool {
			for _, net := range listContainer.Networks {
				if slices.Contains(inputNetNames, net) {
					return true
				}
			}
			return false
		}, nil
	case "restart-policy", "volume", "health", "annotation", "annotation!":
		return nil, fmt.Errorf("filter %s is not applicable for external containers", filter)
	}

	return nil, fmt.Errorf("%s is an invalid filter", filter)
}
