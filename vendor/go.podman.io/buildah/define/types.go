package define

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/buildah/internal/urlsource"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/storage/pkg/archive"
	"go.podman.io/storage/pkg/chrootarchive"
	"go.podman.io/storage/types"
)

const (
	// Package is the name of this package, used in help output and to
	// identify working containers.
	Package = "buildah"
	// Version for the Package. Also used by .packit.sh for Packit builds.
	Version = "1.44.0"

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

// Secret is a secret source that can be used in a RUN
type Secret struct {
	ID         string
	Source     string
	SourceType string
}

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

// TempDirForURL checks if the passed-in string looks like a URL or "-".  If it
// is, TempDirForURL creates a temporary directory, arranges for its contents
// to be the contents of that URL, and returns the temporary directory's path
// (for cleanup) and a relative subdirectory to the build context within it.
// Removal of the temporary directory is the responsibility of the caller.
// If the string doesn't look like a URL or "-", TempDirForURL returns empty
// strings and a nil error code.
func TempDirForURL(dir, prefix, url string) (tempDir string, relativeContextDir string, err error) {
	if !urlsource.IsHTTPOrHTTPS(url) &&
		!strings.HasPrefix(url, "git://") &&
		!strings.HasPrefix(url, "github.com/") &&
		url != "-" {
		return "", "", nil
	}
	tempDir, err = os.MkdirTemp(dir, prefix)
	if err != nil {
		return "", "", fmt.Errorf("creating temporary directory for %q: %w", url, err)
	}
	succeeded := false
	defer func() {
		if !succeeded {
			if err2 := os.RemoveAll(tempDir); err2 != nil {
				logrus.Errorf("error removing temporary directory %q: %v", tempDir, err2)
			}
		}
	}()

	downloadDir := filepath.Join(tempDir, "download")
	if err = os.MkdirAll(downloadDir, 0o700); err != nil {
		return "", "", fmt.Errorf("creating directory %q for %q: %w", downloadDir, url, err)
	}

	var contentSubdir string
	urlParsed, parseErr := neturl.Parse(url)
	if parseErr != nil {
		return "", "", fmt.Errorf("parsing url %q: %w", url, parseErr)
	}

	isGitURL := urlParsed.Scheme == "git" || strings.HasSuffix(urlParsed.Path, ".git")
	switch {
	case isGitURL:
		combinedOutput, gitSubDir, cloneErr := cloneToDirectory(url, downloadDir)
		if cloneErr != nil {
			return "", "", fmt.Errorf("cloning %q to %q:\n%s: %w", url, tempDir, string(combinedOutput), cloneErr)
		}
		contentSubdir = gitSubDir
	case urlsource.IsHTTPOrHTTPS(url):
		if err = downloadToDirectory(url, downloadDir); err != nil {
			return "", "", err
		}
	case strings.HasPrefix(url, "github.com/"):
		ghURL := url
		contentSubdir = path.Base(ghURL) + "-master"
		downloadURL := fmt.Sprintf("https://%s/archive/master.tar.gz", ghURL)
		logrus.Debugf("resolving url %q to %q", ghURL, downloadURL)
		if err = downloadToDirectory(downloadURL, downloadDir); err != nil {
			return "", "", err
		}
	case url == "-":
		if err = stdinToDirectory(downloadDir); err != nil {
			return "", "", err
		}
	}

	contextDir, err := securejoin.SecureJoin(downloadDir, contentSubdir)
	if err != nil {
		return "", "", fmt.Errorf("resolving subdirectory %q in %q: %w", contentSubdir, downloadDir, err)
	}
	relativeContextDir, err = filepath.Rel(tempDir, contextDir)
	if err != nil {
		return "", "", err
	}
	logrus.Debugf("Build context is at %q", contextDir)
	succeeded = true
	return tempDir, relativeContextDir, nil
}

// parseGitBuildContext parses git build context to `repo`, `sub-dir`
// `branch/commit`, accepts GitBuildContext in the format of
// `repourl.git[#[branch-or-commit]:subdir]`.
func parseGitBuildContext(url string) (string, string, string) {
	gitSubdir := ""
	gitBranch := ""
	gitBranchPart := strings.Split(url, "#")
	if len(gitBranchPart) > 1 {
		// check if string contains path to a subdir
		gitSubDirPart := strings.Split(gitBranchPart[1], ":")
		if len(gitSubDirPart) > 1 {
			gitSubdir = gitSubDirPart[1]
		}
		gitBranch = gitSubDirPart[0]
	}
	return gitBranchPart[0], gitSubdir, gitBranch
}

func cloneToDirectory(url, dir string) ([]byte, string, error) {
	var cmd *exec.Cmd
	gitRepo, gitSubdir, gitRef := parseGitBuildContext(url)
	// init repo
	cmd = exec.Command("git", "init", dir)
	combinedOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Return err.Error() instead of err as we want buildah to override error code with more predictable
		// value.
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git init`: %s", err.Error())
	}
	// add origin
	cmd = exec.Command("git", "remote", "add", "origin", gitRepo)
	cmd.Dir = dir
	combinedOutput, err = cmd.CombinedOutput()
	if err != nil {
		// Return err.Error() instead of err as we want buildah to override error code with more predictable
		// value.
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git remote add`: %s", err.Error())
	}

	logrus.Debugf("fetching repo %q and branch (or commit ID) %q to %q", gitRepo, gitRef, dir)
	args := []string{"fetch", "-u", "--depth=1", "origin", "--", gitRef}
	cmd = exec.Command("git", args...)
	cmd.Dir = dir
	combinedOutput, err = cmd.CombinedOutput()
	if err != nil {
		// Return err.Error() instead of err as we want buildah to override error code with more predictable
		// value.
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git fetch`: %s", err.Error())
	}

	cmd = exec.Command("git", "checkout", "FETCH_HEAD")
	cmd.Dir = dir
	combinedOutput, err = cmd.CombinedOutput()
	if err != nil {
		// Return err.Error() instead of err as we want buildah to override error code with more predictable
		// value.
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git checkout`: %s", err.Error())
	}
	return combinedOutput, gitSubdir, nil
}

func downloadToDirectory(url, dir string) error {
	logrus.Debugf("extracting %q to %q", url, dir)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("invalid response status %d", resp.StatusCode)
	}
	if resp.ContentLength == 0 {
		return fmt.Errorf("no contents in %q", url)
	}
	// Try to extract the response as a tar archive; if that fails,
	// assume it is a raw Dockerfile and write it as such.
	if err := chrootarchive.Untar(resp.Body, dir, nil); err != nil {
		resp1, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp1.Body.Close()
		body, err := io.ReadAll(resp1.Body)
		if err != nil {
			return err
		}
		if err := writeFileInRoot(dir, "Dockerfile", body, 0o600); err != nil {
			return fmt.Errorf("failed to write %q to %q: %w", url, filepath.Join(dir, "Dockerfile"), err)
		}
	}
	return nil
}

func stdinToDirectory(dir string) error {
	logrus.Debugf("extracting stdin to %q", dir)
	r := bufio.NewReader(os.Stdin)
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}
	// Try to extract the buffered input as a tar archive; if that fails,
	// assume it is a raw Dockerfile and write it as such.
	reader := bytes.NewReader(b)
	if err := chrootarchive.Untar(reader, dir, nil); err != nil {
		if err := writeFileInRoot(dir, "Dockerfile", b, 0o600); err != nil {
			return fmt.Errorf("failed to write bytes to %q: %w", filepath.Join(dir, "Dockerfile"), err)
		}
	}
	return nil
}

// writeFileInRoot safely writes data to a file inside root, without following
// symlinks that escape the root directory.
func writeFileInRoot(root, name string, data []byte, perm os.FileMode) error { //nolint:unparam,nolintlint
	// Above:
	// unparam: 'name' currently only receives "Dockerfile" but will potentially support other files later
	// nolintlint: the unparam linter only triggers if there are ≥ 4 instances; we do have that
	// with --tests defaulting to true, but not with --tests=false.

	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer rootHandle.Close()

	if err := rootHandle.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	fileHandle, err := rootHandle.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}

	_, err = fileHandle.Write(data)
	if closeErr := fileHandle.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}
