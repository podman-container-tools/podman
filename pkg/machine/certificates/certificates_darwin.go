package certificates

import (
	"crypto/x509"
	"encoding/pem"
	"os/exec"

	"github.com/sirupsen/logrus"
)

var macKeychains = []string{
	"/System/Library/Keychains/SystemRootCertificates.keychain",
	"/Library/Keychains/System.keychain",
}

// extractHostCertificates extracts trusted CA certificates from the macOS system keychains
func extractHostCertificates() []*x509.Certificate {
	var certificates []*x509.Certificate
	for _, keychain := range macKeychains {
		certs := extractFromKeychain(keychain)
		certificates = append(certificates, certs...)
	}
	return certificates
}

// extractFromKeychain extracts certificates from a specific macOS keychain file
// using the `security` command-line tool.
func extractFromKeychain(keychainPath string) []*x509.Certificate {
	// find-certificate [-h] [-a] [-c name] [-e emailAddress] [-m] [-p] [-Z] [keychain...]
	// -a  Find all matching certificates, not just the first one
	// -p  Output certificate in pem format
	out, err := exec.Command("security", "find-certificate", "-a", "-p", keychainPath).Output()
	if err != nil {
		logrus.Debugf("Failed to extract certificates from keychain %s: %v", keychainPath, err)
		return nil
	}

	var certs []*x509.Certificate
	rest := out
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
			logrus.Debugf("Failed to parse certificate from keychain %s: %v", keychainPath, err)
			continue
		}
		certs = append(certs, cert)
	}
	logrus.Debugf("Extracted %d certificates from keychain %s", len(certs), keychainPath)
	return certs
}
