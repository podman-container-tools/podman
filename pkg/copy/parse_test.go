package copy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSourceAndDestination(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		destination string
		srcCtr      string
		srcPath     string
		destCtr     string
		destPath    string
	}{
		{
			name:        "container source, local destination",
			source:      "ctr:/etc/hostname",
			destination: "/tmp/hostname",
			srcCtr:      "ctr",
			srcPath:     "/etc/hostname",
			destCtr:     "",
			destPath:    "/tmp/hostname",
		},
		{
			name:        "local source, container destination",
			source:      "/tmp/hostname",
			destination: "ctr:/etc/hostname",
			srcCtr:      "",
			srcPath:     "/tmp/hostname",
			destCtr:     "ctr",
			destPath:    "/etc/hostname",
		},
		{
			name:        "colon in a path starting with a dot is not a container",
			source:      "./weird:name",
			destination: "ctr:/dst",
			srcCtr:      "",
			srcPath:     "./weird:name",
			destCtr:     "ctr",
			destPath:    "/dst",
		},
		{
			name:        "colon in a path starting with a slash is not a container",
			source:      "/abs/weird:name",
			destination: "ctr:/dst",
			srcCtr:      "",
			srcPath:     "/abs/weird:name",
			destCtr:     "ctr",
			destPath:    "/dst",
		},
		{
			name:        "relative path without a colon has no container",
			source:      "relative/path",
			destination: "ctr:/dst",
			srcCtr:      "",
			srcPath:     "relative/path",
			destCtr:     "ctr",
			destPath:    "/dst",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcCtr, srcPath, destCtr, destPath, err := ParseSourceAndDestination(tt.source, tt.destination)
			require.NoError(t, err)
			assert.Equal(t, tt.srcCtr, srcCtr, "source container")
			assert.Equal(t, tt.srcPath, srcPath, "source path")
			assert.Equal(t, tt.destCtr, destCtr, "destination container")
			assert.Equal(t, tt.destPath, destPath, "destination path")
		})
	}
}

func TestParseSourceAndDestinationErrors(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		destination string
	}{
		{
			name:        "missing source path",
			source:      "ctr:",
			destination: "/tmp/hostname",
		},
		{
			name:        "missing destination path",
			source:      "/tmp/hostname",
			destination: "ctr:",
		},
		{
			name:        "both empty",
			source:      "",
			destination: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := ParseSourceAndDestination(tt.source, tt.destination)
			assert.Error(t, err)
		})
	}
}
