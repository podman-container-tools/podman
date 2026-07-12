package urlsource

import (
	"strings"

	"go.podman.io/storage/pkg/regexp"
)

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
