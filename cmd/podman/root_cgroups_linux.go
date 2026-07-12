//go:build linux

package main

import (
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/cgroups"
)

func checkSupportedCgroups() {
	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		logrus.Fatalf("Error determining cgroups mode")
	}
	if !unified {
		logrus.Fatalf("Cgroups v1 not supported")
	}
}
