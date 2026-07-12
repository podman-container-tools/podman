package directory

import (
	"fmt"
)

const (
	versionPrefix = "Directory Transport Version: "
)

// version represents a parsed directory transport version
type version struct {
	major int
	minor int
}

// Supported versions
// Write version file based on digest algorithm used.
// 1.1 for sha256-only images, 1.2 otherwise.
var (
	version1_1          = version{major: 1, minor: 1}
	version1_2          = version{major: 1, minor: 2}
	maxSupportedVersion = version1_2
)

// String formats a version as a string suitable for writing to the version file
func (v version) String() string {
	return fmt.Sprintf("%s%d.%d\n", versionPrefix, v.major, v.minor)
}

// parseVersion parses a version string into major and minor components.
// Returns an error if the format is invalid.
func parseVersion(versionStr string) (version, error) {
	var v version
	expectedFormat := versionPrefix + "%d.%d\n"
	// Sscanf parsing is a bit loose (treats spaces specially), but a strict check immediately follows
	n, err := fmt.Sscanf(versionStr, expectedFormat, &v.major, &v.minor)
	if err != nil || n != 2 || versionStr != v.String() {
		return version{}, fmt.Errorf("invalid version format")
	}
	return v, nil
}

// TODO: Potential refactor for better interoperability with `cmp`
// https://github.com/containers/container-libs/pull/475#discussion_r2571131267
// isGreaterThan returns true if v is greater than other
func (v version) isGreaterThan(other version) bool {
	if v.major != other.major {
		return v.major > other.major
	}
	return v.minor > other.minor
}

// UnsupportedVersionError indicates that the directory uses a version newer than we support
type UnsupportedVersionError struct {
	Version string // The unsupported version string found
	Path    string // The path to the directory
}

func (e UnsupportedVersionError) Error() string {
	return fmt.Sprintf("unsupported directory transport version %q at %s", e.Version, e.Path)
}
