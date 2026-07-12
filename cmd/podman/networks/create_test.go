package network

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/libnetwork/types"
)

func TestParseRoute(t *testing.T) {
	tests := []struct {
		name        string
		routeStr    string
		want        *types.Route
		wantErr     bool
		errContains string
	}{
		// Valid unicast routes (2 fields)
		{
			name:     "unicast route with destination and gateway",
			routeStr: "10.21.0.0/24,10.19.12.250",
			want: &types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				Gateway:     net.ParseIP("10.19.12.250"),
				RouteType:   types.RouteTypeUnicast,
			},
		},
		// Valid unicast routes (3 fields with metric)
		{
			name:     "unicast route with destination, gateway and metric",
			routeStr: "10.21.0.0/24,10.19.12.250,100",
			want: &types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				Gateway:     net.ParseIP("10.19.12.250"),
				Metric:      uint32Ptr(100),
				RouteType:   types.RouteTypeUnicast,
			},
		},
		// Valid blackhole routes (2 fields)
		{
			name:     "blackhole route with destination only",
			routeStr: "10.21.0.0/24,blackhole",
			want: &types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				RouteType:   types.RouteTypeBlackhole,
			},
		},
		// Valid blackhole routes (3 fields with metric)
		{
			name:     "blackhole route with destination and metric",
			routeStr: "10.21.0.0/24,blackhole,200",
			want: &types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				Metric:      uint32Ptr(200),
				RouteType:   types.RouteTypeBlackhole,
			},
		},
		// Valid unreachable routes
		{
			name:     "unreachable route with destination only",
			routeStr: "192.168.100.0/24,unreachable",
			want: &types.Route{
				Destination: mustParseCIDR(t, "192.168.100.0/24"),
				RouteType:   types.RouteTypeUnreachable,
			},
		},
		{
			name:     "unreachable route with destination and metric",
			routeStr: "192.168.100.0/24,unreachable,150",
			want: &types.Route{
				Destination: mustParseCIDR(t, "192.168.100.0/24"),
				Metric:      uint32Ptr(150),
				RouteType:   types.RouteTypeUnreachable,
			},
		},
		// Valid prohibit routes
		{
			name:     "prohibit route with destination only",
			routeStr: "172.16.0.0/16,prohibit",
			want: &types.Route{
				Destination: mustParseCIDR(t, "172.16.0.0/16"),
				RouteType:   types.RouteTypeProhibit,
			},
		},
		{
			name:     "prohibit route with destination and metric",
			routeStr: "172.16.0.0/16,prohibit,50",
			want: &types.Route{
				Destination: mustParseCIDR(t, "172.16.0.0/16"),
				Metric:      uint32Ptr(50),
				RouteType:   types.RouteTypeProhibit,
			},
		},
		// Case insensitivity for route types
		{
			name:     "blackhole route type uppercase",
			routeStr: "10.21.0.0/24,BLACKHOLE",
			want: &types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				RouteType:   types.RouteTypeBlackhole,
			},
		},
		{
			name:     "unreachable route type mixed case",
			routeStr: "10.21.0.0/24,Unreachable",
			want: &types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				RouteType:   types.RouteTypeUnreachable,
			},
		},
		{
			name:     "prohibit route type mixed case",
			routeStr: "10.21.0.0/24,ProHiBit",
			want: &types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				RouteType:   types.RouteTypeProhibit,
			},
		},
		// IPv6 routes
		{
			name:     "unicast IPv6 route",
			routeStr: "fd00:1::/64,fd00::1",
			want: &types.Route{
				Destination: mustParseCIDR(t, "fd00:1::/64"),
				Gateway:     net.ParseIP("fd00::1"),
				RouteType:   types.RouteTypeUnicast,
			},
		},
		{
			name:     "blackhole IPv6 route",
			routeStr: "fd00:2::/64,blackhole",
			want: &types.Route{
				Destination: mustParseCIDR(t, "fd00:2::/64"),
				RouteType:   types.RouteTypeBlackhole,
			},
		},
		// Invalid routes - too few fields
		{
			name:        "route with only destination",
			routeStr:    "10.21.0.0/24",
			wantErr:     true,
			errContains: "invalid route",
		},
		// Invalid routes - too many fields
		{
			name:        "route with too many fields",
			routeStr:    "10.21.0.0/24,10.19.12.250,100,extra",
			wantErr:     true,
			errContains: "invalid route",
		},
		// Invalid routes - unicast without gateway
		{
			name:        "unicast route type without gateway",
			routeStr:    "10.21.0.0/24,unicast",
			wantErr:     true,
			errContains: "unicast route requires a gateway",
		},
		// Invalid routes - bad destination
		{
			name:        "invalid destination CIDR",
			routeStr:    "invalid-cidr,10.19.12.250",
			wantErr:     true,
			errContains: "invalid route destination",
		},
		// Invalid routes - bad gateway
		{
			name:        "invalid gateway IP",
			routeStr:    "10.21.0.0/24,invalid-gateway",
			wantErr:     true,
			errContains: "invalid route gateway",
		},
		// Invalid routes - bad metric
		{
			name:        "invalid metric for unicast route",
			routeStr:    "10.21.0.0/24,10.19.12.250,invalid-metric",
			wantErr:     true,
			errContains: "invalid route metric",
		},
		{
			name:        "invalid metric for blackhole route",
			routeStr:    "10.21.0.0/24,blackhole,invalid-metric",
			wantErr:     true,
			errContains: "invalid route metric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRoute(tt.routeStr)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.Destination.String(), got.Destination.String())
			assert.Equal(t, tt.want.Gateway, got.Gateway)
			assert.Equal(t, tt.want.RouteType, got.RouteType)
			if tt.want.Metric != nil {
				require.NotNil(t, got.Metric)
				assert.Equal(t, *tt.want.Metric, *got.Metric)
			} else {
				assert.Nil(t, got.Metric)
			}
		})
	}
}

func mustParseCIDR(t *testing.T, cidr string) types.IPNet {
	t.Helper()
	ipnet, err := types.ParseCIDR(cidr)
	require.NoError(t, err)
	return ipnet
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}
