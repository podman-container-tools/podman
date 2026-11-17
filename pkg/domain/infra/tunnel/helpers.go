package tunnel

import (
	"context"
	"errors"
	"fmt"

	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/pods"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

// FIXME: the `ignore` parameter is very likely wrong here as it should rather
//
//	be used on *errors* from operations such as remove.
func getContainersByContext(contextWithConnection context.Context, all, ignore bool, namesOrIDs []string) ([]entities.ListContainer, error) { //nolint:unparam
	ctrs, _, err := getContainersAndInputByContext(contextWithConnection, all, ignore, namesOrIDs, nil)
	return ctrs, err
}

func getContainersAndInputByContext(contextWithConnection context.Context, all, ignore bool, namesOrIDs []string, filters map[string][]string) ([]entities.ListContainer, []string, error) {
	if all && len(namesOrIDs) > 0 {
		return nil, nil, errors.New("cannot look up containers and all")
	}
	// short cut if not all, not filters and no names are given. This can happen with
	// --ignore and --cidfile, https://github.com/containers/podman/issues/23554.
	// In this case we have to do nothting and not even have to do a request
	if !all && len(filters) == 0 && len(namesOrIDs) == 0 {
		return nil, nil, nil
	}

	options := new(containers.ListOptions).WithAll(true).WithSync(true).WithFilters(filters)
	allContainers, err := containers.List(contextWithConnection, options)
	if err != nil {
		return nil, nil, err
	}

	rawInputs := []string{}

	// If no names or IDs are specified, we can return the result as is.
	// Otherwise, we need to do some further lookups.
	if len(namesOrIDs) == 0 {
		for i := range allContainers {
			rawInputs = append(rawInputs, allContainers[i].ID)
		}
		return allContainers, rawInputs, err
	}

	// Note: it would be nicer if the lists endpoint would support batch
	// name/ID lookups as we could use the libpod backend for looking up
	// containers rather than risking diverging the local and remote
	// lookups.
	//
	// A `--filter nameOrId=abc` that can be specified multiple times would
	// be awesome to have.
	filtered := []entities.ListContainer{}
	for _, nameOrID := range namesOrIDs {
		// First determine if the container exists by doing an inspect.
		// Inspect takes supports names and IDs and let's us determine
		// a container's full ID.
		inspectData, err := containers.Inspect(contextWithConnection, nameOrID, new(containers.InspectOptions).WithSize(false))
		if err != nil {
			if ignore && errorhandling.Contains(err, define.ErrNoSuchCtr) {
				continue
			}
			return nil, nil, err
		}

		// Now we can do a full match of the ID to find the right
		// container. Note that we *really* need a full ID match to
		// prevent any ambiguities between IDs and names (see #7837).
		found := false
		for _, ctr := range allContainers {
			if ctr.ID == inspectData.ID {
				filtered = append(filtered, ctr)
				rawInputs = append(rawInputs, nameOrID)
				found = true
				break
			}
		}

		if !found && !ignore {
			return nil, nil, fmt.Errorf("unable to find container %q: %w", nameOrID, define.ErrNoSuchCtr)
		}
	}
	return filtered, rawInputs, nil
}

func getPodsByContext(contextWithConnection context.Context, all bool, ignore bool, namesOrIDs []string) ([]*entities.ListPodsReport, error) {
	if all && len(namesOrIDs) > 0 {
		return nil, errors.New("cannot look up specific pods and all")
	}

	allPods, err := pods.List(contextWithConnection, nil)
	if err != nil {
		return nil, err
	}
	if all {
		return allPods, nil
	}

	filtered := []*entities.ListPodsReport{}
	// Note: it would be nicer if the lists endpoint would support that as
	// we could use the libpod backend for looking up pods rather than
	// risking diverging the local and remote lookups.
	//
	// A `--filter nameOrId=abc` that can be specified multiple times would
	// be awesome to have.
	for _, nameOrID := range namesOrIDs {
		// First determine if the pod exists by doing an inspect.
		// Inspect takes supports names and IDs and let's us determine
		// a container's full ID.
		inspectData, err := pods.Inspect(contextWithConnection, nameOrID, nil)
		if err != nil {
			if errorhandling.Contains(err, define.ErrNoSuchPod) {
				if ignore {
					continue
				}
				return nil, fmt.Errorf("unable to find pod %q: %w", nameOrID, define.ErrNoSuchPod)
			}
			return nil, err
		}

		// Now we can do a full match of the ID to find the right pod.
		// Note that we *really* need a full ID match to prevent any
		// ambiguities between IDs and names (see #7837).
		found := false
		for _, pod := range allPods {
			if pod.Id == inspectData.ID {
				filtered = append(filtered, pod)
				found = true
				break
			}
		}

		if !found {
			if ignore {
				continue
			}
			return nil, fmt.Errorf("unable to find pod %q: %w", nameOrID, define.ErrNoSuchPod)
		}
	}
	return filtered, nil
}
