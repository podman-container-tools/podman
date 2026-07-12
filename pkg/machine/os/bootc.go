package os

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/manifest"
)

// BootcHost represents the top-level bootc status structure
type BootcHost struct {
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	Metadata   bootcMetadata `json:"metadata"`
	Spec       bootcSpec     `json:"spec"`
	Status     bootcStatus   `json:"status"`
}

// bootcMetadata contains metadata about the host
type bootcMetadata struct {
	Name string `json:"name"`
}

// bootcSpec contains the specification for the bootc host
type bootcSpec struct {
	BootOrder string   `json:"bootOrder"`
	Image     imageRef `json:"image"`
}

// imageRef represents a container image reference
type imageRef struct {
	Image     string `json:"image"`
	Transport string `json:"transport"`
}

// bootcStatus contains the current status of the bootc host
type bootcStatus struct {
	Booted         *bootEntry `json:"booted"`
	Rollback       *bootEntry `json:"rollback"`
	Staged         *bootEntry `json:"staged"`
	RollbackQueued bool       `json:"rollbackQueued"`
	Type           string     `json:"type"`
}

// bootEntry represents a boot entry (booted, rollback, or staged)
type bootEntry struct {
	CachedUpdate      *imageStatus `json:"cachedUpdate"`
	Image             imageStatus  `json:"image"`
	Incompatible      bool         `json:"incompatible"`
	Ostree            ostreeInfo   `json:"ostree"`
	Pinned            bool         `json:"pinned"`
	SoftRebootCapable bool         `json:"softRebootCapable"`
	Store             string       `json:"store"`
}

// imageStatus contains detailed information about a container image
type imageStatus struct {
	Architecture string          `json:"architecture"`
	Image        imageRefWithSig `json:"image"`
	ImageDigest  string          `json:"imageDigest"`
	Timestamp    time.Time       `json:"timestamp"`
	Version      string          `json:"version"`
}

// imageRefWithSig is an image reference that may include a signature
type imageRefWithSig struct {
	Image     string     `json:"image"`
	Transport string     `json:"transport"`
	Signature *signature `json:"signature,omitempty"`
}

// signature contains signature information for an image
type signature struct {
	OstreeRemote string `json:"ostreeRemote"`
}

// ostreeInfo contains OSTree-specific information
type ostreeInfo struct {
	Checksum     string `json:"checksum"`
	DeploySerial int    `json:"deploySerial"`
	Stateroot    string `json:"stateroot"`
}

// GetBootedImageRef returns the booted image reference (status -> booted -> image -> image)
// Returns an empty string if booted is nil
func (b *BootcHost) GetBootedImageRef() string {
	if b.Status.Booted == nil {
		return ""
	}
	return b.Status.Booted.Image.Image.Image
}

// GetBootedOSTreeChecksum returns the booted OSTree checksum
// Returns an empty string if booted is nil
func (b *BootcHost) GetBootedOSTreeChecksum() string {
	if b.Status.Booted == nil {
		return ""
	}
	return b.Status.Booted.Ostree.Checksum
}

func (b *BootcHost) getLocalOSOCIInfo() (reference.Named, *semver.Version, error) {
	ref, err := reference.ParseNormalizedNamed(b.GetBootedImageRef())
	if err != nil {
		return nil, nil, err
	}
	named, ok := ref.(reference.NamedTagged)
	if !ok {
		return nil, nil, errors.New("failed to parse ostree origin")
	}
	tagAsVersion, err := semver.ParseTolerant(named.Tag())
	return named, &tagAsVersion, err
}

func (b *BootcHost) getLocalOsImageDigest() (digest.Digest, error) {
	args := []string{"container", "image", "metadata", "--repo", "/ostree/repo", "docker://" + b.GetBootedImageRef()}
	logrus.Debugf("executing : ostree %s", strings.Join(args, " "))
	cmd := exec.Command("ostree", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	localOs, err := manifest.FromBlob(out, v1.MediaTypeImageManifest)
	if err != nil {
		return "", err
	}
	return localOs.ConfigInfo().Digest, nil
}

// newBootcHost creates a new BootcHost by executing 'sudo bootc status --format json'
// and unmarshaling the output
func newBootcHost() (*BootcHost, error) {
	cmd := exec.Command("sudo", "bootc", "status", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var host BootcHost
	if err := json.Unmarshal(output, &host); err != nil {
		return nil, err
	}
	return &host, nil
}
