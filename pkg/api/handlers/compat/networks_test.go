//go:build !remote && (linux || freebsd)

package compat

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nettypes "go.podman.io/common/libnetwork/types"
	netutil "go.podman.io/common/libnetwork/util"
)

// leaseRangeFor builds a LeaseRange the way CreateNetwork does when it receives a
// Docker IPRange, so the tests exercise the exact shape the mapping has to undo.
func leaseRangeFor(t *testing.T, cidr string) *nettypes.LeaseRange {
	t.Helper()

	_, ipNet, err := net.ParseCIDR(cidr)
	require.NoError(t, err)

	first, err := netutil.FirstIPInSubnet(ipNet)
	require.NoError(t, err)
	last, err := netutil.LastIPInSubnet(ipNet)
	require.NoError(t, err)

	return &nettypes.LeaseRange{StartIP: first, EndIP: last}
}

func TestLeaseRangeToDockerIPRange(t *testing.T) {
	t.Run("round-trips a CIDR the Docker API could have supplied", func(t *testing.T) {
		for _, cidr := range []string{
			"10.0.0.0/24",
			"10.10.8.0/22",
			"192.168.1.128/25",
			"172.16.0.0/16",
			"10.0.0.0/8",
			"fd00::/64",
			"2001:db8:1::/48",
			"2001:db8::/32",
		} {
			got := leaseRangeToDockerIPRange(leaseRangeFor(t, cidr))
			assert.Equal(t, cidr, got.String(), "lease range for %s must map back to it", cidr)
		}
	})

	t.Run("nil lease range yields no IPRange", func(t *testing.T) {
		// Networks created without an IPRange have no LeaseRange, and IPAMConfig.IPRange
		// is omitzero, so the field is left out of the response entirely.
		assert.False(t, leaseRangeToDockerIPRange(nil).IsValid())
	})

	t.Run("partially populated lease range yields no IPRange", func(t *testing.T) {
		cases := map[string]*nettypes.LeaseRange{
			"both unset":  {},
			"no end IP":   {StartIP: net.ParseIP("10.0.0.1")},
			"no start IP": {EndIP: net.ParseIP("10.0.0.255")},
		}
		for name, lr := range cases {
			assert.False(t, leaseRangeToDockerIPRange(lr).IsValid(), "%s", name)
		}
	})

	t.Run("range that is not a whole CIDR yields no IPRange", func(t *testing.T) {
		// Returning a wrong-but-plausible prefix here would be worse than returning
		// nothing, since the caller cannot tell it is wrong.
		cases := map[string]*nettypes.LeaseRange{
			"arbitrary sub-range": {
				StartIP: net.ParseIP("10.0.0.5"),
				EndIP:   net.ParseIP("10.0.0.99"),
			},
			"start not aligned to the mask": {
				StartIP: net.ParseIP("10.0.0.2"),
				EndIP:   net.ParseIP("10.0.0.255"),
			},
			"end is not the broadcast address": {
				StartIP: net.ParseIP("10.0.0.1"),
				EndIP:   net.ParseIP("10.0.0.254"),
			},
			"reversed": {
				StartIP: net.ParseIP("10.0.0.200"),
				EndIP:   net.ParseIP("10.0.0.1"),
			},
		}
		for name, lr := range cases {
			assert.False(t, leaseRangeToDockerIPRange(lr).IsValid(), "%s", name)
		}
	})

	t.Run("single address maps to a host route", func(t *testing.T) {
		v4 := &nettypes.LeaseRange{
			StartIP: net.ParseIP("10.0.0.7"),
			EndIP:   net.ParseIP("10.0.0.7"),
		}
		assert.Equal(t, "10.0.0.7/32", leaseRangeToDockerIPRange(v4).String())

		v6 := &nettypes.LeaseRange{
			StartIP: net.ParseIP("fd00::7"),
			EndIP:   net.ParseIP("fd00::7"),
		}
		assert.Equal(t, "fd00::7/128", leaseRangeToDockerIPRange(v6).String())
	})

	t.Run("mixed address families yield no IPRange", func(t *testing.T) {
		lr := &nettypes.LeaseRange{
			StartIP: net.ParseIP("10.0.0.1"),
			EndIP:   net.ParseIP("fd00::ffff"),
		}
		assert.False(t, leaseRangeToDockerIPRange(lr).IsValid())
	})

	t.Run("4-in-6 encoded addresses are treated as IPv4", func(t *testing.T) {
		// net.ParseIP returns 16-byte 4-in-6 for dotted quads; the mapping must not
		// mistake that for an IPv6 range.
		lr := &nettypes.LeaseRange{
			StartIP: net.ParseIP("10.0.0.1").To16(),
			EndIP:   net.ParseIP("10.0.0.255").To16(),
		}
		assert.Equal(t, "10.0.0.0/24", leaseRangeToDockerIPRange(lr).String())
	})

	t.Run("all-zero start yields no IPRange", func(t *testing.T) {
		// There is no network address below 0.0.0.0, so this cannot be the usable
		// span of any CIDR.
		lr := &nettypes.LeaseRange{
			StartIP: net.IPv4zero,
			EndIP:   net.ParseIP("255.255.255.255"),
		}
		assert.False(t, leaseRangeToDockerIPRange(lr).IsValid())
	})
}

func TestHostBitsBetween(t *testing.T) {
	t.Run("counts a contiguous run of trailing ones", func(t *testing.T) {
		cases := []struct {
			network string
			end     string
			want    int
		}{
			{"10.0.0.0", "10.0.0.0", 0},
			{"10.0.0.0", "10.0.0.255", 8},
			{"10.0.0.0", "10.0.3.255", 10},
			{"10.0.0.0", "10.0.255.255", 16},
			{"0.0.0.0", "255.255.255.255", 32},
			{"fd00::", "fd00::ffff", 16},
			{"2001:db8::", "2001:db8::ffff:ffff:ffff:ffff", 64},
			{"::", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", 128},
		}
		for _, tc := range cases {
			got, ok := hostBitsBetween(netip.MustParseAddr(tc.network), netip.MustParseAddr(tc.end))
			require.True(t, ok, "%s^%s must be a valid host mask", tc.network, tc.end)
			assert.Equal(t, tc.want, got, "%s^%s", tc.network, tc.end)
		}
	})

	t.Run("rejects a mask whose ones are not contiguous", func(t *testing.T) {
		// A host mask must be 0...01...1; anything else means the range does not
		// line up with a CIDR boundary.
		cases := [][2]string{
			{"10.0.0.0", "10.0.1.0"},      // 0x00000100
			{"10.0.0.0", "10.0.254.255"},  // 0x0000feff
			{"10.0.0.0", "11.0.0.255"},    // 0x010000ff
			{"10.0.0.0", "245.0.255.255"}, // 0xff00ffff
			{"fd00::", "fd00::1:0"},       // gap above the trailing ones
			{"fd00::", "fd01::ffff"},      // differing byte above the mask
		}
		for _, tc := range cases {
			_, ok := hostBitsBetween(netip.MustParseAddr(tc[0]), netip.MustParseAddr(tc[1]))
			assert.False(t, ok, "%s^%s must be rejected", tc[0], tc[1])
		}
	})
}

// Guard against the prefix being computed from the wrong end of the range.
func TestLeaseRangeToDockerIPRangeUsesNetworkAddress(t *testing.T) {
	lr := leaseRangeFor(t, "10.10.8.0/22")

	got := leaseRangeToDockerIPRange(lr)
	require.True(t, got.IsValid())

	assert.Equal(t, netip.MustParseAddr("10.10.8.0"), got.Addr(),
		"prefix must start at the network address, not at StartIP")
	assert.Equal(t, 22, got.Bits())
}
