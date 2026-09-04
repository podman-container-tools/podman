//go:build !remote && (linux || freebsd)

package compat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCgroupDriver(t *testing.T) {
	tests := []struct {
		name           string
		cgroupManager  string
		isRootless     bool
		expectedDriver string
	}{
		{
			name:           "rootful cgroupfs is reported as-is",
			cgroupManager:  "cgroupfs",
			isRootless:     false,
			expectedDriver: "cgroupfs",
		},
		{
			// Rootless cannot create a cgroup parent outside of the
			// delegated subtree, so clients must not be told to pick one.
			name:           "rootless cgroupfs is reported as none",
			cgroupManager:  "cgroupfs",
			isRootless:     true,
			expectedDriver: "none",
		},
		{
			name:           "rootful systemd is reported as-is",
			cgroupManager:  "systemd",
			isRootless:     false,
			expectedDriver: "systemd",
		},
		{
			name:           "rootless systemd is reported as-is",
			cgroupManager:  "systemd",
			isRootless:     true,
			expectedDriver: "systemd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedDriver, getCgroupDriver(tt.cgroupManager, tt.isRootless))
		})
	}
}
