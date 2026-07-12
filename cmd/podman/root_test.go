package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/storage"
)

func TestFormatError(t *testing.T) {
	err := errors.New("unknown error")
	output := formatError(err, nil)
	expected := fmt.Sprintf("Error: %v", err)

	if output != expected {
		t.Errorf("Expected \"%s\" to equal \"%s\"", output, err.Error())
	}
}

func TestFormatErrorDuplicateName(t *testing.T) {
	withReplace := &cobra.Command{Use: "create"}
	withReplace.Flags().Bool("replace", false, "")
	withoutReplace := &cobra.Command{Use: "create"}

	err := fmt.Errorf("that name is already in use: %w", storage.ErrDuplicateName)
	hint := "or use --replace to instruct Podman to do so."

	tests := []struct {
		name     string
		cmd      *cobra.Command
		wantHint bool
	}{
		{name: "command with --replace flag", cmd: withReplace, wantHint: true},
		{name: "command without --replace flag", cmd: withoutReplace, wantHint: false},
		{name: "no command", cmd: nil, wantHint: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatError(err, tt.cmd)
			if got := strings.Contains(output, hint); got != tt.wantHint {
				t.Errorf("formatError() = %q, want hint contained: %v", output, tt.wantHint)
			}
			if !strings.Contains(output, err.Error()) {
				t.Errorf("formatError() = %q, want the original error message included", output)
			}
		})
	}
}

func TestIndentExamples(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "flush-left lines get indented",
			input:    "podman top ctrID\npodman top ctrID pid seccomp",
			expected: "  podman top ctrID\n  podman top ctrID pid seccomp",
		},
		{
			name:     "preserves empty lines between examples",
			input:    "podman run alpine\n\npodman run busybox",
			expected: "  podman run alpine\n\n  podman run busybox",
		},
		{
			name:     "handles comment lines",
			input:    "# List connections\npodman system connection ls",
			expected: "  # List connections\n  podman system connection ls",
		},
		{
			name:     "single line",
			input:    "podman run alpine",
			expected: "  podman run alpine",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := indentExamples(tt.input); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatOCIError(t *testing.T) {
	expectedPrefix := "Error: "
	expectedSuffix := "OCI runtime output"
	err := fmt.Errorf("%s: %w", expectedSuffix, define.ErrOCIRuntime)
	output := formatError(err, nil)

	if !strings.HasPrefix(output, expectedPrefix) {
		t.Errorf("Expected \"%s\" to start with \"%s\"", output, expectedPrefix)
	}
	if !strings.HasSuffix(output, expectedSuffix) {
		t.Errorf("Expected \"%s\" to end with \"%s\"", output, expectedSuffix)
	}
}
