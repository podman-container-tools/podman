//go:build !remote && (linux || freebsd)

package libpod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
)

// fakeNetavark writes an executable shell script that prints the given
// output for "netavark version" and returns its path.
func fakeNetavark(t *testing.T, output string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "netavark")
	script := "#!/bin/sh\nprintf '%s' '" + output + "'\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func TestNetavarkDefaultFirewallDriver(t *testing.T) {
	tests := []struct {
		name     string
		binary   string
		expected string
	}{
		{
			name: "nftables default",
			binary: fakeNetavark(t, `{
  "version": "2.1.0",
  "default_fw_driver": "nftables"
}`),
			expected: "nftables",
		},
		{
			name: "firewalld default",
			binary: fakeNetavark(t, `{
  "version": "2.1.0",
  "default_fw_driver": "firewalld"
}`),
			expected: "firewalld",
		},
		{
			name: "none default",
			binary: fakeNetavark(t, `{
  "version": "2.1.0",
  "default_fw_driver": "none"
}`),
			expected: "none",
		},
		{
			name: "older netavark without default_fw_driver",
			binary: fakeNetavark(t, `{
  "version": "1.0.0",
  "commit": "abc"
}`),
			expected: "",
		},
		{
			name:     "invalid json",
			binary:   fakeNetavark(t, `not json`),
			expected: "",
		},
		{
			name:     "empty default_fw_driver",
			binary:   fakeNetavark(t, `{"version": "2.1.0", "default_fw_driver": ""}`),
			expected: "",
		},
		{
			name:     "missing binary",
			binary:   filepath.Join(t.TempDir(), "does-not-exist"),
			expected: "",
		},
		{
			name:     "empty binary path",
			binary:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, netavarkDefaultFirewallDriver(tt.binary))
		})
	}
}

func TestNetavarkFirewallDriver(t *testing.T) {
	netavarkBackend := types.NetworkInfo{Backend: types.Netavark, Path: fakeNetavark(t, `{"default_fw_driver": "nftables"}`)}
	conf := &config.Config{}

	t.Run("configured driver reported verbatim", func(t *testing.T) {
		c := &config.Config{}
		c.Network.FirewallDriver = "firewalld"
		assert.Equal(t, "firewalld", netavarkFirewallDriver(c, netavarkBackend))
	})

	t.Run("unset driver uses netavark default", func(t *testing.T) {
		assert.Equal(t, "nftables", netavarkFirewallDriver(conf, netavarkBackend))
	})

	t.Run("non-netavark backend reports nothing", func(t *testing.T) {
		cni := types.NetworkInfo{Backend: types.NetworkBackend("cni")}
		assert.Equal(t, "", netavarkFirewallDriver(conf, cni))
	})

	t.Run("undeterminable default reports nothing", func(t *testing.T) {
		noPath := types.NetworkInfo{Backend: types.Netavark, Path: ""}
		assert.Equal(t, "", netavarkFirewallDriver(conf, noPath))
	})
}
