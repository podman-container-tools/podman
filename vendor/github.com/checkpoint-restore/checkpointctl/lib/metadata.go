// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

var errNotRegularFile = errors.New("not a regular file")

const (
	// container archive
	ConfigDumpFile             = "config.dump"
	SpecDumpFile               = "spec.dump"
	StatusDumpFile             = "status.dump"
	NetworkStatusFile          = "network.status"
	CheckpointDirectory        = "checkpoint"
	CheckpointVolumesDirectory = "volumes"
	DevShmCheckpointTar        = "devshm-checkpoint.tar"
	RootFsDiffTar              = "rootfs-diff.tar"
	DeletedFilesFile           = "deleted.files"
	DumpLogFile                = "dump.log"
	RestoreLogFile             = "restore.log"
	// pod archive
	PodOptionsFile = "pod.options"
	PodDumpFile    = "pod.dump"
	// containerd only
	StatusFile = "status"
	// CRIU Images
	PagesPrefix       = "pages-"
	AmdgpuPagesPrefix = "amdgpu-pages-"
)

// This is a reduced copy of what Podman uses to store checkpoint metadata
type ContainerConfig struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	RootfsImage     string    `json:"rootfsImage,omitempty"`
	RootfsImageRef  string    `json:"rootfsImageRef,omitempty"`
	RootfsImageName string    `json:"rootfsImageName,omitempty"`
	OCIRuntime      string    `json:"runtime,omitempty"`
	CreatedTime     time.Time `json:"createdTime"`
	CheckpointedAt  time.Time `json:"checkpointedTime"`
	RestoredAt      time.Time `json:"restoredTime"`
	Restored        bool      `json:"restored"`
}

type Spec struct {
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ContainerdStatus struct {
	CreatedAt  int64
	StartedAt  int64
	FinishedAt int64
	ExitCode   int32
	Pid        uint32
	Reason     string
	Message    string
}

// This structure is used by the KubernetesContainerCheckpointMetadata structure
type KubernetesCheckpoint struct {
	Archive   string `json:"archive,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// This structure is the basis for Kubernetes to track how many checkpoints
// for a certain container have been created.
type KubernetesContainerCheckpointMetadata struct {
	PodFullName   string                 `json:"podFullName,omitempty"`
	ContainerName string                 `json:"containerName,omitempty"`
	TotalSize     int64                  `json:"totalSize,omitempty"`
	Checkpoints   []KubernetesCheckpoint `json:"checkpoints"`
}

// CheckpointedPodOptions contains metadata about a checkpointed pod
type CheckpointedPodOptions struct {
	// Version is the version of the pod checkpoint format
	Version int `json:"version"`
	// Containers is a map with the short container name as key and the full name as value
	Containers map[string]string `json:"containers"`
	// Annotations stores checkpoint-related annotations (keys defined in annotations.go)
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PodmanNetworkSubnet represents a single subnet entry in the Podman network status
type PodmanNetworkSubnet struct {
	IPNet   string `json:"ipnet"`
	Gateway string `json:"gateway"`
}

// PodmanNetworkInterface represents a network interface in the Podman network status
type PodmanNetworkInterface struct {
	Subnets    []PodmanNetworkSubnet `json:"subnets"`
	MacAddress string                `json:"mac_address"`
}

// PodmanNetworkResult represents the network status for a single CNI/netavark network
type PodmanNetworkResult struct {
	Interfaces map[string]PodmanNetworkInterface `json:"interfaces"`
}

// PodmanNetworkStatus maps network names to their results in the network.status file
type PodmanNetworkStatus map[string]PodmanNetworkResult

func ReadContainerCheckpointNetworkStatus(checkpointDirectory string) (*PodmanNetworkStatus, string, error) {
	var networkStatus PodmanNetworkStatus
	networkStatusFile, err := ReadJSONFile(&networkStatus, checkpointDirectory, NetworkStatusFile)

	return &networkStatus, networkStatusFile, err
}

func ReadContainerCheckpointSpecDump(checkpointDirectory string) (*spec.Spec, string, error) {
	var specDump spec.Spec
	specDumpFile, err := ReadJSONFile(&specDump, checkpointDirectory, SpecDumpFile)

	return &specDump, specDumpFile, err
}

func ReadContainerCheckpointConfigDump(checkpointDirectory string) (*ContainerConfig, string, error) {
	var containerConfig ContainerConfig
	configDumpFile, err := ReadJSONFile(&containerConfig, checkpointDirectory, ConfigDumpFile)

	return &containerConfig, configDumpFile, err
}

func ReadContainerCheckpointDeletedFiles(checkpointDirectory string) ([]string, string, error) {
	var deletedFiles []string
	deletedFilesFile, err := ReadJSONFile(&deletedFiles, checkpointDirectory, DeletedFilesFile)

	return deletedFiles, deletedFilesFile, err
}

func ReadContainerCheckpointStatusFile(checkpointDirectory string) (*ContainerdStatus, string, error) {
	var containerdStatus ContainerdStatus
	statusFile, err := ReadJSONFile(&containerdStatus, checkpointDirectory, StatusFile)

	return &containerdStatus, statusFile, err
}

func ReadCheckpointPodOptions(checkpointDirectory string) (*CheckpointedPodOptions, string, error) {
	var podOptions CheckpointedPodOptions
	podOptionsFile, err := ReadJSONFile(&podOptions, checkpointDirectory, PodOptionsFile)

	return &podOptions, podOptionsFile, err
}

// WriteJSONFile marshalls and writes the given data to a JSON file
func WriteJSONFile(v interface{}, dir, file string) (string, error) {
	fileJSON, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshalling JSON: %w", err)
	}
	file = filepath.Join(dir, file)
	if err := os.WriteFile(file, fileJSON, 0o600); err != nil {
		return "", err
	}

	return file, nil
}

// ReadJSONFile reads JSON from a regular file in dir. On Unix, a symbolic link
// in the final path component is rejected. On Linux, reopening the validated
// file descriptor requires access to a usable procfs instance.
func ReadJSONFile(v interface{}, dir, file string) (string, error) {
	file = filepath.Join(dir, file)
	f, err := openRegularFile(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	if err = json.Unmarshal(content, v); err != nil {
		return "", fmt.Errorf("failed to unmarshal %s: %w", file, err)
	}

	return file, nil
}

// openRegularFile applies platform-specific opening safeguards and verifies
// the opened descriptor before returning it.
func openRegularFile(file string) (*os.File, error) {
	f, err := openFile(file)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("%s is %w", file, errNotRegularFile)
	}

	return f, nil
}

func ByteToString(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
