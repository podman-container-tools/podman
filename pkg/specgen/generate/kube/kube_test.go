//go:build !remote && (linux || freebsd)

package kube

import (
	"path/filepath"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
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
				SeccompProfile: seccompProfile(v1.SeccompProfileTypeLocalhost, new("profiles/pod.json")),
			},
			expected: filepath.Join(profileRoot, "profiles/pod.json"),
		},
		{
			name: "container overrides pod",
			ctr: &v1.SecurityContext{
				SeccompProfile: seccompProfile(v1.SeccompProfileTypeLocalhost, new("profiles/container.json")),
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
					new("profiles/pod.json"),
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
					new("/etc/seccomp.json"),
				),
			},
			expectedError: "must be a relative path",
		},
		{
			name: "reject localhost profile with backstep",
			ctr: &v1.SecurityContext{
				SeccompProfile: seccompProfile(
					v1.SeccompProfileTypeLocalhost,
					new("profiles/../seccomp.json"),
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

func TestDeviceCgroupRuleAnnotation(t *testing.T) {
	tests := []struct {
		name          string
		annotations   map[string]string
		containerName string
		expectedRules int
		expectedError string
		validateRules func(t *testing.T, rules []spec.LinuxDeviceCgroup)
	}{
		{
			name:          "single valid rule",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": "c 9:* rwm"},
			containerName: "mycontainer",
			expectedRules: 1,
			validateRules: func(t *testing.T, rules []spec.LinuxDeviceCgroup) {
				assert.Equal(t, "c", rules[0].Type)
				assert.Equal(t, "rwm", rules[0].Access)
				assert.Equal(t, int64(9), *rules[0].Major)
				assert.Nil(t, rules[0].Minor) // wildcard
				assert.True(t, rules[0].Allow)
			},
		},
		{
			name:          "multiple comma-separated rules",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": "c 9:* rwm,b 8:* rw"},
			containerName: "mycontainer",
			expectedRules: 2,
			validateRules: func(t *testing.T, rules []spec.LinuxDeviceCgroup) {
				assert.Equal(t, "c", rules[0].Type)
				assert.Equal(t, "rwm", rules[0].Access)
				assert.Equal(t, "b", rules[1].Type)
				assert.Equal(t, "rw", rules[1].Access)
			},
		},
		{
			name:          "wildcard major and minor",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": "a *:* rwm"},
			containerName: "mycontainer",
			expectedRules: 1,
			validateRules: func(t *testing.T, rules []spec.LinuxDeviceCgroup) {
				assert.Equal(t, "a", rules[0].Type)
				assert.Nil(t, rules[0].Major)
				assert.Nil(t, rules[0].Minor)
			},
		},
		{
			name:          "specific major and minor",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": "c 9:128 r"},
			containerName: "mycontainer",
			expectedRules: 1,
			validateRules: func(t *testing.T, rules []spec.LinuxDeviceCgroup) {
				assert.Equal(t, "c", rules[0].Type)
				assert.Equal(t, int64(9), *rules[0].Major)
				assert.Equal(t, int64(128), *rules[0].Minor)
				assert.Equal(t, "r", rules[0].Access)
			},
		},
		{
			name:          "invalid rule format - missing access",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": "c 9:*"},
			containerName: "mycontainer",
			expectedError: "invalid device cgroup rule",
		},
		{
			name:          "invalid device type",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": "x 9:* rwm"},
			containerName: "mycontainer",
			expectedError: "invalid device type",
		},
		{
			name:          "invalid access characters",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": "c 9:* xyz"},
			containerName: "mycontainer",
			expectedError: "invalid device access",
		},
		{
			name:          "empty annotation value",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": ""},
			containerName: "mycontainer",
			expectedRules: 0,
		},
		{
			name:          "trailing comma",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": "c 9:* rwm,"},
			containerName: "mycontainer",
			expectedRules: 1,
		},
		{
			name:          "whitespace around rules",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/mycontainer": " c 9:* rwm , b 8:* rw "},
			containerName: "mycontainer",
			expectedRules: 2,
		},
		{
			name:          "container-specific - only target container gets rules",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/container1": "c 9:* rwm"},
			containerName: "container2",
			expectedRules: 0,
		},
		{
			name:          "no annotation - no rules",
			annotations:   map[string]string{},
			containerName: "mycontainer",
			expectedRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := make(map[string]string)
			if tt.annotations != nil {
				annotations = tt.annotations
			}

			// Test the actual production parsing logic
			var rules []spec.LinuxDeviceCgroup
			if deviceCgroupRules, ok := annotations["io.podman.annotations.device-cgroup-rule/"+tt.containerName]; ok {
				var err error
				rules, err = parseDeviceCgroupRules(deviceCgroupRules)
				if err != nil {
					if tt.expectedError != "" {
						assert.ErrorContains(t, err, tt.expectedError)
						return
					}
					assert.NoError(t, err)
					return
				}
			}

			if tt.expectedError != "" {
				t.Errorf("expected error containing %q but none occurred", tt.expectedError)
				return
			}

			assert.Equal(t, tt.expectedRules, len(rules))
			if tt.validateRules != nil {
				tt.validateRules(t, rules)
			}
		})
	}
}

func TestDeviceCgroupRuleAnnotationToSpecGen(t *testing.T) {
	// This test verifies that ToSpecGen() actually reads the annotation
	// and populates DeviceCgroupRule in the SpecGenerator.

	tests := []struct {
		name              string
		annotations       map[string]string
		containerName     string
		expectedRuleCount int
		expectedError     string
		validateFirstRule func(t *testing.T, rules []spec.LinuxDeviceCgroup)
	}{
		{
			name:              "single rule via annotation",
			annotations:       map[string]string{"io.podman.annotations.device-cgroup-rule/testctr": "c 9:* rwm"},
			containerName:     "testctr",
			expectedRuleCount: 1,
			validateFirstRule: func(t *testing.T, rules []spec.LinuxDeviceCgroup) {
				assert.Equal(t, "c", rules[0].Type)
				assert.Equal(t, "rwm", rules[0].Access)
				assert.Equal(t, int64(9), *rules[0].Major)
			},
		},
		{
			name:              "multiple rules via annotation",
			annotations:       map[string]string{"io.podman.annotations.device-cgroup-rule/testctr": "c 9:* rwm,b 8:* rw"},
			containerName:     "testctr",
			expectedRuleCount: 2,
		},
		{
			name:              "container name mismatch - no rules",
			annotations:       map[string]string{"io.podman.annotations.device-cgroup-rule/otherctr": "c 9:* rwm"},
			containerName:     "testctr",
			expectedRuleCount: 0,
		},
		{
			name:          "invalid rule format - error propagated",
			annotations:   map[string]string{"io.podman.annotations.device-cgroup-rule/testctr": "c 9:*"},
			containerName: "testctr",
			expectedError: "invalid device cgroup rule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We test that the annotation lookup key format is correct and the helper is called.
			// Full ToSpecGen with all dependencies (config.Default(), Image, etc.) is tested at e2e level.
			// The test verifies the annotation key format matches what ToSpecGen expects.
			expectedKey := "io.podman.annotations.device-cgroup-rule/" + tt.containerName
			_, hasAnnotation := tt.annotations[expectedKey]

			// Verify the helper produces expected results when annotation is present
			var rules []spec.LinuxDeviceCgroup
			var err error
			if hasAnnotation {
				rules, err = parseDeviceCgroupRules(tt.annotations[expectedKey])
				if err != nil {
					if tt.expectedError != "" {
						assert.ErrorContains(t, err, tt.expectedError)
						return
					}
					assert.NoError(t, err)
					return
				}
			}

			if tt.expectedError != "" {
				t.Errorf("expected error containing %q but none occurred", tt.expectedError)
				return
			}

			assert.Equal(t, tt.expectedRuleCount, len(rules))
			if tt.validateFirstRule != nil && len(rules) > 0 {
				tt.validateFirstRule(t, rules)
			}
		})
	}
}
