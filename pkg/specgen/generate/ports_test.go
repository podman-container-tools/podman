//go:build !remote && (linux || freebsd)

package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/libnetwork/types"
)

func TestParsePortMappingWithHostPort(t *testing.T) {
	tests := []struct {
		name string
		arg  []types.PortMapping
		arg2 map[uint16][]string
		want []types.PortMapping
	}{
		{
			name: "no ports",
			arg:  nil,
			want: nil,
		},
		{
			name: "one tcp port",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "one tcp port no proto",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "one udp port",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "udp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "udp",
					Range:         1,
				},
			},
		},
		{
			name: "one sctp port",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "sctp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "sctp",
					Range:         1,
				},
			},
		},
		{
			name: "one port two protocols",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp,udp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "udp",
					Range:         1,
				},
			},
		},
		{
			name: "one port three protocols",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp,udp,sctp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "udp",
					Range:         1,
				},
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "sctp",
					Range:         1,
				},
			},
		},
		{
			name: "one port with range 1",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "one port with range 5",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         5,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         5,
				},
			},
		},
		{
			name: "two ports joined",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
				{
					HostPort:      8081,
					ContainerPort: 81,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         2,
				},
			},
		},
		{
			name: "two ports joined with range",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         2,
				},
				{
					HostPort:      8081,
					ContainerPort: 81,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         2,
				},
			},
		},
		{
			name: "two ports with no overlapping range",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         10,
				},
				{
					HostPort:      9090,
					ContainerPort: 9090,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      9090,
					ContainerPort: 9090,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         10,
				},
			},
		},
		{
			name: "four ports with two overlapping ranges",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         10,
				},
				{
					HostPort:      8085,
					ContainerPort: 85,
					Protocol:      "tcp",
					Range:         10,
				},
				{
					HostPort:      100,
					ContainerPort: 5,
					Protocol:      "tcp",
				},
				{
					HostPort:      101,
					ContainerPort: 6,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         15,
				},
				{
					HostPort:      100,
					ContainerPort: 5,
					Protocol:      "tcp",
					Range:         2,
				},
			},
		},
		{
			name: "two overlapping ranges",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         10,
				},
				{
					HostPort:      8085,
					ContainerPort: 85,
					Protocol:      "tcp",
					Range:         2,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         10,
				},
			},
		},
		{
			name: "four overlapping ranges",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         10,
				},
				{
					HostPort:      8085,
					ContainerPort: 85,
					Protocol:      "tcp",
					Range:         2,
				},
				{
					HostPort:      8090,
					ContainerPort: 90,
					Protocol:      "tcp",
					Range:         7,
				},
				{
					HostPort:      8095,
					ContainerPort: 95,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         17,
				},
			},
		},
		{
			name: "one port range overlaps 5 ports",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Range:         20,
				},
				{
					HostPort:      8085,
					ContainerPort: 85,
					Range:         2,
				},
				{
					HostPort:      8090,
					ContainerPort: 90,
				},
				{
					HostPort:      8095,
					ContainerPort: 95,
				},
				{
					HostPort:      8096,
					ContainerPort: 96,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         20,
				},
			},
		},
		{
			name: "different host ip same port",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					HostIP:        "192.168.1.1",
				},
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					HostIP:        "192.168.2.1",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					HostIP:        "192.168.1.1",
					Range:         1,
				},
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					HostIP:        "192.168.2.1",
					Range:         1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePortMapping(tt.arg, tt.arg2)
			assert.NoError(t, err, "error is not nil")
			// use ElementsMatch instead of Equal because the order is not consistent
			assert.ElementsMatch(t, tt.want, got, "got unexpected port mapping")
		})
	}
}

func TestParsePortMappingWithoutHostPort(t *testing.T) {
	tests := []struct {
		name string
		arg  []types.PortMapping
		arg2 map[uint16][]string
		want []types.PortMapping
	}{
		{
			name: "one tcp port",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "one port with two protocols",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp,udp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "udp",
					Range:         1,
				},
			},
		},
		{
			name: "same port twice",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "neighbor ports are not joined",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
				{
					HostPort:      0,
					ContainerPort: 81,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 81,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "overlapping range ports are joined",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         2,
				},
				{
					HostPort:      0,
					ContainerPort: 81,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         2,
				},
			},
		},
		{
			name: "four overlapping range ports are joined",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         3,
				},
				{
					HostPort:      0,
					ContainerPort: 81,
					Protocol:      "tcp",
				},
				{
					HostPort:      0,
					ContainerPort: 82,
					Protocol:      "tcp",
					Range:         10,
				},
				{
					HostPort:      0,
					ContainerPort: 90,
					Protocol:      "tcp",
					Range:         5,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         15,
				},
			},
		},
		{
			name: "expose one tcp port",
			arg2: map[uint16][]string{
				8080: {"tcp"},
			},
			want: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 8080,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "expose already defined port",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 8080,
					Protocol:      "tcp",
				},
			},
			arg2: map[uint16][]string{
				8080: {"tcp"},
			},
			want: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 8080,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "expose different proto",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 8080,
					Protocol:      "tcp",
				},
			},
			arg2: map[uint16][]string{
				8080: {"udp"},
			},
			want: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 8080,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 8080,
					Protocol:      "udp",
					Range:         1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePortMapping(tt.arg, tt.arg2)
			assert.NoError(t, err, "error is not nil")

			// because we always get random host ports when it is set to 0 we cannot check that exactly
			// check if it is not 0 and set to 0 afterwards
			for i := range got {
				assert.Greater(t, got[i].HostPort, uint16(0), "host port is zero")
				got[i].HostPort = 0
			}

			// use ElementsMatch instead of Equal because the order is not consistent
			assert.ElementsMatch(t, tt.want, got, "got unexpected port mapping")
		})
	}
}

func TestParsePortMappingMixedHostPort(t *testing.T) {
	tests := []struct {
		name           string
		arg            []types.PortMapping
		want           []types.PortMapping
		resetHostPorts []int
	}{
		{
			name: "two ports one without a hostport set",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
				},
				{
					HostPort:      8080,
					ContainerPort: 8080,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 8080,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
			},
			resetHostPorts: []int{1},
		},
		{
			name: "two ports one without a hostport set, inverted order",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 8080,
				},
				{
					HostPort:      0,
					ContainerPort: 80,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 8080,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
			},
			resetHostPorts: []int{1},
		},
		{
			name: "three ports without host ports, one with a hostport set, , inverted order",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
				},
				{
					HostPort:      0,
					ContainerPort: 85,
				},
				{
					HostPort:      0,
					ContainerPort: 90,
				},
				{
					HostPort:      8080,
					ContainerPort: 8080,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 8080,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 85,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 90,
					Protocol:      "tcp",
					Range:         1,
				},
			},
			resetHostPorts: []int{1, 2, 3},
		},
		{
			name: "three ports without host ports, one with a hostport set",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 8080,
				},
				{
					HostPort:      0,
					ContainerPort: 90,
				},
				{
					HostPort:      0,
					ContainerPort: 85,
				},
				{
					HostPort:      0,
					ContainerPort: 80,
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 8080,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 85,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 90,
					Protocol:      "tcp",
					Range:         1,
				},
			},
			resetHostPorts: []int{1, 2, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePortMapping(tt.arg, nil)
			assert.NoError(t, err, "error is not nil")

			// because we always get random host ports when it is set to 0 we cannot check that exactly
			// use resetHostPorts to know which port element is 0
			for _, num := range tt.resetHostPorts {
				assert.Greater(t, got[num].HostPort, uint16(0), "host port is zero")
				got[num].HostPort = 0
			}

			assert.Equal(t, tt.want, got, "got unexpected port mapping")
		})
	}
}

func TestParsePortMappingError(t *testing.T) {
	tests := []struct {
		name string
		arg  []types.PortMapping
		err  string
	}{
		{
			name: "container port is 0",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 0,
					Protocol:      "tcp",
				},
			},
			err: "container port number must be non-0",
		},
		{
			name: "container port range exceeds max",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 65000,
					Protocol:      "tcp",
					Range:         10000,
				},
			},
			err: "container port range exceeds maximum allowable port number",
		},
		{
			name: "host port range exceeds max",
			arg: []types.PortMapping{
				{
					HostPort:      60000,
					ContainerPort: 1,
					Protocol:      "tcp",
					Range:         10000,
				},
			},
			err: "host port range exceeds maximum allowable port number",
		},
		{
			name: "invalid protocol",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "1",
				},
			},
			err: "unrecognized protocol \"1\" in port mapping",
		},
		{
			name: "invalid protocol 2",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "udp,u",
				},
			},
			err: "unrecognized protocol \"u\" in port mapping",
		},
		{
			name: "invalid ip address",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					HostIP:        "blah",
				},
			},
			err: "invalid IP address \"blah\" in port mapping",
		},
		{
			name: "invalid overalpping range",
			arg: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Range:         5,
				},
				{
					HostPort:      8081,
					ContainerPort: 60,
				},
			},
			err: "conflicting port mappings for host port 8081 (protocol tcp)",
		},
		{
			name: "big port range with host port zero does not fit",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 1,
					Range:         65535,
				},
			},
			err: "failed to find an open port to expose container port 1 with range 65535 on the host",
		},
		{
			name: "big port range with host port zero does not fit",
			arg: []types.PortMapping{
				{
					HostPort:      0,
					ContainerPort: 80,
					Range:         1,
				},
				{
					HostPort:      0,
					ContainerPort: 1000,
					Range:         64535,
				},
			},
			err: "failed to find an open port to expose container port 1000 with range 64535 on the host",
		},
		{
			name: "host port conflict",
			arg: []types.PortMapping{
				{
					HostPort:      80,
					ContainerPort: 80,
					Range:         1,
				},
				{
					HostPort:      80,
					ContainerPort: 8080,
					Range:         1,
				},
			},
			err: `host port ":80/tcp" was already assigned in another port mapping`,
		},
		{
			name: "host port conflict with host ip",
			arg: []types.PortMapping{
				{
					HostIP:        "127.0.0.1",
					HostPort:      1234,
					ContainerPort: 80,
					Range:         1,
				},
				{
					HostIP:        "127.0.0.1",
					HostPort:      1234,
					ContainerPort: 81,
					Range:         1,
				},
			},
			err: `host port "127.0.0.1:1234/tcp" was already assigned in another port mapping`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePortMapping(tt.arg, nil)
			assert.EqualError(t, err, tt.err, "error does not match")
		})
	}
}
