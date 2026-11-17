package config

import (
	"go.podman.io/common/pkg/capabilities"
)

var defaultHelperBinariesDir = []string{
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
	"/usr/libexec/podman",
	"/usr/lib/podman",
}

// Capabilities returns the capabilities parses the Add and Drop capability
// list from the default capabilities for the container.
func (c *Config) Capabilities(user string, addCapabilities, dropCapabilities []string) ([]string, error) {
	userNotRoot := func(user string) bool {
		if user == "" || user == "root" || user == "0" {
			return false
		}
		return true
	}

	defaultCapabilities := c.Containers.DefaultCapabilities.Get()
	if userNotRoot(user) {
		defaultCapabilities = []string{}
	}

	return capabilities.MergeCapabilities(defaultCapabilities, addCapabilities, dropCapabilities)
}
