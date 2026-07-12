package etchosts

import (
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const defaultWSLRoute = "0.0.0.0/0"

// wslHostIP returns the Windows host's IP address. It only make
// sense to execute it when running in a WSL distribution.
// Instructions to retrieve the IP address are section "Identify IP address"
// (scenario 2) of the WSL networking documentation:
// https://learn.microsoft.com/en-us/windows/wsl/networking#identify-ip-address
func wslHostIP() string {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		logrus.Warnf("Failed getting routes in the WSL machine: %v", err)
		return ""
	}
	for _, r := range routes {
		if r.Dst.String() == defaultWSLRoute && r.Gw != nil {
			return r.Gw.String()
		}
	}
	logrus.Warnf("No default route found in the WSL machine")
	return ""
}
