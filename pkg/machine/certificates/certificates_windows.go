package certificates

import (
	"crypto/x509"
	"errors"
	"fmt"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var (
	predefinedWinStores = []string{
		"AAD Token Issuer",
		"AuthRoot",
		"CA",
		"ClientAuthIssuer",
		"Disallowed",
		"eSIM Certification Authorities",
		"FlightRoot",
		"My",
		"OemEsim",
		"PasspointTrustedRoots",
		"Remote Desktop",
		"REQUEST",
		"Root",
		"SmartCardRoot",
		"TestSignRoot",
		"Trust",
		"TrustedAppRoot",
		"TrustedDevices",
		"TrustedPeople",
		"TrustedPublisher",
		"Windows Live ID Token Issuer",
		"WindowsServerUpdateServices",
		"UserDS",
	}
	winStoresLocations = []uint{
		windows.CERT_SYSTEM_STORE_CURRENT_USER,
		windows.CERT_SYSTEM_STORE_LOCAL_MACHINE,
		windows.CERT_SYSTEM_STORE_CURRENT_SERVICE,
		windows.CERT_SYSTEM_STORE_LOCAL_MACHINE_ENTERPRISE,
	}
	winStoresLocationsNames = map[uint]string{
		windows.CERT_SYSTEM_STORE_CURRENT_USER:             "Current User",
		windows.CERT_SYSTEM_STORE_LOCAL_MACHINE:            "Local Machine",
		windows.CERT_SYSTEM_STORE_CURRENT_SERVICE:          "Current Service",
		windows.CERT_SYSTEM_STORE_LOCAL_MACHINE_ENTERPRISE: "Local Machine Enterprise",
	}
)

// extractHostCertificates extracts trusted CA certificates from the prederfined
// Windows certificate stores
func extractHostCertificates() []*x509.Certificate {
	var certificates []*x509.Certificate
	for _, location := range winStoresLocations {
		for _, store := range predefinedWinStores {
			certs := extractFromStore(location, store)
			certificates = append(certificates, certs...)
		}
	}
	return certificates
}

// extractFromStore extracts certificates from a specific Windows certificate store
func extractFromStore(storeLocation uint, storeName string) []*x509.Certificate {
	storeHandle, err := getStoreHandle(storeLocation, storeName)
	if err != nil {
		logrus.Debugf("Failed open store %s in %s: %v",
			storeName,
			winStoresLocationsNames[storeLocation],
			err)
		return nil
	}

	defer func(h windows.Handle) {
		if err := windows.CertCloseStore(h, 0); err != nil {
			logrus.Debugf("Failed to close certificate store: %v", err)
		}
	}(*storeHandle)

	var certs []*x509.Certificate
	var ctx *windows.CertContext = nil
	for {
		ctx, err = windows.CertEnumCertificatesInStore(*storeHandle, ctx)
		if err != nil {
			if errors.Is(err, windows.Errno(windows.CRYPT_E_NOT_FOUND)) {
				// The function reached the end of the store's list
				break
			}
			logrus.Debugf("Failed to enumerate certificates in store %s in %s: %v",
				storeName,
				winStoresLocationsNames[storeLocation],
				err)
			return nil
		}
		if ctx == nil {
			break
		}

		c := parseCertificate(ctx)
		if c != nil {
			certs = append(certs, c)
		}
	}
	logrus.Debugf("Extracted %d certificates from %s store in %s", len(certs), storeName, winStoresLocationsNames[storeLocation])
	return certs
}

// getStoreHandle returns a handle to the certificate store with the given name.
func getStoreHandle(storeLocation uint, storeName string) (*windows.Handle, error) {
	storeNameUTF16, err := windows.UTF16PtrFromString(storeName)
	if err != nil {
		return nil, fmt.Errorf("failed to convert store name to UTF16: %w", err)
	}
	// storeHandle, err := windows.CertOpenSystemStore(0, storeNameUTF16)
	storeHandle, err := windows.CertOpenStore(
		windows.CERT_STORE_PROV_SYSTEM,
		0,
		0,
		uint32(storeLocation)|uint32(windows.CERT_STORE_READONLY_FLAG),
		uintptr(unsafe.Pointer(storeNameUTF16)))
	if err != nil {
		return nil, fmt.Errorf("failed to open certificate store: %w", err)
	}
	return &storeHandle, nil
}

// parseCertificate parses an x509 certificate from a win32 API CertContext
// object. If the certificate cannot be parsed, nil is returned.
func parseCertificate(certCtx *windows.CertContext) *x509.Certificate {
	certSize := certCtx.Length
	certBytesOrig := unsafe.Slice(certCtx.EncodedCert, certSize)
	// Make a copy because certCtx will be overridden
	certBytes := make([]byte, certSize)
	copy(certBytes, certBytesOrig)

	cert, err := x509.ParseCertificate(certBytes) // TODO: if it's not an x509 cert we may want to use a different parising function
	if err != nil {
		certSubject := unsafe.Slice(certCtx.CertInfo.Subject.Data, certCtx.CertInfo.Subject.Size)
		logrus.Debugf("Failed to parse certificate (subject: %s): %v", certSubject, err)
		return nil
	}
	return cert
}
