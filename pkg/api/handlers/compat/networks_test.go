//go:build !remote && (linux || freebsd)

package compat

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrefixFromRange(t *testing.T) {
	tests := []struct {
		name  string
		start string
		end   string
		want  string // empty means prefixFromRange should report ok == false
	}{
		{"ipv4 /24", "10.89.0.1", "10.89.0.255", "10.89.0.0/24"},
		{"ipv4 /28", "192.168.1.17", "192.168.1.31", "192.168.1.16/28"},
		{"ipv4 /8", "10.0.0.1", "10.255.255.255", "10.0.0.0/8"},
		{"ipv6 /64", "fd00::1", "fd00::ffff:ffff:ffff:ffff", "fd00::/64"},
		{"non-aligned range", "10.0.0.5", "10.0.0.20", ""},
		{"reversed range", "10.0.0.255", "10.0.0.1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := prefixFromRange(net.ParseIP(tt.start), net.ParseIP(tt.end))
			if tt.want == "" {
				require.False(t, ok, "expected no CIDR for range %s-%s, got %v", tt.start, tt.end, got)
				return
			}
			require.True(t, ok, "expected a CIDR for range %s-%s", tt.start, tt.end)
			require.Equal(t, tt.want, got.String())
		})
	}
}
