//go:build linux

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestShouldLogToStderr(t *testing.T) {
	tests := []struct {
		name             string
		kmsgOK           bool
		dryRun           bool
		stderrIsTerminal bool
		want             bool
	}{
		{
			name:             "kmsg failed",
			kmsgOK:           false,
			dryRun:           false,
			stderrIsTerminal: false,
			want:             true,
		},
		{
			name:             "dry run",
			kmsgOK:           true,
			dryRun:           true,
			stderrIsTerminal: false,
			want:             true,
		},
		{
			name:             "terminal stderr",
			kmsgOK:           true,
			dryRun:           false,
			stderrIsTerminal: true,
			want:             true,
		},
		{
			name:             "kmsg succeeded and non-terminal stderr",
			kmsgOK:           true,
			dryRun:           false,
			stderrIsTerminal: false,
			want:             false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := shouldLogToStderr(test.kmsgOK, test.dryRun, test.stderrIsTerminal)
			assert.Equal(t, test.want, got)
		})
	}
}
