//go:build !remote && (linux || freebsd)

package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/api/resource"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/util/intstr"
)

func testPropagation(t *testing.T, propagation v1.MountPropagationMode, expected string) {
	dest, options, err := parseMountPath("/to", false, &propagation)
	assert.NoError(t, err)
	assert.Equal(t, dest, "/to")
	assert.Contains(t, options, expected)
}

func TestParseMountPathPropagation(t *testing.T) {
	testPropagation(t, v1.MountPropagationNone, "private")
	testPropagation(t, v1.MountPropagationHostToContainer, "rslave")
	testPropagation(t, v1.MountPropagationBidirectional, "rshared")

	prop := v1.MountPropagationMode("SpaceWave")
	_, _, err := parseMountPath("/to", false, &prop)
	assert.Error(t, err)

	_, options, err := parseMountPath("/to", false, nil)
	assert.NoError(t, err)
	assert.NotContains(t, options, "private")
	assert.NotContains(t, options, "rslave")
	assert.NotContains(t, options, "rshared")
}

func TestParseMountPathRO(t *testing.T) {
	_, options, err := parseMountPath("/to", true, nil)
	assert.NoError(t, err)
	assert.Contains(t, options, "ro")

	_, options, err = parseMountPath("/to", false, nil)
	assert.NoError(t, err)
	assert.NotContains(t, options, "ro")
}

func TestGetPodPorts(t *testing.T) {
	c1 := v1.Container{
		Name: "container1",
		Ports: []v1.ContainerPort{{
			ContainerPort: 5000,
		}, {
			ContainerPort: 5001,
			HostPort:      5002,
		}},
	}
	c2 := v1.Container{
		Name: "container2",
		Ports: []v1.ContainerPort{{
			HostPort: 5004,
		}},
	}
	r, err := getPodPorts([]v1.Container{c1, c2}, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(r))
	assert.Equal(t, uint16(5001), r[0].ContainerPort)
	assert.Equal(t, uint16(5002), r[0].HostPort)
	assert.Equal(t, uint16(5004), r[1].ContainerPort)
	assert.Equal(t, uint16(5004), r[1].HostPort)

	r, err = getPodPorts([]v1.Container{c1, c2}, true)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(r))
	assert.Equal(t, uint16(5000), r[0].ContainerPort)
	assert.Equal(t, uint16(5000), r[0].HostPort)
	assert.Equal(t, uint16(5001), r[1].ContainerPort)
	assert.Equal(t, uint16(5002), r[1].HostPort)
	assert.Equal(t, uint16(5004), r[2].ContainerPort)
	assert.Equal(t, uint16(5004), r[2].HostPort)
}

func TestGetPodPortsDuplicateHostPort(t *testing.T) {
	c1 := v1.Container{
		Name: "nginx-1",
		Ports: []v1.ContainerPort{{
			ContainerPort: 80,
			HostPort:      8077,
		}},
	}
	c2 := v1.Container{
		Name: "nginx-2",
		Ports: []v1.ContainerPort{{
			ContainerPort: 80,
			HostPort:      8077,
		}},
	}

	// Same hostPort on two containers must produce an error.
	_, err := getPodPorts([]v1.Container{c1, c2}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nginx-1")
	assert.Contains(t, err.Error(), "nginx-2")
	assert.Contains(t, err.Error(), "8077")

	// Different hostPorts must succeed.
	c2.Ports[0].HostPort = 8078
	r, err := getPodPorts([]v1.Container{c1, c2}, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(r))

	// Same hostPort but different protocols must succeed.
	c2.Ports[0].HostPort = 8077
	c2.Ports[0].Protocol = "udp"
	r, err = getPodPorts([]v1.Container{c1, c2}, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(r))
}

func TestGetPortNumber(t *testing.T) {
	portSpec := intstr.IntOrString{Type: intstr.Int, IntVal: 3000, StrVal: "myport"}
	cp1 := v1.ContainerPort{Name: "myport", ContainerPort: 4000}
	cp2 := v1.ContainerPort{Name: "myport2", ContainerPort: 5000}
	i, e := getPortNumber(portSpec, []v1.ContainerPort{cp1, cp2})
	assert.NoError(t, e)
	assert.Equal(t, i, int(portSpec.IntVal))

	portSpec.Type = intstr.String
	i, e = getPortNumber(portSpec, []v1.ContainerPort{cp1, cp2})
	assert.NoError(t, e)
	assert.Equal(t, i, 4000)

	portSpec.StrVal = "not_valid"
	_, e = getPortNumber(portSpec, []v1.ContainerPort{cp1, cp2})
	assert.Error(t, e)

	portSpec.StrVal = "6000"
	i, e = getPortNumber(portSpec, []v1.ContainerPort{cp1, cp2})
	assert.NoError(t, e)
	assert.Equal(t, i, 6000)
}

func TestQuantityToInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "integer BinarySI",
			input:    "1536Mi",
			expected: 1536 * 1024 * 1024,
		},
		{
			name:     "fractional BinarySI",
			input:    "1.5Gi",
			expected: 1536 * 1024 * 1024,
		},
		{
			name:     "larger fractional BinarySI",
			input:    "2.5Gi",
			expected: 2560 * 1024 * 1024,
		},
		{
			name:     "fractional DecimalSI",
			input:    "1.5G",
			expected: 1500000000,
		},
		{
			name:     "zero",
			input:    "0",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := resource.MustParse(tt.input)
			got := quantityToInt64(&q)
			assert.Equal(t, tt.expected, got)
		})
	}
}
