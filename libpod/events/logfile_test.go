//go:build linux || freebsd

package events

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRotateLog(t *testing.T) {
	tests := []struct {
		// If sizeInitial + sizeContent >= sizeLimit, then rotate
		sizeInitial uint64
		sizeContent uint64
		sizeLimit   uint64
		mustRotate  bool
	}{
		// No rotation
		{0, 0, 1, false},
		{1, 1, 0, false},
		{10, 10, 30, false},
		{1000, 500, 1600, false},
		// Rotation
		{10, 10, 20, true},
		{30, 0, 29, true},
		{200, 50, 150, true},
		{1000, 600, 1500, true},
	}

	tmpDir := t.TempDir()
	for _, test := range tests {
		tmp, err := os.CreateTemp(tmpDir, "log-rotation-")
		require.NoError(t, err)
		defer tmp.Close()

		// Create dummy file and content.
		initialContent := make([]byte, test.sizeInitial)
		logContent := make([]byte, test.sizeContent)

		// Write content to the file.
		_, err = tmp.Write(initialContent)
		require.NoError(t, err)

		// Now rotate
		fInfoBeforeRotate, err := tmp.Stat()
		require.NoError(t, err)
		isRotated, err := rotateLog(tmp.Name(), string(logContent), test.sizeLimit)
		require.NoError(t, err)

		fInfoAfterRotate, err := os.Stat(tmp.Name())
		// Test if rotation was successful
		if test.mustRotate {
			// File has been renamed
			require.True(t, isRotated)
			require.NoError(t, err, "log file has been renamed")
			require.NotEqual(t, fInfoBeforeRotate.Size(), fInfoAfterRotate.Size())
		} else {
			// File has not been renamed
			require.False(t, isRotated)
			require.NoError(t, err, "log file has not been renamed")
			require.Equal(t, fInfoBeforeRotate.Size(), fInfoAfterRotate.Size())
		}
	}
}

func TestTruncationOutput(t *testing.T) {
	contentBefore := `0
1
2
3
4
5
6
7
8
9
10
`
	// Create dummy file
	tmp, err := os.CreateTemp(t.TempDir(), "log-rotation")
	require.NoError(t, err)
	defer tmp.Close()

	// Write content before truncation to dummy file
	_, err = tmp.WriteString(contentBefore)
	require.NoError(t, err)

	// Truncate the file
	beforeTruncation, err := os.ReadFile(tmp.Name())
	require.NoError(t, err)
	err = truncate(tmp.Name())
	require.NoError(t, err)
	afterTruncation, err := os.ReadFile(tmp.Name())
	require.NoError(t, err)
	// Content has changed
	require.NotEqual(t, beforeTruncation, afterTruncation)
	split := strings.Split(string(afterTruncation), "\n")
	require.Len(t, split, 8) // 2 events + 5 rotated lines + last new line
	require.Contains(t, split[0], "\"Attributes\":{\"io.podman.event.rotate\":\"begin\"}")
	require.Equal(t, split[1:6], []string{"6", "7", "8", "9", "10"})
	require.Contains(t, split[6], "\"Attributes\":{\"io.podman.event.rotate\":\"end\"}")
	require.Contains(t, split[7], "")
}

func TestRenameLog(t *testing.T) {
	fileContent := `0
1
2
3
4
5
`
	tmpDir := t.TempDir()
	// Create two dummy files
	source, err := os.CreateTemp(tmpDir, "removing")
	require.NoError(t, err)
	target, err := os.CreateTemp(tmpDir, "renaming")
	require.NoError(t, err)

	// Write to source dummy file
	_, err = source.WriteString(fileContent)
	require.NoError(t, err)

	// Rename the files
	beforeRename, err := os.ReadFile(source.Name())
	require.NoError(t, err)
	err = renameLog(source.Name(), target.Name())
	require.NoError(t, err)
	afterRename, err := os.ReadFile(target.Name())
	require.NoError(t, err)

	// Test if renaming was successful
	require.Error(t, os.Remove(source.Name()))
	require.NoError(t, os.Remove(target.Name()))
	require.Equal(t, beforeRename, afterRename)
}
