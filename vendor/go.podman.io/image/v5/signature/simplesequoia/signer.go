//go:build containers_image_sequoia

package simplesequoia

import (
	"context"
	"errors"
	"fmt"

	"go.podman.io/image/v5/docker/reference"
	internalSig "go.podman.io/image/v5/internal/signature"
	internalSigner "go.podman.io/image/v5/internal/signer"
	"go.podman.io/image/v5/signature"
	"go.podman.io/image/v5/signature/internal/sequoia"
	"go.podman.io/image/v5/signature/signer"
)

// simpleSequoiaSigner is a signer.SignerImplementation implementation for simple signing signatures using Sequoia.
type simpleSequoiaSigner struct {
	mech           *sequoia.SigningMechanism
	sequoiaHome    string // "" if using the system’s default
	keyFingerprint string
	passphrase     string // "" if not provided.
}

// NewSigner returns a signature.Signer which creates “simple signing” signatures using the user’s default
// Sequoia PGP configuration.
//
// The set of options must identify a key to sign with, probably using a WithKeyFingerprint.
//
// The caller must call Close() on the returned Signer.
func NewSigner(opts ...Option) (*signer.Signer, error) {
	s := simpleSequoiaSigner{}
	for _, o := range opts {
		if err := o(&s); err != nil {
			return nil, err
		}
	}
	if s.keyFingerprint == "" {
		return nil, errors.New("no key identity provided for simple signing")
	}

	if err := sequoia.Init(); err != nil {
		return nil, err // Coverage: This is impractical to test in-process, with the static go_sequoia_dlhandle.
	}
	mech, err := sequoia.NewMechanismFromDirectory(s.sequoiaHome)
	if err != nil {
		return nil, fmt.Errorf("initializing Sequoia: %w", err)
	}
	s.mech = mech
	succeeded := false
	defer func() {
		if !succeeded {
			s.mech.Close() // Coverage: This is currently unreachable.
		}
	}()

	// Ideally, we should look up (and unlock?) the key at this point already. FIXME: is that possible? Anyway, low-priority.

	succeeded = true
	return internalSigner.NewSigner(&s), nil
}

// ProgressMessage returns a human-readable sentence that makes sense to write before starting to create a single signature.
func (s *simpleSequoiaSigner) ProgressMessage() string {
	return "Signing image using Sequoia-PGP simple signing"
}

// SignImageManifest creates a new signature for manifest m as dockerReference.
func (s *simpleSequoiaSigner) SignImageManifest(ctx context.Context, m []byte, dockerReference reference.Named) (internalSig.Signature, error) {
	if reference.IsNameOnly(dockerReference) {
		return nil, fmt.Errorf("reference %s can’t be signed, it has neither a tag nor a digest", dockerReference.String())
	}
	wrapped := sequoiaSigningOnlyMechanism{
		inner: s.mech,
	}
	simpleSig, err := signature.SignDockerManifestWithOptions(m, dockerReference.String(), &wrapped, s.keyFingerprint, &signature.SignOptions{
		Passphrase: s.passphrase,
	})
	if err != nil {
		return nil, err
	}
	return internalSig.SimpleSigningFromBlob(simpleSig), nil
}

func (s *simpleSequoiaSigner) Close() error {
	return s.mech.Close()
}
