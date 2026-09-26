package define

import (
	"context"
	"fmt"
	"os"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"go.podman.io/buildah/pkg/tmpdir"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/storage/pkg/archive"
	"go.podman.io/storage/types"
)

const (
	// Package is the name of this package, used in help output and to
	// identify working containers.
	Package = "buildah"
	// Version for the Package. Also used by .packit.sh for Packit builds.
	Version = "1.46.0-dev"

	// DefaultRuntime if containers.conf fails.
	DefaultRuntime = "runc"

	// OCIv1ImageManifest is the MIME type of an OCIv1 image manifest,
	// suitable for specifying as a value of the PreferredManifestType
	// member of a CommitOptions structure.  It is also the default.
	OCIv1ImageManifest = v1.MediaTypeImageManifest
	// Dockerv2ImageManifest is the MIME type of a Docker v2s2 image
	// manifest, suitable for specifying as a value of the
	// PreferredManifestType member of a CommitOptions structure.
	Dockerv2ImageManifest = manifest.DockerV2Schema2MediaType

	// OCI used to define the "oci" image format
	OCI = "oci"
	// DOCKER used to define the "docker" image format
	DOCKER = "docker"

	// SEV is a known trusted execution environment type: AMD-SEV (secure encrypted virtualization using encrypted state, requires epyc 1000 "naples")
	SEV TeeType = "sev"
	// SNP is a known trusted execution environment type: AMD-SNP (SEV secure nested pages) (requires epyc 3000 "milan")
	SNP TeeType = "snp"
)

// DefaultRlimitValue is the value set by default for nofile and nproc
const RLimitDefaultValue = uint64(1048576)

// TeeType is a supported trusted execution environment type.
type TeeType string

var (
	// Deprecated: DefaultCapabilities values should be retrieved from
	// github.com/containers/common/pkg/config
	DefaultCapabilities = []string{
		"CAP_AUDIT_WRITE",
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_FOWNER",
		"CAP_FSETID",
		"CAP_KILL",
		"CAP_MKNOD",
		"CAP_NET_BIND_SERVICE",
		"CAP_SETFCAP",
		"CAP_SETGID",
		"CAP_SETPCAP",
		"CAP_SETUID",
		"CAP_SYS_CHROOT",
	}
	// Deprecated: DefaultNetworkSysctl values should be retrieved from
	// github.com/containers/common/pkg/config
	DefaultNetworkSysctl = map[string]string{
		"net.ipv4.ping_group_range": "0 0",
	}

	Gzip         = archive.Gzip
	Bzip2        = archive.Bzip2
	Xz           = archive.Xz
	Zstd         = archive.Zstd
	Uncompressed = archive.Uncompressed
)

// IDMappingOptions controls how we set up UID/GID mapping when we set up a
// user namespace.
type IDMappingOptions struct {
	HostUIDMapping bool
	HostGIDMapping bool
	UIDMap         []specs.LinuxIDMapping
	GIDMap         []specs.LinuxIDMapping
	AutoUserNs     bool
	AutoUserNsOpts types.AutoUserNsOptions
}

// Secret is a secret source that can be provided for a RUN instruction in a
// build, or passed to Run(), and optionally mounted as type=secret.
type Secret struct {
	ID         string
	Source     string
	SourceType string
}

// ResolveValue returns the contents of the secret, whatever its source type.
func (s Secret) ResolveValue() ([]byte, error) {
	switch s.SourceType {
	case "env":
		return []byte(os.Getenv(s.Source)), nil
	case "file":
		rv, err := os.ReadFile(s.Source)
		if err != nil {
			return nil, fmt.Errorf("reading file for secret ID %s: %w", s.ID, err)
		}
		return rv, nil
	default:
		return nil, fmt.Errorf("invalid secret type: %s for secret ID: %s", s.SourceType, s.ID)
	}
}

// BuildOutputOptions contains the outcome of parsing the value of a build --output flag
// Deprecated: This structure is now internal
type BuildOutputOption struct {
	Path     string // Only valid if !IsStdout
	IsDir    bool
	IsStdout bool
}

// ConfidentialWorkloadOptions encapsulates options which control whether or not
// we output an image whose rootfs contains a LUKS-compatibly-encrypted disk image
// instead of the usual rootfs contents.
type ConfidentialWorkloadOptions struct {
	Convert                  bool
	AttestationURL           string
	CPUs                     int
	Memory                   int
	TempDir                  string // used for the temporary plaintext copy of the disk image
	TeeType                  TeeType
	IgnoreAttestationErrors  bool
	WorkloadID               string
	DiskEncryptionPassphrase string
	Slop                     string
	FirmwareLibrary          string
}

// SBOMMergeStrategy tells us how to merge multiple SBOM documents into one.
type SBOMMergeStrategy string

const (
	// SBOMMergeStrategyCat literally concatenates the documents.
	SBOMMergeStrategyCat SBOMMergeStrategy = "cat"
	// SBOMMergeStrategyCycloneDXByComponentNameAndVersion adds components
	// from the second document to the first, so long as they have a
	// name+version combination which is not already present in the
	// components array.
	SBOMMergeStrategyCycloneDXByComponentNameAndVersion SBOMMergeStrategy = "merge-cyclonedx-by-component-name-and-version"
	// SBOMMergeStrategySPDXByPackageNameAndVersionInfo adds packages from
	// the second document to the first, so long as they have a
	// name+versionInfo combination which is not already present in the
	// first document's packages array, and adds hasExtractedLicensingInfos
	// items from the second document to the first, so long as they include
	// a licenseId value which is not already present in the first
	// document's hasExtractedLicensingInfos array.
	SBOMMergeStrategySPDXByPackageNameAndVersionInfo SBOMMergeStrategy = "merge-spdx-by-package-name-and-versioninfo"
)

// SBOMScanOptions encapsulates options which control whether or not we run a
// scanner on the rootfs that we're about to commit, and how.
type SBOMScanOptions struct {
	Type            []string          // a shorthand name for a defined group of these options
	Image           string            // the scanner image to use
	PullPolicy      PullPolicy        // how to get the scanner image
	Commands        []string          // one or more commands to invoke for the image rootfs or ContextDir locations
	ContextDir      []string          // one or more "source" directory locations
	SBOMOutput      string            // where to save SBOM scanner output outside of the image (i.e., the local filesystem)
	PURLOutput      string            // where to save PURL list outside of the image (i.e., the local filesystem)
	ImageSBOMOutput string            // where to save SBOM scanner output in the image
	ImagePURLOutput string            // where to save PURL list in the image
	MergeStrategy   SBOMMergeStrategy // how to merge the outputs of multiple scans
}

// TempDirForURL() calls TempDirForURLContext() with context.TODO().
//
//go:fix inline
func TempDirForURL(dir, prefix, url string) (tempDir, relativeContextDir string, err error) {
	return TempDirForURLContext(context.TODO(), dir, prefix, url)
}

// TempDirForURLContext checks if the passed-in string looks like a URL or "-".  If it
// is, TempDirForURLContext creates a temporary directory, arranges for its contents
// to be the contents of that URL, and returns the temporary directory's path
// (for cleanup) and a relative subdirectory to the build context within it.
// Removal of the temporary directory is the responsibility of the caller.
// If the string doesn't look like a URL or "-", TempDirForURLContext returns empty
// strings and a nil error code.
//
// Internally, it calls go.podman.io/pkg/tmpdir.ForURL() to do the heavy lifting.
func TempDirForURLContext(ctx context.Context, dir, prefix, url string) (tempDir, relativeContextDir string, err error) {
	return tmpdir.ForURL(ctx, dir, prefix, url, nil)
}
