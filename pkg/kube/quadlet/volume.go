package quadlet

import (
	"fmt"
	"strings"

	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	"go.podman.io/podman/v6/pkg/systemd/quadlet"
)

func applyMount(unit *parser.UnitFile, m v1.VolumeMount, src v1.VolumeSource, prefix string) error {
	switch {
	case src.HostPath != nil:
		return applyHostPathMount(unit, m, src.HostPath)
	case src.PersistentVolumeClaim != nil:
		return applyPVCMount(unit, m, prefix)
	case src.EmptyDir != nil:
		return applyEmptyDirMount(unit, m, m.Name, prefix)
	case src.Image != nil:
		return applyImageMount(unit, m, src.Image)
	case src.ConfigMap != nil, src.Secret != nil:
		// ConfigMap/Secret volumes are handled by applyVolumeMountsWithConfigSecrets.
		return nil
	default:
		// Unsupported volume source — silently skip (consistent with kube play).
		return nil
	}
}

// mountOptions builds the -v options string from a VolumeMount.
// SubPath is intentionally excluded: PVC mounts with SubPath use Mount=
// (--mount type=volume,subpath=...) instead of Volume= (-v), because
// Podman's -v syntax does not support the subpath= option.
func mountOptions(m v1.VolumeMount) string {
	var opts []string
	if m.ReadOnly {
		opts = append(opts, "ro")
	}
	if m.MountPropagation != nil {
		switch *m.MountPropagation {
		case v1.MountPropagationHostToContainer:
			opts = append(opts, "rslave")
		case v1.MountPropagationBidirectional:
			opts = append(opts, "rshared")
		case v1.MountPropagationNone:
			opts = append(opts, "private")
		}
	}
	return strings.Join(opts, ",")
}

func applyHostPathMount(unit *parser.UnitFile, m v1.VolumeMount, hp *v1.HostPathVolumeSource) error {
	path := hp.Path
	opts := mountOptions(m)

	// Device types use AddDevice=, not Volume=.
	// Podman's --device only accepts r/w/m permission chars; b/c are Linux
	// device-type indicators that --device does not understand.
	if hp.Type != nil {
		switch *hp.Type {
		case v1.HostPathBlockDev, v1.HostPathCharDev:
			unit.Add(quadlet.ContainerGroup, quadlet.KeyAddDevice,
				fmt.Sprintf("%s:%s", path, m.MountPath))
			return nil
		}
	}

	// Paths directly under /dev/ use AddDevice=.
	if strings.HasPrefix(path, "/dev/") {
		unit.Add(quadlet.ContainerGroup, quadlet.KeyAddDevice, path)
		return nil
	}

	// Kernel virtual filesystems must not be relabeled.
	if strings.HasPrefix(path, "/sys/") || strings.HasPrefix(path, "/proc/") {
		vol := path + ":" + m.MountPath
		if m.ReadOnly {
			vol += ":ro"
		}
		unit.Add(quadlet.ContainerGroup, quadlet.KeyVolume, vol)
		return nil
	}

	// All other host paths get :z for SELinux relabeling.
	vol := path + ":" + m.MountPath + ":z"
	if opts != "" {
		vol += "," + opts
	}
	unit.Add(quadlet.ContainerGroup, quadlet.KeyVolume, vol)
	return nil
}

func applyPVCMount(unit *parser.UnitFile, m v1.VolumeMount, prefix string) error {
	volName := fmt.Sprintf("%s-%s.volume", prefix, m.Name)

	if m.SubPath != "" {
		// Podman's -v short syntax does not support subpath=; use --mount
		// type=volume instead. resolveContainerMountParams resolves the
		// .volume unit reference and adds the correct Requires=/After=
		// dependencies, identical to what Volume= would produce.
		mountStr := fmt.Sprintf("type=volume,source=%s,destination=%s,subpath=%s",
			volName, m.MountPath, m.SubPath)
		if m.ReadOnly {
			mountStr += ",ro"
		}
		unit.Add(quadlet.ContainerGroup, quadlet.KeyMount, mountStr)
		return nil
	}

	vol := volName + ":" + m.MountPath
	if opts := mountOptions(m); opts != "" {
		vol += ":" + opts
	}
	unit.Add(quadlet.ContainerGroup, quadlet.KeyVolume, vol)
	return nil
}

func applyEmptyDirMount(unit *parser.UnitFile, m v1.VolumeMount, volName, prefix string) error {
	// Always use a named volume so all containers in the pod share the same
	// underlying storage — matching Kubernetes emptyDir semantics where every
	// container that mounts the volume sees the same data.
	// A companion .volume unit with tmpfs options is generated in convert.go,
	// making the volume volatile (data clears on each pod start).
	// The ".volume" suffix on the reference tells Quadlet to use the managed
	// volume (systemd-<name>) rather than creating a new plain named volume.
	//
	// :z triggers a shared SELinux relabel of the volume at mount time,
	// changing the label from tmpfs_t to container_file_t. This allows
	// container_t processes to create UNIX sockets and execute binaries
	// inside these tmpfs volumes without disabling SELinux confinement.
	// On non-SELinux systems the option is silently ignored.
	anonVol := fmt.Sprintf("%s-%s-empty", prefix, volName)
	unit.Add(quadlet.ContainerGroup, quadlet.KeyVolume,
		fmt.Sprintf("%s.volume:%s:z", anonVol, m.MountPath))
	return nil
}

func applyImageMount(unit *parser.UnitFile, m v1.VolumeMount, img *v1.ImageVolumeSource) error {
	// OCI image volumes are read-only by default in Podman. Add ",rw" unless
	// the mount is explicitly marked read-only (e.g. a read-only container disk).
	mountStr := fmt.Sprintf("type=image,source=%s,dst=%s", img.Reference, m.MountPath)
	if !m.ReadOnly {
		mountStr += ",rw"
	}
	unit.Add(quadlet.ContainerGroup, quadlet.KeyMount, mountStr)
	return nil
}

// applyConfigMapVolume emits the Volume= bind-mount for a configMap volume source.
// Caller is responsible for writing the actual files to ConfigMapDir.
func applyConfigMapVolume(unit *parser.UnitFile, m v1.VolumeMount, cm *v1.ConfigMapVolumeSource, opts Options) error {
	if opts.ConfigMaps == nil {
		if cm.Optional != nil && *cm.Optional {
			return nil
		}
		return fmt.Errorf("configMap %q not provided in opts.ConfigMaps", cm.Name)
	}
	if _, ok := opts.ConfigMaps[cm.Name]; !ok {
		if cm.Optional != nil && *cm.Optional {
			return nil
		}
		return fmt.Errorf("configMap %q not found in opts.ConfigMaps", cm.Name)
	}
	dir := opts.ConfigMapDir + "/" + m.Name
	unit.Add(quadlet.ContainerGroup, quadlet.KeyVolume,
		fmt.Sprintf("%s:%s:ro,z", dir, m.MountPath))
	return nil
}

// applySecretVolume emits the Volume= bind-mount for a secret volume source.
func applySecretVolume(unit *parser.UnitFile, m v1.VolumeMount, sec *v1.SecretVolumeSource, opts Options) error {
	if opts.Secrets == nil {
		if sec.Optional != nil && *sec.Optional {
			return nil
		}
		return fmt.Errorf("secret %q not provided in opts.Secrets", sec.SecretName)
	}
	if _, ok := opts.Secrets[sec.SecretName]; !ok {
		if sec.Optional != nil && *sec.Optional {
			return nil
		}
		return fmt.Errorf("secret %q not found in opts.Secrets", sec.SecretName)
	}
	dir := opts.ConfigMapDir + "/" + m.Name
	unit.Add(quadlet.ContainerGroup, quadlet.KeyVolume,
		fmt.Sprintf("%s:%s:ro,z", dir, m.MountPath))
	return nil
}
