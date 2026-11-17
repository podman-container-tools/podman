package urlsource

import (
	"fmt"
	"os/exec"
	"strings"

	"go.podman.io/storage/pkg/regexp"
)

var commitSHAPattern = regexp.Delayed(`^[a-fA-F0-9]+$`)

// IsCommitSHAPrefix reports whether s could be a hex coded Git commit SHA
func IsCommitSHAPrefix(s string) bool {
	// 4 matches Git's own minimum abbreviation length
	return len(s) >= 4 && commitSHAPattern.MatchString(s)
}

// ResolveCommit returns the full commit SHA that HEAD points to in the Git
// working tree rooted at or above dir
func ResolveCommit(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolving HEAD commit in %q: %w", dir, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// gitURLFragmentSuffix matches fragments to use as Git reference and build
// context from the Git repository e.g.
//
//	github.com/containers/buildah.git
//	github.com/containers/buildah.git#main
//	github.com/containers/buildah.git#v1.35.0
var gitURLFragmentSuffix = regexp.Delayed(`\.git(?:#.+)?$`)

// IsHTTPOrHTTPS reports whether the source is an HTTP(S) URL.
func IsHTTPOrHTTPS(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}

// IsGit reports whether the source is an HTTP(S) Git URL.
func IsGit(source string) bool {
	return IsHTTPOrHTTPS(source) && gitURLFragmentSuffix.MatchString(source)
}

// IsRemote reports whether the source is a remote HTTP(S) URL
// and *not* a Git repository. Certain GitHub URLs such as raw.github.* are allowed.
func IsRemote(source string) bool {
	return IsHTTPOrHTTPS(source) && !gitURLFragmentSuffix.MatchString(source)
}
