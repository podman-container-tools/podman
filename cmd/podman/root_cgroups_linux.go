//go:build linux && !remote

package main

import (
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/cgroups"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

func checkSupportedCgroups() {
	if registry.IsRemote() {
		// In remote mode we should not error for missing cgroups as just the server needs it.
		return
	}
	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		logrus.Fatalf("Error determining cgroups mode")
	}
	if !unified {
		logrus.Fatalf("Cgroups v1 not supported")
	}
}
