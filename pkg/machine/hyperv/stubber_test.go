//go:build windows

package hyperv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/pkg/machine/hyperv/vsock"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func TestCanCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		isElevated        bool
		vsockEntriesExist bool
		isHyperVAdmin     error
		mounts            int
		expectedErr       error
	}{
		{
			name:              "in elevated process can always create",
			isElevated:        true,
			vsockEntriesExist: false,
			isHyperVAdmin:     ErrHypervUserNotInAdminGroup,
			mounts:            2,
			expectedErr:       nil,
		},
		{
			name:              "not elevated, no vsock entries",
			isElevated:        false,
			vsockEntriesExist: false,
			isHyperVAdmin:     nil,
			expectedErr:       ErrHypervRegistryInitRequiresElevation,
		},
		{
			name:              "not elevated, vsock entries exist, not hyperv admin",
			isElevated:        false,
			vsockEntriesExist: true,
			isHyperVAdmin:     ErrHypervUserNotInAdminGroup,
			expectedErr:       ErrHypervUserNotInAdminGroup,
		},
		{
			name:              "not elevated, vsock entries exist, hyperv admin",
			isElevated:        false,
			vsockEntriesExist: true,
			isHyperVAdmin:     nil,
			expectedErr:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checks := permissionChecks{
				isElevatedProcess:   func() bool { return tt.isElevated },
				isHyperVAdminMember: func() error { return tt.isHyperVAdmin },
				vsockEntriesExist:   func(int) bool { return tt.vsockEntriesExist },
			}

			err := checkCanCreate(checks, tt.mounts)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestCanRemove(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		isElevatedProcess       bool
		isHyperVAdminMember     error
		isLegacyMachine         bool
		skipVsockEntriesRemoval bool
		isLastMachine           bool
		expectedErr             error
	}{
		{
			name:                    "elevated process can always remove",
			isElevatedProcess:       true,
			isHyperVAdminMember:     ErrHypervUserNotInAdminGroup,
			isLegacyMachine:         false,
			skipVsockEntriesRemoval: false,
			isLastMachine:           true,
			expectedErr:             nil,
		},
		{
			name:                    "not elevated, legacy machine requires elevation",
			isElevatedProcess:       false,
			isHyperVAdminMember:     ErrHypervUserNotInAdminGroup,
			isLegacyMachine:         true,
			skipVsockEntriesRemoval: false,
			isLastMachine:           true,
			expectedErr:             ErrHypervLegacyMachineRequiresElevation,
		},
		{
			name:                    "not elevated, not hyperv admins member",
			isElevatedProcess:       false,
			isHyperVAdminMember:     ErrHypervUserNotInAdminGroup,
			isLegacyMachine:         false,
			skipVsockEntriesRemoval: false,
			isLastMachine:           true,
			expectedErr:             ErrHypervUserNotInAdminGroup,
		},
		{
			name:                    "not elevated, hyperv admins member, not last machine",
			isElevatedProcess:       false,
			isHyperVAdminMember:     nil,
			isLegacyMachine:         false,
			skipVsockEntriesRemoval: false,
			isLastMachine:           false,
			expectedErr:             nil,
		},
		{
			name:                    "not elevated, hyperv admins member, last machine, skip vsock entries removal",
			isElevatedProcess:       false,
			isHyperVAdminMember:     nil,
			isLegacyMachine:         false,
			skipVsockEntriesRemoval: true,
			isLastMachine:           true,
			expectedErr:             nil,
		},
		{
			name:                    "not elevated, hyperv admins member, last machine, don't skip vsock entries removal",
			isElevatedProcess:       false,
			isHyperVAdminMember:     nil,
			isLegacyMachine:         false,
			skipVsockEntriesRemoval: false,
			isLastMachine:           true,
			expectedErr:             ErrHypervRegistryRemoveRequiresElevation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checks := permissionChecks{
				isElevatedProcess:   func() bool { return tt.isElevatedProcess },
				isHyperVAdminMember: func() error { return tt.isHyperVAdminMember },
				existingMachinesNum: func() (int, error) {
					if tt.isLastMachine {
						return 1, nil
					} else {
						return 2, nil
					}
				},
			}

			vsockMachineName := ""
			if tt.isLegacyMachine {
				vsockMachineName = "podmanmachine"
			}

			mc := &vmconfigs.MachineConfig{
				HyperVHypervisor: &vmconfigs.HyperVConfig{
					ReadyVsock: vsock.HVSockRegistryEntry{
						MachineName:            vsockMachineName,
						KeepAfterMachineRemove: tt.skipVsockEntriesRemoval,
					},
					NetworkVSock: vsock.HVSockRegistryEntry{
						KeepAfterMachineRemove: tt.skipVsockEntriesRemoval,
					},
				},
			}

			err := checkCanRemove(mc, checks)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
