//go:build linux

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/systemd/parser"
)

func TestLogfWritesToStderrWhenKmsgUnavailable(t *testing.T) {
	restoreLogGlobals(t)
	noKmsg = true
	kmsgFile = nil
	dryRunFlag = false

	stderr := captureStderr(t, func() {
		Logf("kmsg unavailable")
	})

	assert.Equal(t, expectedLogLine("kmsg unavailable")+"\n", stderr)
}

func TestLogfWritesToStderrWhenKmsgSucceeds(t *testing.T) {
	restoreLogGlobals(t)
	noKmsg = false
	dryRunFlag = false

	tmpFile, err := os.CreateTemp(t.TempDir(), "kmsg")
	require.NoError(t, err)
	t.Cleanup(func() {
		tmpFile.Close()
	})
	kmsgFile = tmpFile

	stderr := captureStderr(t, func() {
		Logf("kmsg succeeds")
	})

	line := expectedLogLine("kmsg succeeds")
	assert.Equal(t, line+"\n", stderr)

	_, err = tmpFile.Seek(0, io.SeekStart)
	require.NoError(t, err)
	kmsg, err := io.ReadAll(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, line, string(kmsg))
}

func TestLogfWritesToStderrInDryRun(t *testing.T) {
	restoreLogGlobals(t)
	noKmsg = true
	kmsgFile = nil
	dryRunFlag = true

	stderr := captureStderr(t, func() {
		Logf("dry run")
	})

	assert.Equal(t, expectedLogLine("dry run")+"\n", stderr)
}

func TestLogfWritesToStderrWhenKmsgWriteFails(t *testing.T) {
	restoreLogGlobals(t)
	noKmsg = false
	dryRunFlag = false

	tmpFile, err := os.CreateTemp(t.TempDir(), "kmsg")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	kmsgFile = tmpFile

	stderr := captureStderr(t, func() {
		Logf("kmsg write failure")
	})

	assert.Equal(t, expectedLogLine("kmsg write failure")+"\n", stderr)
	assert.Nil(t, kmsgFile)
}

func restoreLogGlobals(t *testing.T) {
	t.Helper()

	oldNoKmsg := noKmsg
	oldKmsgFile := kmsgFile
	oldDryRunFlag := dryRunFlag

	t.Cleanup(func() {
		noKmsg = oldNoKmsg
		kmsgFile = oldKmsgFile
		dryRunFlag = oldDryRunFlag
	})
}

func captureStderr(t *testing.T, f func()) string {
	t.Helper()

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stderr = writer
	defer func() {
		os.Stderr = oldStderr
	}()

	f()
	require.NoError(t, writer.Close())
	os.Stderr = oldStderr

	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	return string(output)
}

func expectedLogLine(message string) string {
	return fmt.Sprintf("quadlet-generator[%d]: %s", os.Getpid(), message)
}

func TestIsUnambiguousName(t *testing.T) {
	tests := []struct {
		input string
		res   bool
	}{
		// Ambiguous names
		{"fedora", false},
		{"fedora:latest", false},
		{"library/fedora", false},
		{"library/fedora:latest", false},
		{"busybox@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", false},
		{"busybox:latest@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", false},
		{"d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05", false},
		{"d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05aa", false},

		// Unambiguous names
		{"quay.io/fedora", true},
		{"docker.io/fedora", true},
		{"docker.io/library/fedora:latest", true},
		{"localhost/fedora", true},
		{"localhost:5000/fedora:latest", true},
		{"example.foo.this.may.be.garbage.but.maybe.not:1234/fedora:latest", true},
		{"docker.io/library/busybox@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
		{"docker.io/library/busybox:latest@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
		{"docker.io/fedora@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
		{"sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
		{"d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
	}

	for _, test := range tests {
		res := isUnambiguousName(test.input)
		assert.Equal(t, res, test.res, "%q", test.input)
	}
}

func TestLoadUnitDropinsSymlink(t *testing.T) {
	// Create a target directory simulating /etc/containers/systemd
	targetDir := t.TempDir()
	targetUnitPath := filepath.Join(targetDir, "test@.container")
	targetDropinDir := filepath.Join(targetDir, "test@.container.d")
	require.NoError(t, os.MkdirAll(targetDropinDir, 0o755))

	// Base unit definition in target directory
	baseUnitContent := "[Container]\nImage=alpine\n"
	require.NoError(t, os.WriteFile(targetUnitPath, []byte(baseUnitContent), 0o644))

	// Drop-in file in target directory (test@.container.d)
	targetDropinPath := filepath.Join(targetDropinDir, "10-override.conf")
	dropinContent := "[Container]\nEnvironment=FOO=BAR\n"
	require.NoError(t, os.WriteFile(targetDropinPath, []byte(dropinContent), 0o644))

	// Create a user directory simulating ~/.config/containers/systemd
	userDir := t.TempDir()
	userSymlinkPath := filepath.Join(userDir, "test@.container")
	require.NoError(t, os.Symlink(targetUnitPath, userSymlinkPath))

	// Parse the unit via the symlink
	unit, err := parser.ParseUnitFile(userSymlinkPath)
	require.NoError(t, err)

	// In rootless mode, sourcePaths only contains user directories (e.g. userDir)
	sourcePaths := []string{userDir}
	err = loadUnitDropins(unit, sourcePaths)
	require.NoError(t, err)

	// Verify that the drop-in from targetDir's test@.container.d was found and merged
	envVal, ok := unit.Lookup("Container", "Environment")
	assert.True(t, ok, "Expected Container.Environment key to be merged from symlink target's drop-in")
	assert.Equal(t, "FOO=BAR", envVal)
}
