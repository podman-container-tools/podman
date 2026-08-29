//go:build !remote && (linux || freebsd)

package kube

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/libpod"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/api/resource"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/util/intstr"
	"go.podman.io/podman/v6/pkg/specgen"
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

func seccompProfile(profileType v1.SeccompProfileType, localhostProfile *string) *v1.SeccompProfile {
	return &v1.SeccompProfile{Type: profileType, LocalhostProfile: localhostProfile}
}

func stringPtr(value string) *string {
	return &value
}

func TestSetupSecurityContextSeccompProfile(t *testing.T) {
	profileRoot := t.TempDir()
	defaultPath, err := libpod.DefaultSeccompPath()
	assert.NoError(t, err)

	tests := []struct {
		name                   string
		ctr                    *v1.SecurityContext
		pod                    *v1.PodSecurityContext
		seccompAnnotationPaths *SeccompAnnotationPaths
		ctrName                string
		expected               string
		expectedError          string
	}{
		{
			name: "container profile",
			ctr: &v1.SecurityContext{
				SeccompProfile: seccompProfile(v1.SeccompProfileTypeUnconfined, nil),
			},
			expected: "unconfined",
		},
		{
			name: "pod profile",
			pod: &v1.PodSecurityContext{
				SeccompProfile: seccompProfile(v1.SeccompProfileTypeLocalhost, stringPtr("profiles/pod.json")),
			},
			expected: filepath.Join(profileRoot, "profiles/pod.json"),
		},
		{
			name: "container overrides pod",
			ctr: &v1.SecurityContext{
				SeccompProfile: seccompProfile(v1.SeccompProfileTypeLocalhost, stringPtr("profiles/container.json")),
			},
			pod: &v1.PodSecurityContext{
				SeccompProfile: seccompProfile(v1.SeccompProfileTypeUnconfined, nil),
			},
			expected: filepath.Join(profileRoot, "profiles/container.json"),
		},
		{
			name: "container securityContext overrides container annotation",
			ctr: &v1.SecurityContext{
				SeccompProfile: seccompProfile(
					v1.SeccompProfileTypeUnconfined,
					nil,
				),
			},
			seccompAnnotationPaths: &SeccompAnnotationPaths{
				containerPaths: map[string]string{
					"test-container": filepath.Join(profileRoot, "annotation.json"),
				},
			},
			ctrName:  "test-container",
			expected: "unconfined",
		},
		{
			name: "pod securityContext overrides pod annotation",
			pod: &v1.PodSecurityContext{
				SeccompProfile: seccompProfile(
					v1.SeccompProfileTypeLocalhost,
					stringPtr("profiles/pod.json"),
				),
			},
			seccompAnnotationPaths: &SeccompAnnotationPaths{
				containerPaths: map[string]string{},
				podPath:        filepath.Join(profileRoot, "annotation.json"),
			},
			expected: filepath.Join(profileRoot, "profiles/pod.json"),
		},
		{
			name:     "unset profile uses default",
			expected: defaultPath,
		},
		{
			name: "RuntimeDefault profile",
			ctr: &v1.SecurityContext{
				SeccompProfile: seccompProfile(v1.SeccompProfileTypeRuntimeDefault, nil),
			},
			expected: defaultPath,
		},
		{
			name: "reject empty seccomp profile type",
			ctr: &v1.SecurityContext{
				SeccompProfile: &v1.SeccompProfile{},
			},
			expectedError: "invalid seccomp profile type",
		},
		{
			name: "reject absolute localhost profile",
			ctr: &v1.SecurityContext{
				SeccompProfile: seccompProfile(
					v1.SeccompProfileTypeLocalhost,
					stringPtr("/etc/seccomp.json"),
				),
			},
			expectedError: "must be a relative path",
		},
		{
			name: "reject localhost profile with backstep",
			ctr: &v1.SecurityContext{
				SeccompProfile: seccompProfile(
					v1.SeccompProfileTypeLocalhost,
					stringPtr("profiles/../seccomp.json"),
				),
			},
			expectedError: "must not contain '..'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &specgen.SpecGenerator{}
			err := setupSecurityContext(s, tt.ctr, tt.pod, profileRoot, tt.seccompAnnotationPaths, tt.ctrName)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, s.SeccompProfilePath)
		})
	}
}
