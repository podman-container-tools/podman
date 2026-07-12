package image

import (
	"fmt"

	"github.com/opencontainers/go-digest"
)

func validateBlobAgainstDigest(blob []byte, expectedDigest digest.Digest) error {
	if expectedDigest == "" {
		return fmt.Errorf("expected digest is empty")
	}
	err := expectedDigest.Validate()
	if err != nil {
		return fmt.Errorf("invalid digest format %q: %w", expectedDigest, err)
	}
	digestAlgorithm := expectedDigest.Algorithm()
	if !digestAlgorithm.Available() {
		return fmt.Errorf("unsupported digest algorithm: %s", digestAlgorithm)
	}
	computedDigest := digestAlgorithm.FromBytes(blob)
	if computedDigest != expectedDigest {
		return fmt.Errorf("blob digest %s does not match expected %s", computedDigest, expectedDigest)
	}
	return nil
}
