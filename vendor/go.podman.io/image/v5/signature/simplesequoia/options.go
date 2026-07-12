package simplesequoia

import (
	"errors"
	"strings"
)

type Option func(*simpleSequoiaSigner) error

// WithSequoiaHome returns an Option for NewSigner, specifying a Sequoia home directory to use.
func WithSequoiaHome(sequoiaHome string) Option {
	return func(s *simpleSequoiaSigner) error {
		s.sequoiaHome = sequoiaHome
		return nil
	}
}

// WithKeyFingerprint returns an Option for NewSigner, specifying a key to sign with, using the provided Sequoia-PGP key fingerprint.
func WithKeyFingerprint(keyFingerprint string) Option {
	return func(s *simpleSequoiaSigner) error {
		s.keyFingerprint = keyFingerprint
		return nil
	}
}

// WithPassphrase returns an Option for NewSigner, specifying a passphrase for the private key.
func WithPassphrase(passphrase string) Option {
	return func(s *simpleSequoiaSigner) error {
		// The gpgme implementation can’t use passphrase with \n; reject it here for consistent behavior.
		// FIXME: We don’t need it in this API at all, but the "\n" check exists in the current call stack. That should go away.
		if strings.Contains(passphrase, "\n") {
			return errors.New("invalid passphrase: must not contain a line break")
		}
		s.passphrase = passphrase
		return nil
	}
}
