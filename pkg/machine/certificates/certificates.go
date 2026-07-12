package certificates

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/internal/localapi"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

const (
	// CertificatesFileName is the name of the PEM file containing the host
	// trusted CA certificates
	CertificatesFileName = "host-ca-certs.pem"
	// GuestAnchorsPath is the Fedora folder where the command `update-ca-trust`
	// looks for users-defined trusted certificates
	GuestAnchorsPath = "/etc/pki/ca-trust/source/anchors"
	// UpdateCATrustCommand is the Fedora command to update the system store
	UpdateCATrustCommand = "update-ca-trust"
)

// ImportNativeCertificates imports the host's trusted CA certificates into the
// guest machine OS trust store. To do that it:
// - extract all certificates from the host trust store
// - export all the certificates in a single file in the machine data folder
// - check if the file is already mounted in the guest or transfer via SCP
// - update the guest trust store to include the certificates
func ImportNativeCertificates(mc *vmconfigs.MachineConfig, vmType define.VMType) error {
	logrus.Debugf("Importing the host CA certificates into machine %q", mc.Name)
	// Extract certificates from the host system store
	certs := deduplicateCertificates(extractHostCertificates())
	if len(certs) == 0 {
		logrus.Debugf("No native CA certificates found to import")
		return nil
	}
	logrus.Debugf("Extracted %d host certificates from the system store", len(certs))
	// Save the certificates to a PEM file in the machine data folder
	certFilePath, err := saveCertificatesToFile(mc, certs)
	if err != nil {
		return fmt.Errorf("failed to create certs file in machine data folder: %s", err)
	}
	logrus.Debugf("Saved the certificates to file %q", certFilePath)
	// Copy or transfer via SCP the file with the certificates to the anchors
	// folder in the guest
	err = copyOrTransferFileToGuestAnchorsFolder(mc, vmType, certFilePath)
	if err != nil {
		return fmt.Errorf("failed to transfer or copy the certs file in the guest anchors folder: %s", err)
	}
	// Update the CA trust list
	if err := runUpdateCATrustInGuest(mc); err != nil {
		return fmt.Errorf("failed to update CA trust in guest: %w", err)
	}
	logrus.Debugf("Successfully imported the host trusted certificates into machine %q", mc.Name)
	return nil
}

// saveCertificatesToFile saves the certificates to a PEM file in the machine data directory.
func saveCertificatesToFile(mc *vmconfigs.MachineConfig, certs []*x509.Certificate) (string, error) {
	dataDir, err := mc.DataDir()
	if err != nil {
		return "", fmt.Errorf("failed to get machine data directory: %w", err)
	}
	// Create the directory if it doesn't exist
	certFileDirPath := filepath.Join(dataDir.GetPath(), mc.Name)
	if err = os.MkdirAll(certFileDirPath, 0o755); err != nil {
		return "", fmt.Errorf("failed to create certificate directory: %w", err)
	}
	// Save certificates to PEM file in the directory
	certFilePath := filepath.Join(certFileDirPath, CertificatesFileName)
	if err := saveCertificatesToPEM(certs, certFilePath); err != nil {
		return "", fmt.Errorf("failed to save certificates to PEM file: %w", err)
	}
	return certFilePath, nil
}

// copyOrTransferFileToGuestAnchorsFolder copies or transfers hostFilePath to
// the guest anchor folder, depending on whether the file is already mounted in
// the guest or not.
func copyOrTransferFileToGuestAnchorsFolder(mc *vmconfigs.MachineConfig, vmType define.VMType, hostFilePath string) error {
	// Look for the file in the machine mounts
	mounts := mc.Mounts
	if localMap, ok := localapi.IsPathAvailableOnMachine(mounts, vmType, hostFilePath); ok {
		// Copy the mounted file to the guest anchors folder
		remotePath := localMap.RemotePath
		logrus.Debugf("The certificates file is already mounted in the guest (%q), copy it to the anchors folder", remotePath)
		return copyFileToGuestAnchorsFolder(mc, remotePath)
	}
	// Transfer the certificate file to the guest OS
	logrus.Debugf("The certificates file isn't mounted in the guest, transfer it via SCP.")
	return machine.LocalhostSSHCopy(
		"root", // need root to copy to /etc/pki/ca-trust/source/anchors/
		mc.SSH.IdentityPath,
		mc.SSH.Port,
		hostFilePath,
		GuestAnchorsPath,
		false,
		true)
}

// copyFileToGuestAnchorsFolder runs `sudo cp` in ghe guest to copy guestFilePath
// to /etc/pki/ca-trust/source/anchors/
func copyFileToGuestAnchorsFolder(mc *vmconfigs.MachineConfig, guestFilePath string) error {
	logrus.Debugf("Running %q in guest", UpdateCATrustCommand)
	return machine.LocalhostSSHSilent(
		mc.SSH.RemoteUsername,
		mc.SSH.IdentityPath,
		mc.Name,
		mc.SSH.Port,
		[]string{"sudo", "cp", guestFilePath, GuestAnchorsPath},
	)
}

// saveCertificatesToPEM exports the certificates in certs to a PEM file
func saveCertificatesToPEM(certs []*x509.Certificate, certsFilePath string) error {
	certsFile, err := os.Create(certsFilePath)
	if err != nil {
		return err
	}
	defer certsFile.Close()
	for _, cert := range certs {
		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		}
		if err := pem.Encode(certsFile, block); err != nil {
			return err
		}
	}
	return nil
}

// runUpdateCATrustInGuest runs the update-ca-trust command in the guest OS
func runUpdateCATrustInGuest(mc *vmconfigs.MachineConfig) error {
	logrus.Debugf("Running %q in guest", UpdateCATrustCommand)
	return machine.LocalhostSSHSilent(
		mc.SSH.RemoteUsername,
		mc.SSH.IdentityPath,
		mc.Name,
		mc.SSH.Port,
		[]string{"sudo", UpdateCATrustCommand},
	)
}

// deduplicateCertificates removes duplicate certificates from a slice of certificates.
func deduplicateCertificates(certs []*x509.Certificate) []*x509.Certificate {
	seen := make(map[string]bool)
	var unique []*x509.Certificate
	for _, cert := range certs {
		if cert == nil {
			continue
		}
		if seen[string(cert.Signature)] {
			continue
		}
		seen[string(cert.Signature)] = true
		unique = append(unique, cert)
	}
	return unique
}
