package certificates

var certBundlePaths = []string{
	"/etc/ssl/certs/ca-certificates.crt", // Debian/Ubuntu
	"/etc/pki/tls/certs/ca-bundle.crt",   // RHEL/Fedora/CentOS
	"/etc/ssl/ca-bundle.pem",             // OpenSUSE
	"/etc/pki/tls/cacert.pem",            // OpenELEC
	"/etc/ssl/cert.pem",                  // Alpine Linux
}
