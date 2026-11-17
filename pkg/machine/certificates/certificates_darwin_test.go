package certificates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFromKeychain(t *testing.T) {
	t.Run("system root certificates keychain", func(t *testing.T) {
		certs := extractFromKeychain("/System/Library/Keychains/SystemRootCertificates.keychain")
		require.NotEmpty(t, certs)
		for _, cert := range certs {
			assert.True(t, cert.IsCA, "expected CA certificate, got: %s", cert.Subject)
		}
	})

	t.Run("system keychain", func(t *testing.T) {
		// System.keychain may or may not have certificates depending on the machine,
		// so we just verify it doesn't error out.
		certs := extractFromKeychain("/Library/Keychains/System.keychain")
		for _, cert := range certs {
			assert.NotNil(t, cert)
		}
	})

	t.Run("nonexistent keychain", func(t *testing.T) {
		certs := extractFromKeychain("/nonexistent/path.keychain")
		assert.Empty(t, certs)
	})
}
