//go:build !remote && (linux || freebsd)

package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"os"

	metadata "github.com/checkpoint-restore/checkpointctl/lib"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod"
	ann "go.podman.io/podman/v6/pkg/annotations"
	"go.podman.io/podman/v6/pkg/checkpoint/crutils"
	"go.podman.io/podman/v6/pkg/criu"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgen/generate"
	"go.podman.io/podman/v6/pkg/specgenutil"
)

// Prefixing the checkpoint/restore related functions with 'cr'

func CRImportCheckpointTar(ctx context.Context, runtime *libpod.Runtime, restoreOptions entities.RestoreOptions) ([]*libpod.Container, error) {
	// First get the container definition from the
	// tarball to a temporary directory
	dir, err := os.MkdirTemp("", "checkpoint")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			logrus.Errorf("Could not recursively remove %s: %q", dir, err)
		}
	}()
	if err := crutils.CRImportCheckpointConfigOnly(dir, restoreOptions.Import); err != nil {
		return nil, err
	}
	return CRImportCheckpoint(ctx, runtime, restoreOptions, dir)
}

// CRImportCheckpoint it the function which imports the information
// from checkpoint tarball and re-creates the container from that information
func CRImportCheckpoint(ctx context.Context, runtime *libpod.Runtime, restoreOptions entities.RestoreOptions, dir string) ([]*libpod.Container, error) {
	// Load spec.dump from temporary directory
	dumpSpec := new(spec.Spec)
	if _, err := metadata.ReadJSONFile(dumpSpec, dir, metadata.SpecDumpFile); err != nil {
		return nil, err
	}

	// Load config.dump from temporary directory
	ctrConfig := new(libpod.ContainerConfig)
	if _, err := metadata.ReadJSONFile(ctrConfig, dir, metadata.ConfigDumpFile); err != nil {
		return nil, err
	}

	if ctrConfig.Pod != "" && restoreOptions.Pod == "" {
		return nil, errors.New("cannot restore pod container without --pod")
	}

	if ctrConfig.Pod == "" && restoreOptions.Pod != "" {
		return nil, errors.New("cannot restore non pod container into pod")
	}

	// This should not happen as checkpoints with these options are not exported.
	if len(ctrConfig.Dependencies) > 0 {
		return nil, errors.New("cannot import checkpoints of containers with dependencies")
	}

	// Volumes included in the checkpoint should not exist
	if !restoreOptions.IgnoreVolumes {
		for _, vol := range ctrConfig.NamedVolumes {
			exists, err := runtime.HasVolume(vol.Name)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, fmt.Errorf("volume with name %s already exists. Use --ignore-volumes to not restore content of volumes", vol.Name)
			}
		}
	}

	ctrID := ctrConfig.ID
	newName := false

	// Check if the restored container gets a new name
	if restoreOptions.Name != "" {
		ctrConfig.ID = ""
		ctrConfig.Name = restoreOptions.Name
		newName = true
	}

	if restoreOptions.Pod != "" {
		// Restoring into a Pod requires much newer versions of CRIU
		if err := criu.CheckForCriu(criu.PodCriuVersion); err != nil {
			return nil, fmt.Errorf("restoring containers into pod: %w", err)
		}
		// The runtime also has to support it
		if !crutils.CRRuntimeSupportsPodCheckpointRestore(runtime.GetOCIRuntimePath()) {
			return nil, fmt.Errorf("runtime %s does not support pod restore", runtime.GetOCIRuntimePath())
		}

		// According to podman pod create a pod can share the following namespaces:
		// cgroup, ipc, net, pid, uts
		// Let's make sure we are restoring into a pod with the same shared namespaces.
		pod, err := runtime.LookupPod(restoreOptions.Pod)
		if err != nil {
			return nil, fmt.Errorf("pod %q cannot be retrieved: %w", ctrConfig.Pod, err)
		}

		// Restoring into an existing Pod
		ctrConfig.Pod = pod.ID()

		infraContainer, err := pod.InfraContainer()
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve infra container from pod %q: %w", ctrConfig.Pod, err)
		}

		// If a namespace was shared (!= "") it needs to be set to the new infrastructure container.
		// If the infrastructure container does not share the same namespaces as the to be restored
		// container we abort.
		if ctrConfig.IPCNsCtr != "" {
			if !pod.SharesIPC() {
				return nil, fmt.Errorf("pod %s does not share the IPC namespace", ctrConfig.Pod)
			}
			ctrConfig.IPCNsCtr = infraContainer.ID()
		}

		if ctrConfig.NetNsCtr != "" {
			if !pod.SharesNet() {
				return nil, fmt.Errorf("pod %s does not share the network namespace", ctrConfig.Pod)
			}
			ctrConfig.NetNsCtr = infraContainer.ID()
			for net, opts := range ctrConfig.Networks {
				opts.StaticIPs = nil
				opts.StaticMAC = nil
				ctrConfig.Networks[net] = opts
			}
		}

		if ctrConfig.PIDNsCtr != "" {
			if !pod.SharesPID() {
				return nil, fmt.Errorf("pod %s does not share the PID namespace", ctrConfig.Pod)
			}
			ctrConfig.PIDNsCtr = infraContainer.ID()
		}

		if ctrConfig.UTSNsCtr != "" {
			if !pod.SharesUTS() {
				return nil, fmt.Errorf("pod %s does not share the UTS namespace", ctrConfig.Pod)
			}
			ctrConfig.UTSNsCtr = infraContainer.ID()
		}

		if ctrConfig.CgroupNsCtr != "" {
			if !pod.SharesCgroup() {
				return nil, fmt.Errorf("pod %s does not share the cgroup namespace", ctrConfig.Pod)
			}
			ctrConfig.CgroupNsCtr = infraContainer.ID()
		}

		// Change SELinux labels to infrastructure container labels
		ctrConfig.MountLabel = infraContainer.MountLabel()
		ctrConfig.ProcessLabel = infraContainer.ProcessLabel()

		// Fix parent cgroup
		cgroupPath, err := pod.CgroupPath()
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve cgroup path from pod %q: %w", ctrConfig.Pod, err)
		}
		ctrConfig.CgroupParent = cgroupPath

		oldPodID := dumpSpec.Annotations[ann.SandboxID]
		// Fix up SandboxID in the annotations
		dumpSpec.Annotations[ann.SandboxID] = ctrConfig.Pod
		// Fix up CreateCommand
		for i, c := range ctrConfig.CreateCommand {
			if c == oldPodID {
				ctrConfig.CreateCommand[i] = ctrConfig.Pod
			}
		}
	}

	if len(restoreOptions.PublishPorts) > 0 {
		pubPorts, err := specgenutil.CreatePortBindings(restoreOptions.PublishPorts)
		if err != nil {
			return nil, err
		}

		ports, err := generate.ParsePortMapping(pubPorts, nil)
		if err != nil {
			return nil, err
		}
		ctrConfig.PortMappings = ports
	}

	pullOptions := &libimage.PullOptions{}
	pullOptions.Writer = os.Stderr
	if _, err := runtime.LibimageRuntime().Pull(ctx, ctrConfig.RootfsImageName, config.PullPolicyMissing, pullOptions); err != nil {
		return nil, err
	}

	// Now create a new container from the just loaded information
	container, err := runtime.RestoreContainer(ctx, dumpSpec, ctrConfig)
	if err != nil {
		return nil, err
	}

	if container == nil {
		return nil, nil
	}

	containerConfig := container.Config()
	ctrName := ctrConfig.Name
	if containerConfig.Name != ctrName {
		return nil, fmt.Errorf("name of restored container (%s) does not match requested name (%s)", containerConfig.Name, ctrName)
	}

	if !newName {
		// Only check ID for a restore with the same name.
		// Using -n to request a new name for the restored container, will also create a new ID
		if containerConfig.ID != ctrID {
			return nil, fmt.Errorf("ID of restored container (%s) does not match requested ID (%s)", containerConfig.ID, ctrID)
		}
	}

	return []*libpod.Container{container}, nil
}
