package path

import (
	"fmt"

	"github.com/opencontainers/go-digest"
)

// RegularFilePath returns the path used in the composefs backing store for a
// regular file with the provided content digest.
//
// The caller MUST ensure d is a valid digest (in particular, that it contains no path separators or .. entries)
func RegularFilePathForValidatedDigest(d digest.Digest) (string, error) {
	if algo := d.Algorithm(); algo != digest.SHA256 {
		return "", fmt.Errorf("unexpected digest algorithm %q", algo)
	}
	e := d.Encoded()
	return e[0:2] + "/" + e[2:], nil
}
