//go:build !remote && (linux || freebsd)

package generate

import (
	"fmt"
	"testing"

	"go.podman.io/common/libnetwork/types"
)

func benchmarkParsePortMapping(b *testing.B, ports []types.PortMapping) {
	for b.Loop() {
		_, _ = ParsePortMapping(ports, nil)
	}
}

func BenchmarkParsePortMappingNoPorts(b *testing.B) {
	benchmarkParsePortMapping(b, nil)
}

func BenchmarkParsePortMapping1(b *testing.B) {
	benchmarkParsePortMapping(b, []types.PortMapping{
		{
			HostPort:      8080,
			ContainerPort: 80,
			Protocol:      "tcp",
		},
	})
}

func BenchmarkParsePortMapping100(b *testing.B) {
	ports := make([]types.PortMapping, 0, 100)
	for i := uint16(8080); i < 8180; i++ {
		ports = append(ports, types.PortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkParsePortMapping(b, ports)
}

func BenchmarkParsePortMapping1k(b *testing.B) {
	ports := make([]types.PortMapping, 0, 1000)
	for i := uint16(8080); i < 9080; i++ {
		ports = append(ports, types.PortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkParsePortMapping(b, ports)
}

func BenchmarkParsePortMapping10k(b *testing.B) {
	ports := make([]types.PortMapping, 0, 30000)
	for i := uint16(8080); i < 18080; i++ {
		ports = append(ports, types.PortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkParsePortMapping(b, ports)
}

func BenchmarkParsePortMapping1m(b *testing.B) {
	ports := make([]types.PortMapping, 0, 1000000)
	for j := range 20 {
		for i := uint16(1); i <= 50000; i++ {
			ports = append(ports, types.PortMapping{
				HostPort:      i,
				ContainerPort: i,
				Protocol:      "tcp",
				HostIP:        fmt.Sprintf("192.168.1.%d", j),
			})
		}
	}
	b.ResetTimer()
	benchmarkParsePortMapping(b, ports)
}

func BenchmarkParsePortMappingReverse100(b *testing.B) {
	ports := make([]types.PortMapping, 0, 100)
	for i := uint16(8180); i > 8080; i-- {
		ports = append(ports, types.PortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkParsePortMapping(b, ports)
}

func BenchmarkParsePortMappingReverse1k(b *testing.B) {
	ports := make([]types.PortMapping, 0, 1000)
	for i := uint16(9080); i > 8080; i-- {
		ports = append(ports, types.PortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkParsePortMapping(b, ports)
}

func BenchmarkParsePortMappingReverse10k(b *testing.B) {
	ports := make([]types.PortMapping, 0, 30000)
	for i := uint16(18080); i > 8080; i-- {
		ports = append(ports, types.PortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkParsePortMapping(b, ports)
}

func BenchmarkParsePortMappingReverse1m(b *testing.B) {
	ports := make([]types.PortMapping, 0, 1000000)
	for j := range 20 {
		for i := uint16(50000); i > 0; i-- {
			ports = append(ports, types.PortMapping{
				HostPort:      i,
				ContainerPort: i,
				Protocol:      "tcp",
				HostIP:        fmt.Sprintf("192.168.1.%d", j),
			})
		}
	}
	b.ResetTimer()
	benchmarkParsePortMapping(b, ports)
}

func BenchmarkParsePortMappingRange1(b *testing.B) {
	benchmarkParsePortMapping(b, []types.PortMapping{
		{
			HostPort:      8080,
			ContainerPort: 80,
			Protocol:      "tcp",
			Range:         1,
		},
	})
}

func BenchmarkParsePortMappingRange100(b *testing.B) {
	benchmarkParsePortMapping(b, []types.PortMapping{
		{
			HostPort:      8080,
			ContainerPort: 80,
			Protocol:      "tcp",
			Range:         100,
		},
	})
}

func BenchmarkParsePortMappingRange1k(b *testing.B) {
	benchmarkParsePortMapping(b, []types.PortMapping{
		{
			HostPort:      8080,
			ContainerPort: 80,
			Protocol:      "tcp",
			Range:         1000,
		},
	})
}

func BenchmarkParsePortMappingRange10k(b *testing.B) {
	benchmarkParsePortMapping(b, []types.PortMapping{
		{
			HostPort:      8080,
			ContainerPort: 80,
			Protocol:      "tcp",
			Range:         10000,
		},
	})
}

func BenchmarkParsePortMappingRange1m(b *testing.B) {
	ports := make([]types.PortMapping, 0, 1000000)
	for j := range 20 {
		ports = append(ports, types.PortMapping{
			HostPort:      1,
			ContainerPort: 1,
			Protocol:      "tcp",
			Range:         50000,
			HostIP:        fmt.Sprintf("192.168.1.%d", j),
		})
	}
	b.ResetTimer()
	benchmarkParsePortMapping(b, ports)
}
