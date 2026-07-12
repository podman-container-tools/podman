//go:build !remote && (linux || freebsd)

package abi

import "testing"

func TestValidateApplicationName(t *testing.T) {
	tests := []struct {
		name        string
		baseDir     string
		application string
		wantErr     bool
	}{
		{
			name:        "valid simple name",
			baseDir:     "/opt/apps",
			application: "myapp",
			wantErr:     false,
		},
		{
			name:        "valid with dots",
			baseDir:     "/opt/apps",
			application: "my.app.v1",
			wantErr:     false,
		},
		{
			name:        "empty application",
			baseDir:     "/opt/apps",
			application: "",
			wantErr:     true,
		},
		{
			name:        "contains slash",
			baseDir:     "/opt/apps",
			application: "foo/bar",
			wantErr:     true,
		},
		{
			name:        "contains traversal",
			baseDir:     "/opt/apps",
			application: "../etc",
			wantErr:     true,
		},
		{
			name:        "is dot",
			baseDir:     "/opt/apps",
			application: ".",
			wantErr:     true,
		},
		{
			name:        "is dot dot",
			baseDir:     "/opt/apps",
			application: "..",
			wantErr:     true,
		},
		{
			name:        "base dir not absolute",
			baseDir:     "relative/path",
			application: "myapp",
			wantErr:     true,
		},
		{
			name:        "attempt to escape base dir (defense check)",
			baseDir:     "/opt/apps",
			application: "../../etc",
			wantErr:     true,
		},
		{
			name:        "prefix edge case (similar prefix but not subdir)",
			baseDir:     "/opt/app",
			application: "sneaky", // results in /opt/app/sneaky (valid)
			wantErr:     false,
		},
		{
			name:        "name ending with a quadlet extension is invalid",
			baseDir:     "/opt/apps",
			application: "myapp.container",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApplicationName(tt.baseDir, tt.application)

			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}
