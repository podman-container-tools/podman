//go:build !remote && (linux || freebsd)

package libpod

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnotateGvproxyResponseError(t *testing.T) {
	const permDenied = "bind: permission denied"

	tests := []struct {
		name string
		body string
		// local is the host-side "ip:port" gvproxy was asked to bind.
		local string
		// wantHint is true when the macOS/gvproxy privileged-port hint is expected.
		wantHint bool
	}{
		{"privileged port on a specific IP returns the gvproxy hint", permDenied, "192.168.1.5:80", true},
		{"privileged port on an IPv6 address returns the gvproxy hint", permDenied, "[fe80::1]:443", true},
		{"privileged port without a host IP stays generic", permDenied, ":80", false},
		{"privileged port on 0.0.0.0 stays generic", permDenied, "0.0.0.0:80", false},
		{"privileged port on the IPv6 unspecified address stays generic", permDenied, "[::]:80", false},
		{"unprivileged port on a specific IP stays generic", permDenied, "192.168.1.5:8080", false},
		{"unrelated error on a specific privileged port stays generic", "some other failure", "192.168.1.5:80", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := annotateGvproxyResponseError(strings.NewReader(tt.body), machineExpose{Local: tt.local})
			require.Error(t, err)
			assert.Equal(t, tt.wantHint, strings.Contains(err.Error(), "gvproxy"))
			if tt.wantHint {
				// the hint must carry the actionable guidance, not just the word "gvproxy"
				assert.Contains(t, err.Error(), "without a host IP")
			}
			// The raw gvproxy response body is always preserved for debugging.
			assert.Contains(t, err.Error(), tt.body)
		})
	}
}

func TestAnnotateGvproxyResponseErrorEmptyBody(t *testing.T) {
	err := annotateGvproxyResponseError(strings.NewReader(""), machineExpose{Local: "192.168.1.5:80"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not read response")
}
