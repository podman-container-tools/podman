//go:build freebsd || linux

package certificates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Self-signed test certificate generated with:
//
//	openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
//	  -keyout NUL -nodes -days 3650 -subj "/CN=Test CA"
const testCertPEM = `-----BEGIN CERTIFICATE-----
MIIBeTCCAR+gAwIBAgIUJU2uvB3odGixXSogBtJkbj9F4R4wCgYIKoZIzj0EAwIw
EjEQMA4GA1UEAwwHVGVzdCBDQTAeFw0yNjA0MzAyMTEzMTlaFw0zNjA0MjcyMTEz
MTlaMBIxEDAOBgNVBAMMB1Rlc3QgQ0EwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNC
AATpHURrMAmb3C5aOPhcYhesyivuSeFGee9OUnjamS0lm0aiLPxnbQZRe58yppXv
F3AlW0CLHdA3PQxlhmbCtdz+o1MwUTAdBgNVHQ4EFgQUNeQe4WGi2+N7YQh6lsDG
T+fqv/wwHwYDVR0jBBgwFoAUNeQe4WGi2+N7YQh6lsDGT+fqv/wwDwYDVR0TAQH/
BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiEArJQPc60I6e0vMkl1u4AFAbiei4be
81eHA5/1che3VcoCIGxHae0h9z+9mcJRPeL6B7jXOFyFXdzGtBqmGu9JwkLO
-----END CERTIFICATE-----`

func TestExtractHostCertificatesFromSSLCertFile(t *testing.T) {
	certFile := filepath.Join(t.TempDir(), "cert.pem")
	require.NoError(t, os.WriteFile(certFile, []byte(testCertPEM), 0o644))

	t.Setenv("SSL_CERT_FILE", certFile)

	certs := extractHostCertificates()
	require.Len(t, certs, 1)
	assert.Equal(t, "Test CA", certs[0].Subject.CommonName)
}

func TestExtractHostCertificatesFromSSLCertDir(t *testing.T) {
	certDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(certDir, "ca.pem"), []byte(testCertPEM), 0o644))

	t.Setenv("SSL_CERT_DIR", certDir)

	certs := extractHostCertificates()
	require.Len(t, certs, 1)
	assert.Equal(t, "Test CA", certs[0].Subject.CommonName)
}
