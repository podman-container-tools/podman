//go:build linux && !remote

package system

import (
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/cgroups"
	"go.podman.io/common/pkg/servicereaper"
)

// Currently, we only need servicereaper on Linux for rootless networking.
func maybeStartServiceReaper() {
	servicereaper.Start()
}

func maybeMoveToSubCgroup() {
	if err := cgroups.MaybeMoveToSubCgroup(); err != nil {
		// it is a best effort operation, so just print the
		// error for debugging purposes.
		logrus.Debugf("Could not move to subcgroup: %v", err)
	}
}
