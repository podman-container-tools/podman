//go:build !remote

package generate

import "go.podman.io/podman/v6/pkg/specgen"

// verifyContainerResources does nothing on freebsd as it has no cgroups
func verifyContainerResources(_ *specgen.SpecGenerator) ([]string, error) {
	return nil, nil
}
