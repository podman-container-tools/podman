package quadlet

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/api/resource"
	metav1 "go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/apis/meta/v1"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/util/intstr"
	"go.podman.io/podman/v6/pkg/systemd/quadlet"
)

// helpers

func minimalPod(name, image string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "app", Image: image},
			},
		},
	}
}

func findFile(files []*GeneratedFile, suffix string) *GeneratedFile {
	for _, f := range files {
		if strings.HasSuffix(f.Name, suffix) {
			return f
		}
	}
	return nil
}

// podUnit returns the .pod GeneratedFile from the output slice.
func podUnit(files []*GeneratedFile) *GeneratedFile {
	for _, f := range files {
		if strings.HasSuffix(f.Name, ".pod") {
			return f
		}
	}
	return nil
}

// containerUnit returns the .container GeneratedFile for the given container name.
func containerUnit(files []*GeneratedFile, prefix, name string) *GeneratedFile {
	return findFile(files, fmt.Sprintf("%s-%s.container", prefix, name))
}

func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}

func intOrString(i int) intstr.IntOrString {
	return intstr.FromInt(i)
}

// ── tests ──────────────────────────────────────────────────────────────────

func TestConvert_AlwaysEmitsPodUnit(t *testing.T) {
	pod := minimalPod("myapp", "nginx:latest")
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	pu := podUnit(files)
	require.NotNil(t, pu, "expected .pod unit in output")
	assert.Equal(t, "myapp.pod", pu.Name)

	// PodName= must be set
	name, ok := pu.Unit.Lookup(quadlet.PodGroup, quadlet.KeyPodName)
	assert.True(t, ok)
	assert.Equal(t, "myapp", name)

	// ExitPolicy=continue must be set so the infra container (pause) outlives
	// individual workload containers — including init containers that complete
	// before main containers start. Without this, Podman's default "stop" policy
	// tears down the infra on the first container exit, killing the pod.
	ep, ok := pu.Unit.Lookup(quadlet.PodGroup, quadlet.KeyExitPolicy)
	assert.True(t, ok, "ExitPolicy must be set on pod unit")
	assert.Equal(t, "continue", ep)

	// Pod unit is WantedBy default.target
	wb := pu.Unit.LookupAll(quadlet.InstallGroup, "WantedBy")
	assert.Contains(t, wb, "default.target")
}

func TestConvert_ContainerJoinsPod(t *testing.T) {
	pod := minimalPod("myapp", "nginx:latest")
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	cu := containerUnit(files, "myapp", "app")
	require.NotNil(t, cu)

	pod_, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyPod)
	assert.True(t, ok)
	assert.Equal(t, "myapp.pod", pod_)

	// Container WantedBy pod service
	wb := cu.Unit.LookupAll(quadlet.InstallGroup, "WantedBy")
	assert.Contains(t, wb, "myapp-pod.service")
}

func TestConvert_NamePrefix(t *testing.T) {
	pod := minimalPod("myapp", "nginx:latest")
	files, err := Convert(pod, Options{NamePrefix: "custom"})
	require.NoError(t, err)

	assert.NotNil(t, findFile(files, "custom.pod"))
	assert.NotNil(t, findFile(files, "custom-app.container"))
}

func TestConvert_InitContainerOrdering(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				{Name: "init-a", Image: "busybox"},
				{Name: "init-b", Image: "busybox"},
			},
			Containers: []v1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	initA := containerUnit(files, "p", "init-a")
	require.NotNil(t, initA)
	// init-a has no After= (no prior init services)
	assert.False(t, initA.Unit.HasKey(quadlet.UnitGroup, "After"))
	// init-a is oneshot
	svcType, _ := initA.Unit.Lookup(quadlet.ServiceGroup, "Type")
	assert.Equal(t, "oneshot", svcType)
	// init-a WantedBy first regular container
	wb := initA.Unit.LookupAll(quadlet.InstallGroup, "WantedBy")
	assert.Contains(t, wb, "p-app.service")

	initB := containerUnit(files, "p", "init-b")
	require.NotNil(t, initB)
	// init-b After= init-a
	afters := initB.Unit.LookupAll(quadlet.UnitGroup, "After")
	assert.Contains(t, afters, "p-init-a.service")

	// Both init containers join the pod
	for _, iu := range []*GeneratedFile{initA, initB} {
		pod_, ok := iu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyPod)
		assert.True(t, ok)
		assert.Equal(t, "p.pod", pod_)
	}

	// app container After= both init services
	appCU := containerUnit(files, "p", "app")
	require.NotNil(t, appCU)
	appAfters := appCU.Unit.LookupAll(quadlet.UnitGroup, "After")
	assert.Contains(t, appAfters, "p-init-a.service")
	assert.Contains(t, appAfters, "p-init-b.service")
}

func TestConvert_MultipleContainersAllJoinPod(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "web", Image: "nginx"},
				{Name: "proxy", Image: "envoy"},
			},
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	for _, name := range []string{"web", "proxy"} {
		cu := containerUnit(files, "p", name)
		require.NotNil(t, cu, "missing container unit for %s", name)
		pod_, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyPod)
		assert.True(t, ok)
		assert.Equal(t, "p.pod", pod_)
		// No explicit Network= in container unit — networking is handled by pod
		assert.False(t, cu.Unit.HasKey(quadlet.ContainerGroup, quadlet.KeyNetwork),
			"container %s must not have Network= (pod handles it)", name)
	}
}

func TestConvert_NoAnchorPattern(t *testing.T) {
	// Regression: sidecars must not use Network=container: or --volumes-from.
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "web", Image: "nginx"},
				{Name: "proxy", Image: "envoy"},
			},
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	for _, f := range files {
		if f.Unit == nil {
			continue
		}
		content, err := f.Unit.ToString()
		require.NoError(t, err)
		assert.NotContains(t, content, "Network=container:", "anchor pattern found in %s", f.Name)
		assert.NotContains(t, content, "--volumes-from", "anchor pattern found in %s", f.Name)
	}
}

func TestConvert_RestartPolicy(t *testing.T) {
	tests := []struct {
		policy v1.RestartPolicy
		want   string
	}{
		{v1.RestartPolicyAlways, "always"},
		{v1.RestartPolicyOnFailure, "on-failure"},
		{v1.RestartPolicyNever, ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.policy), func(t *testing.T) {
			pod := minimalPod("p", "nginx")
			pod.Spec.RestartPolicy = tt.policy
			files, err := Convert(pod, Options{})
			require.NoError(t, err)
			cu := containerUnit(files, "p", "app")
			require.NotNil(t, cu)
			val, ok := cu.Unit.Lookup(quadlet.ServiceGroup, "Restart")
			if tt.want == "" {
				assert.False(t, ok)
			} else {
				assert.True(t, ok)
				assert.Equal(t, tt.want, val)
			}
		})
	}
}

func TestConvert_TerminationGracePeriod_OnPodUnit(t *testing.T) {
	pod := minimalPod("p", "nginx")
	grace := int64(30)
	pod.Spec.TerminationGracePeriodSeconds = &grace
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	pu := podUnit(files)
	require.NotNil(t, pu)
	val, ok := pu.Unit.Lookup(quadlet.PodGroup, quadlet.KeyStopTimeout)
	assert.True(t, ok)
	assert.Equal(t, "30", val)

	// Must NOT be on the container unit
	cu := containerUnit(files, "p", "app")
	require.NotNil(t, cu)
	assert.False(t, cu.Unit.HasKey(quadlet.ContainerGroup, quadlet.KeyStopTimeout))
}

func TestConvert_ImagePullPolicy(t *testing.T) {
	tests := []struct {
		policy v1.PullPolicy
		want   string
	}{
		{v1.PullAlways, "always"},
		{v1.PullNever, "never"},
		{v1.PullIfNotPresent, "missing"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.policy), func(t *testing.T) {
			pod := minimalPod("p", "nginx")
			pod.Spec.Containers[0].ImagePullPolicy = tt.policy
			files, err := Convert(pod, Options{})
			require.NoError(t, err)
			cu := containerUnit(files, "p", "app")
			require.NotNil(t, cu)
			val, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyPull)
			if tt.want == "" {
				assert.False(t, ok)
			} else {
				assert.Equal(t, tt.want, val)
			}
		})
	}
}

func TestConvert_Network_OnPodUnit(t *testing.T) {
	pod := minimalPod("p", "nginx")
	files, err := Convert(pod, Options{Network: "podman"})
	require.NoError(t, err)
	pu := podUnit(files)
	require.NotNil(t, pu)
	net, ok := pu.Unit.Lookup(quadlet.PodGroup, quadlet.KeyNetwork)
	assert.True(t, ok)
	assert.Equal(t, "podman", net)
}

func TestConvert_HostNetwork_OnPodUnit(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.HostNetwork = true
	files, err := Convert(pod, Options{Network: "podman"})
	require.NoError(t, err)
	pu := podUnit(files)
	require.NotNil(t, pu)
	// hostNetwork overrides opts.Network
	net, ok := pu.Unit.Lookup(quadlet.PodGroup, quadlet.KeyNetwork)
	assert.True(t, ok)
	assert.Equal(t, "host", net)
}

func TestConvert_HostPID_OnPodUnit(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.HostPID = true
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	pu := podUnit(files)
	require.NotNil(t, pu)
	args := pu.Unit.LookupAll(quadlet.PodGroup, quadlet.KeyPodmanArgs)
	assert.Contains(t, args, "--pid=host")
}

func TestConvert_HostIPC_OnPodUnit(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.HostIPC = true
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	pu := podUnit(files)
	require.NotNil(t, pu)
	args := pu.Unit.LookupAll(quadlet.PodGroup, quadlet.KeyPodmanArgs)
	assert.Contains(t, args, "--ipc=host")
}

func TestConvert_Hostname_HostAliases_DNS_OnPodUnit(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Hostname = "myhost"
	pod.Spec.HostAliases = []v1.HostAlias{
		{IP: "1.2.3.4", Hostnames: []string{"foo.local", "bar.local"}},
	}
	pod.Spec.DNSConfig = &v1.PodDNSConfig{
		Nameservers: []string{"8.8.8.8"},
		Searches:    []string{"example.com"},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	pu := podUnit(files)
	require.NotNil(t, pu)

	hn, _ := pu.Unit.Lookup(quadlet.PodGroup, quadlet.KeyHostName)
	assert.Equal(t, "myhost", hn)

	hosts := pu.Unit.LookupAll(quadlet.PodGroup, quadlet.KeyAddHost)
	assert.Contains(t, hosts, "foo.local:1.2.3.4")
	assert.Contains(t, hosts, "bar.local:1.2.3.4")

	dns := pu.Unit.LookupAll(quadlet.PodGroup, quadlet.KeyDNS)
	assert.Contains(t, dns, "8.8.8.8")

	search := pu.Unit.LookupAll(quadlet.PodGroup, quadlet.KeyDNSSearch)
	assert.Contains(t, search, "example.com")
}

func TestConvert_ShareProcessNamespace_OnPodUnit(t *testing.T) {
	yes := true
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: v1.PodSpec{
			ShareProcessNamespace: &yes,
			Containers: []v1.Container{
				{Name: "a", Image: "nginx"},
				{Name: "b", Image: "busybox"},
			},
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	pu := podUnit(files)
	require.NotNil(t, pu)
	args := pu.Unit.LookupAll(quadlet.PodGroup, quadlet.KeyPodmanArgs)
	assert.Contains(t, args, "--share pid")
}

func TestConvert_PodLevelSysctls_OnPodUnit(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.SecurityContext = &v1.PodSecurityContext{
		Sysctls: []v1.Sysctl{
			{Name: "net.ipv4.ip_forward", Value: "1"},
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	pu := podUnit(files)
	require.NotNil(t, pu)
	args := pu.Unit.LookupAll(quadlet.PodGroup, quadlet.KeyPodmanArgs)
	assert.True(t, func() bool {
		for _, a := range args {
			if strings.Contains(a, "--sysctl") && strings.Contains(a, "net.ipv4.ip_forward=1") {
				return true
			}
		}
		return false
	}(), "expected --sysctl net.ipv4.ip_forward=1 in pod PodmanArgs")
}

func TestConvert_PodLevelSupplementalGroups_OnContainers(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.SecurityContext = &v1.PodSecurityContext{
		SupplementalGroups: []int64{1001, 1002},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	require.NotNil(t, cu)
	groups := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyGroupAdd)
	assert.Contains(t, groups, "1001")
	assert.Contains(t, groups, "1002")
}

func TestConvert_SecurityContext_Capabilities(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].SecurityContext = &v1.SecurityContext{
		Capabilities: &v1.Capabilities{
			Add:  []v1.Capability{"NET_ADMIN", "SYS_TIME"},
			Drop: []v1.Capability{"ALL"},
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	require.NotNil(t, cu)

	adds := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyAddCapability)
	assert.Contains(t, adds, "NET_ADMIN")
	assert.Contains(t, adds, "SYS_TIME")

	drops := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyDropCapability)
	assert.Contains(t, drops, "ALL")
}

func TestConvert_SecurityContext_ReadOnly(t *testing.T) {
	pod := minimalPod("p", "nginx")
	yes := true
	pod.Spec.Containers[0].SecurityContext = &v1.SecurityContext{ReadOnlyRootFilesystem: &yes}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	val, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyReadOnly)
	assert.True(t, ok)
	assert.Equal(t, "true", val)
}

func TestConvert_SecurityContext_NoNewPrivileges(t *testing.T) {
	pod := minimalPod("p", "nginx")
	no := false
	pod.Spec.Containers[0].SecurityContext = &v1.SecurityContext{AllowPrivilegeEscalation: &no}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	val, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyNoNewPrivileges)
	assert.True(t, ok)
	assert.Equal(t, "true", val)
}

func TestConvert_SecurityContext_UserGroup(t *testing.T) {
	pod := minimalPod("p", "nginx")
	uid, gid := int64(1000), int64(2000)
	pod.Spec.Containers[0].SecurityContext = &v1.SecurityContext{
		RunAsUser:  &uid,
		RunAsGroup: &gid,
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	val, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyUser)
	assert.True(t, ok)
	assert.Equal(t, "1000:2000", val)
}

func TestConvert_SecurityContext_SELinux(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].SecurityContext = &v1.SecurityContext{
		SELinuxOptions: &v1.SELinuxOptions{
			User:  "u",
			Role:  "r",
			Type:  "container_t",
			Level: "s0",
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")

	args := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyPodmanArgs)
	assert.Contains(t, args, "--security-opt label=user:u")
	assert.Contains(t, args, "--security-opt label=role:r")

	typ, _ := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeySecurityLabelType)
	assert.Equal(t, "container_t", typ)
	lvl, _ := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeySecurityLabelLevel)
	assert.Equal(t, "s0", lvl)
}

func TestConvert_SecurityContext_Seccomp(t *testing.T) {
	localhostPath := "profiles/myprofile.json"
	tests := []struct {
		name    string
		profile v1.SeccompProfile
		want    string
	}{
		{"localhost", v1.SeccompProfile{Type: v1.SeccompProfileTypeLocalhost, LocalhostProfile: &localhostPath}, localhostPath},
		{"runtime-default", v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault}, ""},
		{"unconfined", v1.SeccompProfile{Type: v1.SeccompProfileTypeUnconfined}, "unconfined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := minimalPod("p", "nginx")
			profile := tt.profile
			pod.Spec.Containers[0].SecurityContext = &v1.SecurityContext{SeccompProfile: &profile}
			files, err := Convert(pod, Options{})
			require.NoError(t, err)
			cu := containerUnit(files, "p", "app")
			val, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeySeccompProfile)
			assert.True(t, ok)
			assert.Equal(t, tt.want, val)
		})
	}
}

func TestConvert_ResourceLimits(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].Resources = v1.ResourceRequirements{
		Limits: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("512Mi"),
			v1.ResourceCPU:    resource.MustParse("500m"),
		},
		Requests: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	require.NotNil(t, cu)

	mem, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyMemory)
	assert.True(t, ok)
	assert.Equal(t, fmt.Sprintf("%d", int64(512*1024*1024)), mem, "memory must be raw bytes")

	args := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyPodmanArgs)
	hasCPUQuota := false
	hasMemReservation := false
	for _, a := range args {
		if strings.HasPrefix(a, "--cpu-quota=") {
			hasCPUQuota = true
		}
		if strings.HasPrefix(a, "--memory-reservation=") {
			hasMemReservation = true
		}
	}
	assert.True(t, hasCPUQuota, "cpu-quota PodmanArgs expected")
	assert.True(t, hasMemReservation, "memory-reservation PodmanArgs expected")
}

func TestConvert_MemoryDirectiveIsMemory_NotMemoryLimit(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].Resources = v1.ResourceRequirements{
		Limits: v1.ResourceList{v1.ResourceMemory: resource.MustParse("1Gi")},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	content, err := cu.Unit.ToString()
	require.NoError(t, err)
	assert.Contains(t, content, "Memory=")
	assert.NotContains(t, content, "MemoryLimit=")
}

func TestConvert_VolumeMount_PVC(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Volumes = []v1.Volume{
		{Name: "data", VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: "my-pvc"},
		}},
	}
	pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
		{Name: "data", MountPath: "/data"},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	volFile := findFile(files, "p-data.volume")
	require.NotNil(t, volFile, "expected p-data.volume in output")

	cu := containerUnit(files, "p", "app")
	vols := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyVolume)
	assert.True(t, func() bool {
		for _, v := range vols {
			if strings.HasPrefix(v, "p-data.volume:/data") {
				return true
			}
		}
		return false
	}(), "expected p-data.volume:/data in container Volume=")
}

func TestConvert_VolumeMount_PVC_SubPath(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Volumes = []v1.Volume{
		{Name: "state", VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: "vm-state"},
		}},
	}
	pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
		{Name: "state", MountPath: "/data/nvram", SubPath: "nvram"},
		{Name: "state", MountPath: "/data/swtpm", SubPath: "swtpm"},
		{Name: "state", MountPath: "/data/swtpm-localca", SubPath: "swtpm-localca", ReadOnly: true},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	cu := containerUnit(files, "p", "app")

	// SubPath mounts must use Mount= (--mount type=volume,subpath=...) because
	// Podman's -v syntax does not support the subpath= option.
	mounts := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyMount)
	assert.Contains(t, mounts, "type=volume,source=p-state.volume,destination=/data/nvram,subpath=nvram")
	assert.Contains(t, mounts, "type=volume,source=p-state.volume,destination=/data/swtpm,subpath=swtpm")
	assert.Contains(t, mounts, "type=volume,source=p-state.volume,destination=/data/swtpm-localca,subpath=swtpm-localca,ro")

	// None of the SubPath mounts should appear in Volume= lines.
	vols := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyVolume)
	for _, v := range vols {
		assert.NotContains(t, v, "subpath", "SubPath mount must not appear in Volume= line")
	}

	// The companion .volume unit must still be generated.
	volFile := findFile(files, "p-state.volume")
	require.NotNil(t, volFile, "expected p-state.volume in output")
}

func TestConvert_VolumeMount_HostPath(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Volumes = []v1.Volume{
		{Name: "d", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/host/data"}}},
	}
	pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{{Name: "d", MountPath: "/data"}}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	vols := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyVolume)
	assert.True(t, func() bool {
		for _, v := range vols {
			if strings.HasPrefix(v, "/host/data:/data:z") {
				return true
			}
		}
		return false
	}(), "expected /host/data:/data:z in Volume=")
}

func TestConvert_VolumeMount_HostPath_SysFs(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Volumes = []v1.Volume{
		{Name: "sys", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/sys/fs/cgroup"}}},
	}
	pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{{Name: "sys", MountPath: "/sys/fs/cgroup"}}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	content, err := cu.Unit.ToString()
	require.NoError(t, err)
	assert.NotContains(t, content, ":z")
}

func TestConvert_VolumeMount_EmptyDir_DefaultIsNamedVolume(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Volumes = []v1.Volume{
		{Name: "tmp", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
	}
	pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{{Name: "tmp", MountPath: "/tmp/data"}}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	content, err := cu.Unit.ToString()
	require.NoError(t, err)
	assert.NotContains(t, content, "tmpfs", "emptyDir default medium must not use tmpfs")
	vols := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyVolume)
	assert.True(t, func() bool {
		for _, v := range vols {
			if strings.Contains(v, "p-tmp-empty") {
				return true
			}
		}
		return false
	}(), "emptyDir should produce named anonymous volume p-tmp-empty")
}

func TestConvert_VolumeMount_EmptyDir_Memory(t *testing.T) {
	// Memory-medium emptyDir must generate a SHARED named volume backed by
	// tmpfs, not a per-container Mount=type=tmpfs.  Using a named volume
	// ensures all containers in the pod mount the same tmpfs instance,
	// matching Kubernetes emptyDir pod-scoped sharing semantics.
	pod := minimalPod("p", "nginx")
	pod.Spec.Volumes = []v1.Volume{
		{Name: "mem", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{Medium: v1.StorageMediumMemory}}},
	}
	pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{{Name: "mem", MountPath: "/mem"}}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	// A companion .volume unit with tmpfs options must be generated.
	volFile := findFile(files, "p-mem-empty.volume")
	require.NotNil(t, volFile, "expected p-mem-empty.volume for Memory-medium emptyDir")
	volContent, err := volFile.Unit.ToString()
	require.NoError(t, err)
	assert.Contains(t, volContent, "Type=tmpfs", "volume unit must declare tmpfs type")

	// The container must use Volume= referencing the named .volume unit, not Mount=type=tmpfs.
	cu := containerUnit(files, "p", "app")
	vols := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyVolume)
	hasNamedVol := false
	for _, v := range vols {
		if strings.HasPrefix(v, "p-mem-empty.volume:/mem") {
			hasNamedVol = true
			break
		}
	}
	assert.True(t, hasNamedVol, "container must mount p-mem-empty.volume named volume at /mem")

	// No per-container Mount=type=tmpfs directive.
	mounts := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyMount)
	for _, m := range mounts {
		assert.NotContains(t, m, "type=tmpfs", "per-container tmpfs mount must not be emitted; use named volume instead")
	}
}

func TestConvert_EmptyDir_Memory_SharedAcrossContainers(t *testing.T) {
	// Two containers that both mount the same Memory-medium emptyDir volume
	// must reference the same named volume so they see each other's writes —
	// matching Kubernetes pod-scoped emptyDir sharing semantics.
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{Name: "shared", VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{Medium: v1.StorageMediumMemory},
				}},
			},
			Containers: []v1.Container{
				{
					Name:         "main",
					Image:        "nginx",
					VolumeMounts: []v1.VolumeMount{{Name: "shared", MountPath: "/run/shared"}},
				},
				{
					Name:         "sidecar",
					Image:        "busybox",
					VolumeMounts: []v1.VolumeMount{{Name: "shared", MountPath: "/run/shared"}},
				},
			},
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	// Exactly one .volume unit for the shared emptyDir.
	var volFiles []*GeneratedFile
	for _, f := range files {
		if strings.HasSuffix(f.Name, "-shared-empty.volume") {
			volFiles = append(volFiles, f)
		}
	}
	assert.Len(t, volFiles, 1, "exactly one .volume unit for the shared emptyDir")

	volName := "p-shared-empty"

	// Both containers reference the same named .volume unit at the same mount path.
	for _, ctrName := range []string{"main", "sidecar"} {
		cu := containerUnit(files, "p", ctrName)
		require.NotNil(t, cu, "missing container unit for %s", ctrName)
		vols := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyVolume)
		hasVol := false
		for _, v := range vols {
			if strings.HasPrefix(v, volName+".volume:") {
				hasVol = true
				break
			}
		}
		assert.True(t, hasVol, "container %s must mount named volume %s", ctrName, volName+".volume")
	}
}

func TestConvert_VolumeMount_MountPropagation(t *testing.T) {
	hostToContainer := v1.MountPropagationHostToContainer
	bidirectional := v1.MountPropagationBidirectional
	tests := []struct {
		name     string
		prop     *v1.MountPropagationMode
		wantOpts string
	}{
		{"HostToContainer", &hostToContainer, "rslave"},
		{"Bidirectional", &bidirectional, "rshared"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := minimalPod("p", "nginx")
			pod.Spec.Volumes = []v1.Volume{
				{Name: "d", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/host"}}},
			}
			pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
				{Name: "d", MountPath: "/mnt", MountPropagation: tt.prop},
			}
			files, err := Convert(pod, Options{})
			require.NoError(t, err)
			cu := containerUnit(files, "p", "app")
			vols := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyVolume)
			assert.True(t, func() bool {
				for _, v := range vols {
					if strings.Contains(v, tt.wantOpts) {
						return true
					}
				}
				return false
			}(), "expected %s in volume options", tt.wantOpts)
		})
	}
}

func TestConvert_LivenessProbe_Exec(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].LivenessProbe = &v1.Probe{
		Handler:          v1.Handler{Exec: &v1.ExecAction{Command: []string{"cat", "/tmp/healthy"}}},
		PeriodSeconds:    5,
		TimeoutSeconds:   2,
		FailureThreshold: 3,
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	cmd, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyHealthCmd)
	assert.True(t, ok)
	assert.Contains(t, cmd, "cat")
	interval, _ := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyHealthInterval)
	assert.Equal(t, "5s", interval)
}

func TestConvert_LivenessProbe_HTTPGet(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].LivenessProbe = &v1.Probe{
		Handler: v1.Handler{HTTPGet: &v1.HTTPGetAction{Port: intOrString(8080), Path: "/healthz"}},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	cmd, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyHealthCmd)
	assert.True(t, ok)
	assert.Contains(t, cmd, "curl")
	assert.Contains(t, cmd, "8080")
}

func TestConvert_LivenessProbe_TCPSocket(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].LivenessProbe = &v1.Probe{
		Handler: v1.Handler{TCPSocket: &v1.TCPSocketAction{Port: intOrString(3306)}},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	cmd, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyHealthCmd)
	assert.True(t, ok)
	assert.Contains(t, cmd, "/dev/tcp/localhost/3306")
}

func TestConvert_StartupProbe(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].StartupProbe = &v1.Probe{
		Handler:          v1.Handler{Exec: &v1.ExecAction{Command: []string{"test", "-f", "/ready"}}},
		FailureThreshold: 30,
		PeriodSeconds:    10,
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	cmd, ok := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyHealthStartupCmd)
	assert.True(t, ok)
	assert.Contains(t, cmd, "/ready")
	retries, _ := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyHealthStartupRetries)
	assert.Equal(t, "30", retries)
}

func TestConvert_ExecCommand_ShellScript(t *testing.T) {
	pod := minimalPod("p", "busybox")
	pod.Spec.Containers[0].Command = []string{"/bin/sh", "-c"}
	pod.Spec.Containers[0].Args = []string{"echo hello && sleep 1"}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	scriptFile := findFile(files, "p-app.sh")
	require.NotNil(t, scriptFile)
	assert.Equal(t, "echo hello && sleep 1", scriptFile.Content)

	cu := containerUnit(files, "p", "app")
	entrypoint, _ := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyEntrypoint)
	assert.Equal(t, "/bin/sh", entrypoint)
	exec, _ := cu.Unit.Lookup(quadlet.ContainerGroup, quadlet.KeyExec)
	assert.Equal(t, "/init.sh", exec)

	vols := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyVolume)
	assert.Contains(t, vols, "./p-app.sh:/init.sh:ro,z",
		"script must be bind-mounted via relative path so units are self-contained")
}

func TestConvert_EnvPlain(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].Env = []v1.EnvVar{{Name: "FOO", Value: "bar"}}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	envs := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyEnvironment)
	assert.True(t, func() bool {
		for _, e := range envs {
			if strings.Contains(e, "FOO") {
				return true
			}
		}
		return false
	}())
}

func TestConvert_EnvConfigMapKeyRef(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].Env = []v1.EnvVar{
		{Name: "DB_HOST", ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				LocalObjectReference: v1.LocalObjectReference{Name: "mycm"},
				Key:                  "host",
			},
		}},
	}
	opts := Options{ConfigMaps: map[string]v1.ConfigMap{
		"mycm": {Data: map[string]string{"host": "postgres"}},
	}}
	files, err := Convert(pod, opts)
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	envs := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyEnvironment)
	assert.True(t, func() bool {
		for _, e := range envs {
			if strings.Contains(e, "DB_HOST") && strings.Contains(e, "postgres") {
				return true
			}
		}
		return false
	}())
}

func TestConvert_EnvConfigMapKeyRef_Missing_Required(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].Env = []v1.EnvVar{
		{Name: "KEY", ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				LocalObjectReference: v1.LocalObjectReference{Name: "missing"},
				Key:                  "key",
			},
		}},
	}
	_, err := Convert(pod, Options{})
	assert.Error(t, err)
}

func TestConvert_EnvFieldRef(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Namespace = "mynamespace"
	pod.Spec.Containers[0].Env = []v1.EnvVar{
		{Name: "POD_NS", ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
		}},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	envs := cu.Unit.LookupAll(quadlet.ContainerGroup, quadlet.KeyEnvironment)
	assert.True(t, func() bool {
		for _, e := range envs {
			if strings.Contains(e, "POD_NS") && strings.Contains(e, "mynamespace") {
				return true
			}
		}
		return false
	}())
}

func TestConvert_PortsComment(t *testing.T) {
	pod := minimalPod("p", "nginx")
	pod.Spec.Containers[0].Ports = []v1.ContainerPort{
		{ContainerPort: 8080, Protocol: v1.ProtocolTCP},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)
	cu := containerUnit(files, "p", "app")
	content, err := cu.Unit.ToString()
	require.NoError(t, err)
	assert.Contains(t, content, "8080")
	assert.NotContains(t, content, "PublishPort=8080")
}

func TestConvert_OutputOrdering(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				{Name: "init", Image: "busybox"},
			},
			Containers: []v1.Container{
				{Name: "app", Image: "nginx"},
				{Name: "sidecar", Image: "envoy"},
			},
			Volumes: []v1.Volume{
				{Name: "data", VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: "pvc"},
				}},
			},
		},
	}
	pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{{Name: "data", MountPath: "/data"}}

	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}

	volIdx := indexOf(names, "p-data.volume")
	podIdx := indexOf(names, "p.pod")
	initIdx := indexOf(names, "p-init.container")
	appIdx := indexOf(names, "p-app.container")
	sideIdx := indexOf(names, "p-sidecar.container")

	assert.Less(t, volIdx, podIdx, ".volume must come before .pod")
	assert.Less(t, podIdx, initIdx, ".pod must come before init container")
	assert.Less(t, initIdx, appIdx, "init container must come before first regular container")
	assert.Less(t, appIdx, sideIdx, "first container before second container")
}

func TestConvert_OutputOrdering_EmptyDirVolumeBeforePod(t *testing.T) {
	// Memory-medium emptyDir generates a .volume unit.  It must be ordered
	// before the .pod unit so that Quadlet can resolve the volume reference
	// when it processes the pod and container units.
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{Name: "runtime", VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{Medium: v1.StorageMediumMemory},
				}},
			},
			Containers: []v1.Container{
				{
					Name:         "app",
					Image:        "nginx",
					VolumeMounts: []v1.VolumeMount{{Name: "runtime", MountPath: "/run"}},
				},
			},
		},
	}
	files, err := Convert(pod, Options{})
	require.NoError(t, err)

	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}

	volIdx := indexOf(names, "p-runtime-empty.volume")
	podIdx := indexOf(names, "p.pod")

	assert.GreaterOrEqual(t, volIdx, 0, "p-runtime-empty.volume must be present")
	assert.Less(t, volIdx, podIdx, "emptyDir .volume unit must come before .pod unit")
}
