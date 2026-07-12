package certificates

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func verifyCertificateFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block from certificate file")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}
	return nil
}

func TestExtractAndSaveCertificates(t *testing.T) {
	certs := extractHostCertificates()
	assert.NotEmpty(t, certs)

	filePath := filepath.Join(t.TempDir(), "cert.pem")
	err := saveCertificatesToPEM(certs, filePath)
	assert.NoError(t, err)

	err = verifyCertificateFile(filePath)
	assert.NoError(t, err)
}

func TestDeduplicateCertificates(t *testing.T) {
	certA := &x509.Certificate{Signature: []byte("sig-a")}
	certB := &x509.Certificate{Signature: []byte("sig-b")}
	certC := &x509.Certificate{Signature: []byte("sig-c")}

	tests := []struct {
		name     string
		input    []*x509.Certificate
		expected []*x509.Certificate
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []*x509.Certificate{},
			expected: nil,
		},
		{
			name:     "no duplicates",
			input:    []*x509.Certificate{certA, certB, certC},
			expected: []*x509.Certificate{certA, certB, certC},
		},
		{
			name:     "with duplicates",
			input:    []*x509.Certificate{certA, certB, certA, certC, certB},
			expected: []*x509.Certificate{certA, certB, certC},
		},
		{
			name:     "all duplicates",
			input:    []*x509.Certificate{certA, certA, certA},
			expected: []*x509.Certificate{certA},
		},
		{
			name:     "nil entries are skipped",
			input:    []*x509.Certificate{nil, certA, nil, certB},
			expected: []*x509.Certificate{certA, certB},
		},
		{
			name:     "nil and duplicate entries",
			input:    []*x509.Certificate{nil, certA, certB, nil, certA},
			expected: []*x509.Certificate{certA, certB},
		},
		{
			name:     "single certificate",
			input:    []*x509.Certificate{certA},
			expected: []*x509.Certificate{certA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateCertificates(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
