//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// multiQuadletSrc is a .quadlets file with two quadlet sections and
// no trailing newline.
const multiQuadletSrc = "# FileName=one\n[Container]\nContainerName=one\nImage=localhost/one:latest\n\n---\n\n# FileName=two\n[Container]\nContainerName=two\nImage=localhost/two:latest"

func TestParseMultiQuadletFileTrailingNewline(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
	}{
		{name: "source ends with newline", source: multiQuadletSrc + "\n"},
		{name: "source without trailing newline", source: multiQuadletSrc},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srcFile := filepath.Join(t.TempDir(), "t.quadlets")
			require.NoError(t, os.WriteFile(srcFile, []byte(tt.source), 0o644))

			sections, err := parseMultiQuadletFile(srcFile)
			require.NoError(t, err)
			require.Len(t, sections, 2)

			for _, section := range sections {
				assert.True(t, strings.HasSuffix(section.content, "\n"),
					"section %q does not end with a newline", section.name)
				assert.False(t, strings.HasSuffix(section.content, "\n\n"),
					"section %q ends with more than one newline", section.name)
			}
		})
	}
}

func TestMultiQuadletFileInstallTrailingNewline(t *testing.T) {
	// The parser trims each section and adds a trailing newline, so the
	// installed files end up identical regardless of whether the source
	// ends with a newline; test both to pin the behavior for source
	// files that lack a trailing newline.
	for _, tt := range []struct {
		name   string
		source string
	}{
		{name: "source ends with newline", source: multiQuadletSrc + "\n"},
		{name: "source without trailing newline", source: multiQuadletSrc},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srcFile := filepath.Join(t.TempDir(), "t.quadlets")
			require.NoError(t, os.WriteFile(srcFile, []byte(tt.source), 0o644))

			sections, err := parseMultiQuadletFile(srcFile)
			require.NoError(t, err)
			require.Len(t, sections, 2)

			installDir := t.TempDir()
			ic := &ContainerEngine{}
			for _, section := range sections {
				dest, err := installMultiQuadletSection(context.Background(), ic, section, installDir, false)
				require.NoError(t, err)

				data, err := os.ReadFile(dest)
				require.NoError(t, err)
				assert.True(t, strings.HasSuffix(string(data), "\n"),
					"installed %q does not end with a newline", section.name)
				assert.False(t, strings.HasSuffix(string(data), "\n\n"),
					"installed %q ends with more than one newline", section.name)
			}
		})
	}
}
