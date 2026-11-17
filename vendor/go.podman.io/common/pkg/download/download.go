package download

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Options holds named options for FromURL.
type Options struct {
	// If not nil, may contain TLS _algorithm_ options (e.g. TLS version, cipher suites, “curves”, etc.).
	BaseTLSConfig *tls.Config
}

// FromURL downloads the specified source to a file in tmpdir (OS defaults if
// empty).
func FromURL(ctx context.Context, tmpdir, source string, options Options) (string, error) {
	tmp, err := os.CreateTemp(tmpdir, "")
	if err != nil {
		return "", fmt.Errorf("creating temporary download file: %w", err)
	}
	defer tmp.Close()
	succeeded := false
	defer func() {
		if !succeeded {
			os.Remove(tmp.Name())
		}
	}()

	// nil means http.DefaultTransport.
	// This variable must have type http.RoundTripper, not *http.Transport, to avoid https://go.dev/doc/faq#nil_error .
	var transport http.RoundTripper
	if options.BaseTLSConfig != nil {
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return "", errors.New("internal error: http.DefaultTransport is not a *http.Transport")
		}
		t := defaultTransport.Clone()
		t.TLSClientConfig = options.BaseTLSConfig
		defer t.CloseIdleConnections()
		transport = t
	}
	client := &http.Client{
		Transport: transport,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return "", fmt.Errorf("preparing to download %q: %w", source, err)
	}
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", source, err)
	}
	defer response.Body.Close()

	_, err = io.Copy(tmp, response.Body)
	if err != nil {
		return "", fmt.Errorf("copying %s to %s: %w", source, tmp.Name(), err)
	}

	succeeded = true
	return tmp.Name(), nil
}
