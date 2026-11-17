//go:build !remote

package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDevice(t *testing.T) {
	tests := []struct {
		device string
		src    string
		dst    string
		perm   string
	}{
		{"/dev/foo", "/dev/foo", "/dev/foo", "rwm"},
		{"/dev/foo:/dev/bar", "/dev/foo", "/dev/bar", "rwm"},
		{"/dev/foo:/dev/bar:rw", "/dev/foo", "/dev/bar", "rw"},
		{"/dev/foo:rw", "/dev/foo", "/dev/foo", "rw"},
		{"/dev/foo::rw", "/dev/foo", "/dev/foo", "rw"},
		{"/dev/foo:", "/dev/foo", "/dev/foo", "rwm"},
	}
	for _, test := range tests {
		src, dst, perm, err := ParseDevice(test.device)
		assert.NoError(t, err)
		assert.Equal(t, src, test.src)
		assert.Equal(t, dst, test.dst)
		assert.Equal(t, perm, test.perm)
	}
}

func TestParseDeviceErrors(t *testing.T) {
	errorTests := []struct {
		device        string
		expectedError string
	}{
		{"/dev/fuse::", "empty device mode in device specification: /dev/fuse::"},
		{"/dev/fuse:invalid", `invalid device mode "invalid" in device "/dev/fuse:invalid"`},
		{"/dev/fuse:/path:xyz", `invalid device mode "xyz" in device "/dev/fuse:/path:xyz"`},
		{"/dev/fuse:/path:rw:extra", `invalid device specification: /dev/fuse:/path:rw:extra`},
		{"/dev/fuse:/path:rw:extra:more", `invalid device specification: /dev/fuse:/path:rw:extra:more`},
		{"/dev/fuse:notapath", `invalid device mode "notapath" in device "/dev/fuse:notapath"`},
		{"/dev/fuse:x", `invalid device mode "x" in device "/dev/fuse:x"`},
		{"/dev/fuse:rwx", `invalid device mode "rwx" in device "/dev/fuse:rwx"`},
		{"/dev/fuse:rrw", `invalid device mode "rrw" in device "/dev/fuse:rrw"`},
	}

	for _, test := range errorTests {
		_, _, _, err := ParseDevice(test.device)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), test.expectedError)
	}
}
