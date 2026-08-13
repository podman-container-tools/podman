//go:build !remote && (linux || freebsd)

package compat

import "testing"

func TestIsLocalhostServerAddress(t *testing.T) {
	tests := []struct {
		name          string
		serverAddress string
		want          bool
	}{
		{"bare host with port", "localhost:5000", true},
		{"https URL with port", "https://localhost:5000", true},
		{"https URL with trailing slash", "https://localhost/", true},
		{"https URL with path", "https://localhost:5000/v2/", true},
		{"https URL, no port or path", "https://localhost", true},
		{"bare host, no port", "localhost", true},
		{"case-insensitive host", "https://LOCALHOST:5000", true},
		{"case-insensitive bare host", "LOCALHOST:5000", true},

		// The actual bug: a crafted userinfo section that merely
		// starts with "https://localhost:" must not be confused with
		// the "localhost" host.
		{"userinfo bypass with scheme", "https://localhost:password@evil.example", false},
		{"userinfo bypass without scheme", "localhost:password@evil.example", false},
		{"userinfo bypass with path", "https://localhost:1234@evil.example/v2/", false},

		{"unrelated https host", "https://example.com", false},
		{"unrelated bare host", "example.com:5000", false},
		{"subdomain lookalike", "https://localhost.evil.example", false},
		{"query string lookalike", "https://evil.example/?x=localhost", false},
		{"http scheme not treated as insecure-allowed", "http://localhost:5000", false},
		{"empty address", "", false},
		{"loopback IP is not localhost by name", "https://127.0.0.1:5000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLocalhostServerAddress(tt.serverAddress)
			if got != tt.want {
				t.Errorf("isLocalhostServerAddress(%q) = %v, want %v", tt.serverAddress, got, tt.want)
			}
		})
	}
}
