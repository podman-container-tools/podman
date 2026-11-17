//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func Test_sortMounts(t *testing.T) {
	tests := []struct {
		name string
		args []spec.Mount
		want []spec.Mount
	}{
		{
			name: "simple nested mounts",
			args: []spec.Mount{
				{
					Destination: "/abc/123",
				},
				{
					Destination: "/abc",
				},
			},
			want: []spec.Mount{
				{
					Destination: "/abc",
				},
				{
					Destination: "/abc/123",
				},
			},
		},
		{
			name: "root mount",
			args: []spec.Mount{
				{
					Destination: "/abc",
				},
				{
					Destination: "/",
				},
				{
					Destination: "/def",
				},
			},
			want: []spec.Mount{
				{
					Destination: "/",
				},
				{
					Destination: "/abc",
				},
				{
					Destination: "/def",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortMounts(tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}

type mockVendorLister struct {
	vendors []string
}

func (m *mockVendorLister) ListVendors() []string {
	return m.vendors
}

func Test_gpusToCDIDevices(t *testing.T) {
	tests := []struct {
		name          string
		gpus          []string
		vendors       []string
		expectError   bool
		expectDevices []string
	}{
		{
			name:    "No GPUs",
			gpus:    []string{},
			vendors: []string{"amd.com"},
		},
		{
			name:    "Nil GPUs",
			gpus:    nil,
			vendors: []string{"amd.com"},
		},
		{
			name:        "Nil vendors",
			gpus:        []string{"0"},
			vendors:     nil,
			expectError: true,
		},
		{
			name:          "Single GPU with AMD",
			gpus:          []string{"0"},
			vendors:       []string{"amd.com"},
			expectDevices: []string{"amd.com/gpu=0"},
		},
		{
			name:          "Multiple GPUs with AMD",
			gpus:          []string{"0", "1"},
			vendors:       []string{"amd.com"},
			expectDevices: []string{"amd.com/gpu=0", "amd.com/gpu=1"},
		},
		{
			name:          "Single GPU with NVIDIA",
			gpus:          []string{"0"},
			vendors:       []string{"nvidia.com"},
			expectDevices: []string{"nvidia.com/gpu=0"},
		},
		{
			name:          "Multiple GPUs with NVIDIA",
			gpus:          []string{"0", "1"},
			vendors:       []string{"nvidia.com"},
			expectDevices: []string{"nvidia.com/gpu=0", "nvidia.com/gpu=1"},
		},
		{
			name:        "No vendors",
			gpus:        []string{"0"},
			vendors:     []string{},
			expectError: true,
		},
		{
			name:        "Unknown vendor",
			gpus:        []string{"0"},
			vendors:     []string{"unknown.com"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lister vendorLister
			if tt.vendors != nil {
				lister = &mockVendorLister{vendors: tt.vendors}
			}
			cdiDevices, err := gpusToCDIDevices(tt.gpus, lister)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectDevices, cdiDevices)
			}
		})
	}
}

func Test_discoverGPUVendorFromCDI(t *testing.T) {
	tests := []struct {
		name         string
		vendors      []string
		expectVendor string
		expectError  bool
	}{
		{
			name:        "Nil vendors",
			vendors:     nil,
			expectError: true,
		},
		{
			name:         "NVIDIA vendor",
			vendors:      []string{"nvidia.com"},
			expectVendor: "nvidia.com",
		},
		{
			name:         "AMD vendor",
			vendors:      []string{"amd.com"},
			expectVendor: "amd.com",
		},
		{
			name:        "No vendors",
			vendors:     []string{},
			expectError: true,
		},
		{
			name:        "Unknown vendor",
			vendors:     []string{"unknown.com"},
			expectError: true,
		},
		{
			name:         "Mixed vendor",
			vendors:      []string{"amd.com", "nvidia.com"},
			expectVendor: "nvidia.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lister vendorLister
			if tt.vendors != nil {
				lister = &mockVendorLister{vendors: tt.vendors}
			}
			vendor, err := discoverGPUVendorFromCDI(lister)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectVendor, vendor)
			}
		})
	}
}
