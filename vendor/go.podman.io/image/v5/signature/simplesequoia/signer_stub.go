//go:build !containers_image_sequoia

package simplesequoia

import (
	"errors"

	"go.podman.io/image/v5/signature/signer"
)

// simpleSequoiaSigner is a signer.SignerImplementation implementation for simple signing signatures using Sequoia.
type simpleSequoiaSigner struct {
	// This is not really used, we just keep the struct fields so that the With… Option functions can be compiled.

	sequoiaHome    string // "" if using the system's default
	keyFingerprint string
	passphrase     string // "" if not provided.
}

// NewSigner returns a signature.Signer which creates "simple signing" signatures using the user's default
// Sequoia PGP configuration.
//
// The set of options must identify a key to sign with, probably using a WithKeyFingerprint.
//
// The caller must call Close() on the returned Signer.
func NewSigner(opts ...Option) (*signer.Signer, error) {
	return nil, errors.New("Sequoia-PGP support is not enabled in this build")
}
