//go:build windows

package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/machine/define"
)

func TestGetByVMType_Supported(t *testing.T) {
	testCases := []struct {
		name   string
		vmType define.VMType
	}{
		{"HyperV", define.HyperVVirt},
		{"WSL", define.WSLVirt},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// GetByVMType for supported types should always return a provider.
			// Permission checks are handled at the stubber method level.
			provider, err := GetByVMType(tc.vmType)

			require.NoError(t, err)
			assert.NotNil(t, provider)
			assert.Equal(t, tc.vmType, provider.VMType())
		})
	}
}

func TestGetByVMType_UnsupportedProvider(t *testing.T) {
	provider, err := GetByVMType(define.QemuVirt)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported virtualization provider")
	assert.Nil(t, provider)
}
