//go:build !remote

package libpod

import (
	"net"
	"reflect"
	"testing"

	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/libpod/define"
)

func Test_resultToBasicNetworkConfig(t *testing.T) {
	testCases := []struct {
		description           string
		inputResult           types.StatusBlock
		expectedNetworkConfig define.InspectBasicNetworkConfig
	}{
		{
			description: "single secondary IPv4 address is shown as define.Address",
			inputResult: types.StatusBlock{
				Interfaces: map[string]types.NetInterface{
					"eth1": {
						Subnets: []types.NetAddress{
							{
								Gateway: net.ParseIP("172.26.0.1"),
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.2"),
										Mask: net.CIDRMask(20, 32),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.3"),
										Mask: net.CIDRMask(10, 32),
									},
								},
							},
						},
					},
				},
			},
			expectedNetworkConfig: define.InspectBasicNetworkConfig{
				IPAddress:   "172.26.0.2",
				IPPrefixLen: 20,
				Gateway:     "172.26.0.1",
				SecondaryIPAddresses: []define.Address{
					{
						Addr:         "172.26.0.3",
						PrefixLength: 10,
					},
				},
			},
		},
		{
			description: "multiple secondary IPv4 addresses are shown as define.Address",
			inputResult: types.StatusBlock{
				Interfaces: map[string]types.NetInterface{
					"eth1": {
						Subnets: []types.NetAddress{
							{
								Gateway: net.ParseIP("172.26.0.1"),
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.2"),
										Mask: net.CIDRMask(20, 32),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.3"),
										Mask: net.CIDRMask(10, 32),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.4"),
										Mask: net.CIDRMask(24, 32),
									},
								},
							},
						},
					},
				},
			},
			expectedNetworkConfig: define.InspectBasicNetworkConfig{
				IPAddress:   "172.26.0.2",
				IPPrefixLen: 20,
				Gateway:     "172.26.0.1",
				SecondaryIPAddresses: []define.Address{
					{
						Addr:         "172.26.0.3",
						PrefixLength: 10,
					},
					{
						Addr:         "172.26.0.4",
						PrefixLength: 24,
					},
				},
			},
		},
		{
			description: "single secondary IPv6 address is shown as define.Address",
			inputResult: types.StatusBlock{
				Interfaces: map[string]types.NetInterface{
					"eth1": {
						Subnets: []types.NetAddress{
							{
								Gateway: net.ParseIP("ff02::fb"),
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fc"),
										Mask: net.CIDRMask(20, 128),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fd"),
										Mask: net.CIDRMask(10, 128),
									},
								},
							},
						},
					},
				},
			},
			expectedNetworkConfig: define.InspectBasicNetworkConfig{
				GlobalIPv6Address:   "ff02::fc",
				GlobalIPv6PrefixLen: 20,
				IPv6Gateway:         "ff02::fb",
				SecondaryIPv6Addresses: []define.Address{
					{
						Addr:         "ff02::fd",
						PrefixLength: 10,
					},
				},
			},
		},
		{
			description: "multiple secondary IPv6 addresses are shown as define.Address",
			inputResult: types.StatusBlock{
				Interfaces: map[string]types.NetInterface{
					"eth1": {
						Subnets: []types.NetAddress{
							{
								Gateway: net.ParseIP("ff02::fb"),
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fc"),
										Mask: net.CIDRMask(20, 128),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fd"),
										Mask: net.CIDRMask(10, 128),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fe"),
										Mask: net.CIDRMask(24, 128),
									},
								},
							},
						},
					},
				},
			},
			expectedNetworkConfig: define.InspectBasicNetworkConfig{
				GlobalIPv6Address:   "ff02::fc",
				GlobalIPv6PrefixLen: 20,
				IPv6Gateway:         "ff02::fb",
				SecondaryIPv6Addresses: []define.Address{
					{
						Addr:         "ff02::fd",
						PrefixLength: 10,
					},
					{
						Addr:         "ff02::fe",
						PrefixLength: 24,
					},
				},
			},
		},
	}

	for _, tcl := range testCases {
		tc := tcl
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			actualNetworkConfig := resultToBasicNetworkConfig(tc.inputResult)

			if !reflect.DeepEqual(tc.expectedNetworkConfig, actualNetworkConfig) {
				t.Fatalf(
					"Expected networkConfig %+v didn't match actual value %+v", tc.expectedNetworkConfig, actualNetworkConfig)
			}
		})
	}
}
