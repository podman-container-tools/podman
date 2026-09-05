package quadlet

import (
	"fmt"
	"strings"

	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	"go.podman.io/podman/v6/pkg/systemd/quadlet"
)

// Convert translates a Kubernetes Pod spec into a set of Quadlet unit files.
//
// Output ordering: .volume files first, then the .pod unit, then init
// .container files in pod order (with companion .sh files immediately
// following each), then regular .container files, then .env files.
func Convert(pod *v1.Pod, opts Options) ([]*GeneratedFile, error) {
	if opts.NamePrefix == "" {
		opts.NamePrefix = pod.Name
	}

	prefix := opts.NamePrefix

	// podSvcName is the systemd service name Quadlet derives from the .pod unit:
	// "<prefix>.pod" → "<prefix>-pod.service".
	podSvcName := fmt.Sprintf("%s-pod.service", prefix)

	// firstContainerSvcName is used as WantedBy= target for init containers so
	// that enabling the pod also activates them in the right order.
	var firstContainerSvcName string
	if len(pod.Spec.Containers) > 0 {
		firstContainerSvcName = fmt.Sprintf("%s-%s.service", prefix, pod.Spec.Containers[0].Name)
	}

	var output []*GeneratedFile
	var envFiles []*GeneratedFile

	// emptyDirVolNames collects the generated volume names for all emptyDir
	// volumes so the pod unit can mount them on the infra container (Step 2).
	var emptyDirVolNames []string

	// Step 1 — .volume units for PVC volumes and emptyDir volumes.
	for _, v := range pod.Spec.Volumes {
		switch {
		case v.PersistentVolumeClaim != nil:
			volUnit := parser.NewUnitFile()
			volUnit.Filename = fmt.Sprintf("%s-%s.volume", prefix, v.Name)
			volUnit.Set(quadlet.UnitGroup, "Description",
				fmt.Sprintf("Volume %s for pod %s", v.Name, pod.Name))
			volUnit.AddComment(quadlet.VolumeGroup, "Local named volume, no driver required.")
			output = append(output, &GeneratedFile{Name: volUnit.Filename, Unit: volUnit})

		case v.EmptyDir != nil:
			// All emptyDir volumes (regardless of medium) map to a tmpfs-backed
			// named volume. In Kubernetes, emptyDir is always ephemeral — data is
			// cleared on every pod restart. Using a named volume (rather than a
			// per-container Mount=type=tmpfs) ensures all containers in the pod
			// share the same tmpfs instance, matching Kubernetes pod-scoped semantics.
			// A tmpfs-backed named volume is naturally volatile: data lives only
			// as long as the volume is mounted, so each pod start gets clean state.
			volName := fmt.Sprintf("%s-%s-empty", prefix, v.Name)
			volUnit := parser.NewUnitFile()
			volUnit.Filename = volName + ".volume"
			volUnit.Set(quadlet.UnitGroup, "Description",
				fmt.Sprintf("Shared tmpfs volume %s for pod %s", v.Name, pod.Name))
			volUnit.Set(quadlet.VolumeGroup, "Driver", "local")
			volUnit.Set(quadlet.VolumeGroup, "Device", "tmpfs")
			volUnit.Set(quadlet.VolumeGroup, "Type", "tmpfs")
			volUnit.Set(quadlet.VolumeGroup, "Options", "nodev,mode=0777")
			output = append(output, &GeneratedFile{Name: volUnit.Filename, Unit: volUnit})
			emptyDirVolNames = append(emptyDirVolNames, volName)
		}
	}

	// Step 2 — .pod unit.
	podUnit := buildPodUnit(pod, opts, emptyDirVolNames)
	output = append(output, &GeneratedFile{Name: podUnit.Filename, Unit: podUnit})

	// Step 3 — init container units.
	var initServiceNames []string
	for _, c := range pod.Spec.InitContainers {
		files, err := buildContainerUnit(c, pod, opts)
		if err != nil {
			return nil, fmt.Errorf("init container %q: %w", c.Name, err)
		}
		unitFile := files[0].Unit

		// Join the pod.
		unitFile.Set(quadlet.ContainerGroup, quadlet.KeyPod,
			fmt.Sprintf("%s.pod", prefix))

		// After= all prior init services (pod dependency is implicit via Pod=).
		for _, svc := range initServiceNames {
			unitFile.Add(quadlet.UnitGroup, "After", svc)
			unitFile.Add(quadlet.UnitGroup, "Requires", svc)
		}

		// Oneshot — runs to completion.
		unitFile.Set(quadlet.ServiceGroup, "Type", "oneshot")
		unitFile.Set(quadlet.ServiceGroup, "RemainAfterExit", "yes")

		// WantedBy= the first regular container: activating the pod service
		// activates the first container service, which pulls in these init services.
		if firstContainerSvcName != "" {
			unitFile.Add(quadlet.InstallGroup, "WantedBy", firstContainerSvcName)
		}

		svcName := fmt.Sprintf("%s-%s.service", prefix, c.Name)
		initServiceNames = append(initServiceNames, svcName)

		collectFiles(files, &output, &envFiles)
	}

	// Step 4 — regular container units.
	for i, c := range pod.Spec.Containers {
		files, err := buildContainerUnit(c, pod, opts)
		if err != nil {
			return nil, fmt.Errorf("container %q: %w", c.Name, err)
		}
		unitFile := files[0].Unit

		// Join the pod.
		unitFile.Set(quadlet.ContainerGroup, quadlet.KeyPod,
			fmt.Sprintf("%s.pod", prefix))

		// First container waits for all init containers.
		if i == 0 {
			for _, svc := range initServiceNames {
				unitFile.Add(quadlet.UnitGroup, "After", svc)
				unitFile.Add(quadlet.UnitGroup, "Requires", svc)
			}
		}

		// [Service] restart policy.
		applyRestartPolicy(unitFile, pod.Spec.RestartPolicy)

		// [Install] — activated when the pod service starts.
		unitFile.Add(quadlet.InstallGroup, "WantedBy", podSvcName)

		collectFiles(files, &output, &envFiles)
	}

	// Env files go last.
	output = append(output, envFiles...)
	return output, nil
}

// buildPodUnit constructs the .pod Quadlet unit for the pod.
// Pod-level networking, DNS, hostname, and namespace fields live here so that
// all containers share them through the Podman infra container — the same way
// Kubernetes uses a pause container to hold shared namespaces.
//
// emptyDirVolNames lists the generated volume names for all emptyDir volumes.
// Each is mounted on the infra container at /.emptydir/<name> to keep the
// underlying tmpfs alive as long as the pod runs. This matches Kubernetes
// emptyDir semantics: data survives individual container crashes (infra is
// still running) but is cleared when the pod itself is terminated.
func buildPodUnit(pod *v1.Pod, opts Options, emptyDirVolNames []string) *parser.UnitFile {
	prefix := opts.NamePrefix
	u := parser.NewUnitFile()
	u.Filename = fmt.Sprintf("%s.pod", prefix)
	u.Set(quadlet.UnitGroup, "Description",
		fmt.Sprintf("Pod %s", pod.Name))

	// Explicit pod name so all containers join the right pod.
	u.Set(quadlet.PodGroup, quadlet.KeyPodName, prefix)

	// Keep the infra container alive regardless of workload container state.
	// This matches Kubernetes pause-container semantics: the pod's network
	// namespace is owned by the infra container and must outlive any individual
	// workload, including init containers that complete and are removed.
	// Without this, Podman's default "stop" exit policy tears down the infra
	// when the last non-infra container exits — creating a race where init
	// containers complete before main containers start, leaving the pod empty.
	u.Set(quadlet.PodGroup, quadlet.KeyExitPolicy, "continue")

	// Network.
	if pod.Spec.HostNetwork {
		u.Set(quadlet.PodGroup, quadlet.KeyNetwork, "host")
	} else if opts.Network != "" {
		u.Set(quadlet.PodGroup, quadlet.KeyNetwork, opts.Network)
	}

	// Hostname.
	if pod.Spec.Hostname != "" {
		u.Set(quadlet.PodGroup, quadlet.KeyHostName, pod.Spec.Hostname)
	}

	// Host aliases.
	for _, ha := range pod.Spec.HostAliases {
		for _, h := range ha.Hostnames {
			u.Add(quadlet.PodGroup, quadlet.KeyAddHost,
				fmt.Sprintf("%s:%s", h, ha.IP))
		}
	}

	// DNS config.
	if pod.Spec.DNSConfig != nil {
		for _, ns := range pod.Spec.DNSConfig.Nameservers {
			u.Add(quadlet.PodGroup, quadlet.KeyDNS, ns)
		}
		for _, s := range pod.Spec.DNSConfig.Searches {
			u.Add(quadlet.PodGroup, quadlet.KeyDNSSearch, s)
		}
		for _, o := range pod.Spec.DNSConfig.Options {
			opt := o.Name
			if o.Value != nil {
				opt += "=" + *o.Value
			}
			u.Add(quadlet.PodGroup, quadlet.KeyDNSOption, opt)
		}
	}

	// Host namespaces.
	if pod.Spec.HostPID {
		u.Add(quadlet.PodGroup, quadlet.KeyPodmanArgs, "--pid=host")
	}
	if pod.Spec.HostIPC {
		u.Add(quadlet.PodGroup, quadlet.KeyPodmanArgs, "--ipc=host")
	}

	// Process namespace sharing between containers.
	if pod.Spec.ShareProcessNamespace != nil && *pod.Spec.ShareProcessNamespace {
		u.Add(quadlet.PodGroup, quadlet.KeyPodmanArgs, "--share pid")
	}

	// Pod-level sysctls.
	if pod.Spec.SecurityContext != nil {
		for _, s := range pod.Spec.SecurityContext.Sysctls {
			u.Add(quadlet.PodGroup, quadlet.KeyPodmanArgs,
				fmt.Sprintf("--sysctl %s=%s", s.Name, s.Value))
		}
	}

	// Termination grace period.
	if pod.Spec.TerminationGracePeriodSeconds != nil {
		u.Set(quadlet.PodGroup, quadlet.KeyStopTimeout,
			fmt.Sprintf("%d", *pod.Spec.TerminationGracePeriodSeconds))
	}

	// Mount each emptyDir volume on the infra container.
	// The infra container keeps the tmpfs live for the full pod lifetime, so
	// individual container crashes do not lose emptyDir data — matching the
	// Kubernetes guarantee that emptyDir survives container restarts but not
	// pod termination. Containers mount the same named volume via their own
	// Volume= lines; this entry merely anchors the tmpfs to the pod lifecycle.
	// The ".volume" suffix ensures Quadlet uses the managed tmpfs-backed volume.
	for _, volName := range emptyDirVolNames {
		u.Add(quadlet.PodGroup, quadlet.KeyVolume,
			fmt.Sprintf("%s.volume:/.emptydir/%s", volName, volName))
	}

	// [Install] — the pod service is the root systemd unit.
	u.Add(quadlet.InstallGroup, "WantedBy", "default.target")

	return u
}

// collectFiles splits files from buildContainerUnit into unit files (output)
// and env files (envFiles). Script (.sh) files follow their container unit
// immediately; .env files are deferred to the end.
func collectFiles(files []*GeneratedFile, output, envFiles *[]*GeneratedFile) {
	*output = append(*output, files[0])
	for _, f := range files[1:] {
		if strings.HasSuffix(f.Name, ".env") {
			*envFiles = append(*envFiles, f)
		} else {
			*output = append(*output, f)
		}
	}
}

// applyRestartPolicy emits Restart= in [Service].
func applyRestartPolicy(unit *parser.UnitFile, policy v1.RestartPolicy) {
	switch policy {
	case v1.RestartPolicyAlways:
		unit.Set(quadlet.ServiceGroup, "Restart", "always")
	case v1.RestartPolicyOnFailure:
		unit.Set(quadlet.ServiceGroup, "Restart", "on-failure")
		// Never: omit Restart=
	}
}
