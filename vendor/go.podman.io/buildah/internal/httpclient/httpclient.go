package httpclient

import (
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/docker/go-connections/tlsconfig"
	"go.podman.io/image/v5/pkg/tlsclientconfig"
	"go.podman.io/image/v5/types"
)

// URLOptions provides client-side options used by ForURLOptions().
type URLOptions struct {
	// Location of CA certificates, if not the system default
	CertPath              string
	InsecureSkipTLSVerify types.OptionalBool
	// Callback which controls which, if any, proxy server to use when retrieving HTTP or
	// HTTPS sources.  Used to construct an http.Client's Transport.
	Proxy func(*http.Request) (*url.URL, error)
}

// ForURLOptions returns an http.Client with the settings from `options`
// applied to its mostly-default transport, configured to use proxy settings
// from the environment.
func ForURLOptions(options URLOptions) (*http.Client, error) {
	tlsClientConfig := &tls.Config{
		// As of 2025-08, tlsconfig.ClientDefault() differs from Go 1.23 defaults only in CipherSuites;
		// so, limit us to only using that value. If go-connections/tlsconfig changes its policy, we
		// will want to consider that and make a decision whether to follow suit.
		// There is some chance that eventually the Go default will be to require TLS 1.3, and that point
		// we might want to drop the dependency on go-connections entirely.
		CipherSuites: tlsconfig.ClientDefault().CipherSuites,
	}
	if err := tlsclientconfig.SetupCertificates(options.CertPath, tlsClientConfig); err != nil {
		return nil, err
	}
	tlsClientConfig.InsecureSkipVerify = options.InsecureSkipTLSVerify == types.OptionalBoolTrue

	tr := &http.Transport{
		TLSClientConfig: tlsClientConfig,
		Proxy:           options.Proxy,
	}
	httpClient := &http.Client{Transport: tr}
	return httpClient, nil
}
