package containers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/libnetwork/types"
)

func Test_portsToString(t *testing.T) {
	tests := []struct {
		name         string
		ports        []types.PortMapping
		exposedPorts map[uint16][]string
		want         string
	}{
		{
			name: "no ports",
			want: "",
		},
		{
			name:         "no ports empty map/slice",
			ports:        []types.PortMapping{},
			exposedPorts: map[uint16][]string{},
			want:         "",
		},
		{
			name: "single published port",
			ports: []types.PortMapping{
				{
					ContainerPort: 80,
					HostPort:      8080,
					Protocol:      "tcp",
					Range:         1,
				},
			},
			want: "0.0.0.0:8080->80/tcp",
		},
		{
			name: "published port with host ip",
			ports: []types.PortMapping{
				{
					ContainerPort: 80,
					HostPort:      8080,
					HostIP:        "127.0.0.1",
					Protocol:      "tcp",
					Range:         1,
				},
			},
			want: "127.0.0.1:8080->80/tcp",
		},
		{
			name: "published port with same exposed port",
			ports: []types.PortMapping{
				{
					ContainerPort: 80,
					HostPort:      8080,
					Protocol:      "tcp",
					Range:         1,
				},
			},
			exposedPorts: map[uint16][]string{
				80: {"tcp"},
			},
			want: "0.0.0.0:8080->80/tcp",
		},
		{
			name: "published port and exposed port with different protocol",
			ports: []types.PortMapping{
				{
					ContainerPort: 80,
					HostPort:      8080,
					Protocol:      "tcp",
					Range:         1,
				},
			},
			exposedPorts: map[uint16][]string{
				80: {"udp"},
			},
			want: "0.0.0.0:8080->80/tcp, 80/udp",
		},
		{
			name: "published port range",
			ports: []types.PortMapping{
				{
					ContainerPort: 80,
					HostPort:      8080,
					Protocol:      "tcp",
					Range:         3,
				},
			},
			want: "0.0.0.0:8080-8082->80-82/tcp",
		},
		{
			name: "published port range and exposed port in that range",
			ports: []types.PortMapping{
				{
					ContainerPort: 80,
					HostPort:      8080,
					Protocol:      "tcp",
					Range:         3,
				},
			},
			exposedPorts: map[uint16][]string{
				81: {"tcp"},
			},
			want: "0.0.0.0:8080-8082->80-82/tcp",
		},
		{
			name: "two published ports",
			ports: []types.PortMapping{
				{
					ContainerPort: 80,
					HostPort:      8080,
					Protocol:      "tcp",
					Range:         3,
				},
				{
					ContainerPort: 80,
					HostPort:      8080,
					Protocol:      "udp",
					Range:         1,
				},
			},
			want: "0.0.0.0:8080-8082->80-82/tcp, 0.0.0.0:8080->80/udp",
		},
		{
			name: "exposed port",
			exposedPorts: map[uint16][]string{
				80: {"tcp"},
			},
			want: "80/tcp",
		},
		{
			name: "exposed port multiple protocols",
			exposedPorts: map[uint16][]string{
				80: {"tcp", "udp"},
			},
			want: "80/tcp, 80/udp",
		},
		{
			name: "exposed port range",
			exposedPorts: map[uint16][]string{
				80: {"tcp"},
				81: {"tcp"},
				82: {"tcp"},
			},
			want: "80-82/tcp",
		},
		{
			name: "exposed port range with different protocols",
			exposedPorts: map[uint16][]string{
				80: {"tcp", "udp"},
				81: {"tcp", "sctp"},
				82: {"tcp", "udp"},
			},
			want: "81/sctp, 80-82/tcp, 80/udp, 82/udp",
		},
		{
			name: "multiple exposed port ranges",
			exposedPorts: map[uint16][]string{
				80: {"tcp"},
				81: {"tcp"},
				82: {"tcp"},
				// 83 missing to split the range
				84: {"tcp"},
				85: {"tcp"},
				86: {"tcp"},
			},
			want: "80-82/tcp, 84-86/tcp",
		},
		{
			name: "published port range partially overlaps with exposed port range",
			ports: []types.PortMapping{
				{
					ContainerPort: 80,
					HostPort:      8080,
					Protocol:      "tcp",
					Range:         3,
				},
			},
			exposedPorts: map[uint16][]string{
				82: {"tcp"},
				83: {"tcp"},
				84: {"tcp"},
			},
			want: "0.0.0.0:8080-8082->80-82/tcp, 83-84/tcp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := portsToString(tt.ports, tt.exposedPorts)
			assert.Equal(t, tt.want, got)
		})
	}
}
