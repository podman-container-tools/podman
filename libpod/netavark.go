//go:build !remote && (linux || freebsd)

package libpod

import (
	"os/exec"

	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
)

// netavarkVersionInfo is the subset of "netavark version" output that
// podman consumes. default_fw_driver is the firewall backend the binary
// was built with; it is absent in older netavark releases.
type netavarkVersionInfo struct {
	DefaultFwDriver string `json:"default_fw_driver"`
}

// netavarkFirewallDriver returns the effective firewall driver for the
// netavark backend: the configured network.firewall_driver when set,
// otherwise the default compiled into the netavark binary. It returns an
// empty string for non-netavark backends or when the driver cannot be
// determined, rather than guessing a value.
func netavarkFirewallDriver(conf *config.Config, netInfo types.NetworkInfo) string {
	if netInfo.Backend != types.Netavark {
		return ""
	}
	if conf.Network.FirewallDriver != "" {
		return conf.Network.FirewallDriver
	}
	return netavarkDefaultFirewallDriver(netInfo.Path)
}

// netavarkDefaultFirewallDriver asks the netavark binary which firewall
// driver it was built with via "netavark version". This runs once per
// Info() call; caching it is left to a follow-up that surfaces
// default_fw_driver through container-libs/common NetworkInfo, which is
// already populated once per info request.
func netavarkDefaultFirewallDriver(binary string) string {
	if binary == "" {
		return ""
	}
	out, err := exec.Command(binary, "version").Output()
	if err != nil {
		return ""
	}
	var info netavarkVersionInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return ""
	}
	return info.DefaultFwDriver
}
