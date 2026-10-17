//go:build windows

package hyperv

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func TestClient9pArgs(t *testing.T) {
	vsockPort := uint64(50000)

	tests := []struct {
		name        string
		mount       *vmconfigs.Mount
		wantArgs    []string
		wantErr     bool
		expectedErr string
	}{
		{
			name: "read-write mount generates client9p args without read-only",
			mount: &vmconfigs.Mount{
				ReadOnly:    false,
				Target:      "/mnt/data",
				VSockNumber: &vsockPort,
			},
			wantArgs: []string{"machine", "client9p", "50000", strconv.Quote("/mnt/data")},
			wantErr:  false,
		},
		{
			name: "read-only mount generates client9p args without read-only flag for guest compatibility",
			mount: &vmconfigs.Mount{
				ReadOnly:    true,
				Target:      "/mnt/readonly",
				VSockNumber: &vsockPort,
			},
			wantArgs: []string{"machine", "client9p", "50000", strconv.Quote("/mnt/readonly")},
			wantErr:  false,
		},
		{
			name: "undefined vsock number returns error",
			mount: &vmconfigs.Mount{
				ReadOnly:    false,
				Target:      "/mnt/data",
				VSockNumber: nil,
			},
			wantErr:     true,
			expectedErr: "cannot start 9p shares with undefined vsock number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client9pArgs(tt.mount)
			if tt.wantErr {
				assert.EqualError(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantArgs, got)
			}
		})
	}
}

func TestStartShareMountArgs(t *testing.T) {
	vsockPort := uint64(50000)

	tests := []struct {
		name        string
		mount       *vmconfigs.Mount
		debug       bool
		wantArgs    []string
		wantErr     bool
		expectedErr string
	}{
		{
			name: "read-write mount without debug",
			mount: &vmconfigs.Mount{
				ReadOnly:    false,
				Target:      "/mnt/data",
				VSockNumber: &vsockPort,
			},
			debug: false,
			wantArgs: []string{
				"sudo", "mkdir", "-p", strconv.Quote("/mnt/data"), "; ",
				"sudo", "podman", "machine", "client9p", "50000", strconv.Quote("/mnt/data"),
			},
			wantErr: false,
		},
		{
			name: "read-only mount sets CONTAINERS_MACHINE_9P_READ_ONLY env",
			mount: &vmconfigs.Mount{
				ReadOnly:    true,
				Target:      "/mnt/readonly",
				VSockNumber: &vsockPort,
			},
			debug: false,
			wantArgs: []string{
				"sudo", "mkdir", "-p", strconv.Quote("/mnt/readonly"), "; ",
				"sudo", "CONTAINERS_MACHINE_9P_READ_ONLY=1", "podman", "machine", "client9p", "50000", strconv.Quote("/mnt/readonly"),
			},
			wantErr: false,
		},
		{
			name: "read-only mount with debug flag",
			mount: &vmconfigs.Mount{
				ReadOnly:    true,
				Target:      "/mnt/readonly",
				VSockNumber: &vsockPort,
			},
			debug: true,
			wantArgs: []string{
				"sudo", "mkdir", "-p", strconv.Quote("/mnt/readonly"), "; ",
				"sudo", "CONTAINERS_MACHINE_9P_READ_ONLY=1", "podman", "--log-level=debug", "machine", "client9p", "50000", strconv.Quote("/mnt/readonly"),
			},
			wantErr: false,
		},
		{
			name: "read-only mount outside home and mnt includes chattr",
			mount: &vmconfigs.Mount{
				ReadOnly:    true,
				Target:      "/var/shared",
				VSockNumber: &vsockPort,
			},
			debug: false,
			wantArgs: []string{
				"sudo", "chattr", "-i", "/", "; ",
				"sudo", "mkdir", "-p", strconv.Quote("/var/shared"), "; ",
				"sudo", "chattr", "+i", "/", "; ",
				"sudo", "CONTAINERS_MACHINE_9P_READ_ONLY=1", "podman", "machine", "client9p", "50000", strconv.Quote("/var/shared"),
			},
			wantErr: false,
		},
		{
			name: "undefined vsock number returns error",
			mount: &vmconfigs.Mount{
				ReadOnly:    false,
				Target:      "/mnt/data",
				VSockNumber: nil,
			},
			debug:       false,
			wantErr:     true,
			expectedErr: "cannot start 9p shares with undefined vsock number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := startShareMountArgs(tt.mount, tt.debug)
			if tt.wantErr {
				assert.EqualError(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantArgs, got)
			}
		})
	}
}
