//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSubDir(t *testing.T) {
	assert.True(t, isSubDir("/foo", "/foo"))
	assert.True(t, isSubDir("/foo/bar", "/foo"))
	assert.True(t, isSubDir("/foo/bar", "/foo/"))
	assert.True(t, isSubDir("/foo/bar", "/foo//"))
	assert.True(t, isSubDir("/foo/bar/", "/foo"))
	assert.True(t, isSubDir("/foo/bar/baz/", "/foo"))
	assert.True(t, isSubDir("/foo/bar/baz/", "/foo/bar"))
	assert.True(t, isSubDir("/foo/bar/baz/", "/foo/bar/"))
	assert.False(t, isSubDir("/foo/bar/baz/", "/foobar/"))
	assert.False(t, isSubDir("/foo/bar/baz/../../", "/foobar/"))
	assert.False(t, isSubDir("/foo/bar/baz/", "../foo/bar"))
	assert.False(t, isSubDir("/foo/bar/baz/", "../foo/"))
	assert.False(t, isSubDir("/foo/bar/baz/", "../foo"))
	assert.False(t, isSubDir("/", ".."))
	assert.False(t, isSubDir("//", ".."))
	assert.False(t, isSubDir("//", "../"))
	assert.False(t, isSubDir("//", "..//"))
	assert.True(t, isSubDir("/foo/bar/baz/../../", "/foo/"))
}
