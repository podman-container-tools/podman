//go:build freebsd || linux

package certificates

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// extractHostCertificates extracts trusted CA certificates from
// the Linux system certificate bundles (stop after finding one).
// For consistency with the Go stdlib, if the environment variables
// SSL_CERT_FILE and SSL_CERT_DIR are set, they override the system
// default locations.
func extractHostCertificates() []*x509.Certificate {
	var certificates []*x509.Certificate

	if certFile, ok := os.LookupEnv("SSL_CERT_FILE"); ok {
		certs := extractFromCertBundle(certFile)
		certificates = append(certificates, certs...)
	}

	if certDir, ok := os.LookupEnv("SSL_CERT_DIR"); ok {
		certs := extractFromCertDir(certDir)
		certificates = append(certificates, certs...)
	}

	if len(certificates) > 0 {
		return certificates
	}

	for _, bundlePath := range certBundlePaths {
		certs := extractFromCertBundle(bundlePath)
		if len(certs) > 0 {
			return certs
		}
	}
	return nil
}

// extractFromCertBundle reads and parses PEM-encoded certificates from a
// certificate bundle file.
func extractFromCertBundle(bundlePath string) []*x509.Certificate {
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		logrus.Debugf("Failed to read certificate bundle %s: %v", bundlePath, err)
		return nil
	}

	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			logrus.Debugf("Failed to parse certificate from bundle %s: %v", bundlePath, err)
			continue
		}
		certs = append(certs, cert)
	}
	logrus.Debugf("Extracted %d certificates from bundle %s", len(certs), bundlePath)
	return certs
}

func extractFromCertDir(dirPath string) []*x509.Certificate {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		logrus.Debugf("Failed to read certificate directory %s: %v", dirPath, err)
		return nil
	}

	var certs []*x509.Certificate
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		certs = append(certs, extractFromCertBundle(filepath.Join(dirPath, entry.Name()))...)
	}
	logrus.Debugf("Extracted %d certificates from directory %s", len(certs), dirPath)
	return certs
}
