//go:build containers_image_sequoia

package simplesequoia

// This implements a signature.signingMechanismWithPassphrase that only supports signing.
//
// FIXME: Consider restructuring the simple signing signature creation code path
// not to require this indirection and all those unimplemented methods.

import (
	"go.podman.io/image/v5/signature/internal/sequoia"
)

// A GPG/OpenPGP signing mechanism, implemented using Sequoia.
type sequoiaSigningOnlyMechanism struct {
	inner *sequoia.SigningMechanism
}

func (m *sequoiaSigningOnlyMechanism) Close() error {
	panic("Should never be called")
}

// SupportsSigning returns nil if the mechanism supports signing, or a SigningNotSupportedError.
func (m *sequoiaSigningOnlyMechanism) SupportsSigning() error {
	panic("Should never be called")
}

// Sign creates a (non-detached) signature of input using keyIdentity and passphrase.
// Fails with a SigningNotSupportedError if the mechanism does not support signing.
func (m *sequoiaSigningOnlyMechanism) SignWithPassphrase(input []byte, keyIdentity string, passphrase string) ([]byte, error) {
	return m.inner.SignWithPassphrase(input, keyIdentity, passphrase)
}

// Sign creates a (non-detached) signature of input using keyIdentity.
// Fails with a SigningNotSupportedError if the mechanism does not support signing.
func (m *sequoiaSigningOnlyMechanism) Sign(input []byte, keyIdentity string) ([]byte, error) {
	panic("Should never be called")
}

// Verify parses unverifiedSignature and returns the content and the signer's identity
func (m *sequoiaSigningOnlyMechanism) Verify(unverifiedSignature []byte) (contents []byte, keyIdentity string, err error) {
	panic("Should never be called")
}

// UntrustedSignatureContents returns UNTRUSTED contents of the signature WITHOUT ANY VERIFICATION,
// along with a short identifier of the key used for signing.
// WARNING: The short key identifier (which corresponds to "Key ID" for OpenPGP keys)
// is NOT the same as a "key identity" used in other calls to this interface, and
// the values may have no recognizable relationship if the public key is not available.
func (m *sequoiaSigningOnlyMechanism) UntrustedSignatureContents(untrustedSignature []byte) (untrustedContents []byte, shortKeyIdentifier string, err error) {
	panic("Should never be called")
}
