//go:build !remote

package generate

import (
	"github.com/opencontainers/runtime-tools/generate"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/specgen"
)

// setLabelOpts sets the label options of the SecurityConfig according to the
// input.
func setLabelOpts(_ *specgen.SpecGenerator, _ *libpod.Runtime, _ specgen.Namespace, _ specgen.Namespace) error {
	return nil
}

func securityConfigureGenerator(s *specgen.SpecGenerator, g *generate.Generator, _ *libimage.Image, _ *config.Config) error {
	// If this is a privileged container, change the devfs ruleset to expose all devices.
	if s.IsPrivileged() {
		for k, m := range g.Config.Mounts {
			if m.Type == "devfs" {
				m.Options = []string{
					"ruleset=0",
				}
				g.Config.Mounts[k] = m
			}
		}
	}

	if s.ReadOnlyFilesystem != nil {
		g.SetRootReadonly(*s.ReadOnlyFilesystem)
	}

	return nil
}
