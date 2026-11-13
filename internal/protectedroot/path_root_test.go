package protectedroot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPathRoot(t *testing.T) {
	path := "/var/lib/containers/storage/overlay/abc/merged"
	pr, err := NewPathRoot(path)
	assert.NoError(t, err)
	assert.Equal(t, path, pr.path)
}

func TestNewPathRootRejectsEmpty(t *testing.T) {
	_, err := NewPathRoot("")
	assert.Error(t, err)
}

func TestNewOptionalPathRoot(t *testing.T) {
	path := "/var/lib/containers/storage/overlay/abc/merged"
	pr, err := NewOptionalPathRoot(path)
	assert.NoError(t, err)
	require.NotNil(t, pr)
	assert.Equal(t, path, pr.path)
}

func TestNewOptionalPathRootReturnsNilForEmpty(t *testing.T) {
	pr, err := NewOptionalPathRoot("")
	assert.NoError(t, err)
	assert.Nil(t, pr)
}

func TestJoin(t *testing.T) {
	root := t.TempDir()
	pr, err := NewPathRoot(root)
	require.NoError(t, err)

	joined, err := pr.Join("a/b/c")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "a/b/c"), joined)
}

func TestJoinConfinesLexicalEscape(t *testing.T) {
	root := t.TempDir()
	pr, err := NewPathRoot(root)
	require.NoError(t, err)

	escaped, err := pr.Join("../../../../etc/passwd")
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(escaped, root), "Join result %q escaped root %q", escaped, root)
}

func TestJoinConfinesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	pr, err := NewPathRoot(root)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "escape")))

	joined, err := pr.Join("escape/passwd")
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(joined, root), "Join result %q escaped root %q", joined, root)
	assert.False(t, strings.HasPrefix(joined, outside), "Join result %q leaked into %q", joined, outside)
}

func TestJoinPanicsOnUninitializedValue(t *testing.T) {
	var pr PathRoot
	assert.Panics(t, func() { _, _ = pr.Join("a") })
}

func TestPathWithoutProtection(t *testing.T) {
	path := "/var/lib/containers/storage/overlay/abc/merged"
	pr, err := NewPathRoot(path)
	assert.NoError(t, err)
	assert.Equal(t, path, pr.PathWithoutProtection())
}

func TestPathWithoutProtectionPanicsOnUninitializedValue(t *testing.T) {
	var pr PathRoot
	assert.Panics(t, func() { pr.PathWithoutProtection() })
}

func TestFormat(t *testing.T) {
	path := "/var/lib/containers/storage/overlay/abc/merged"
	pr, err := NewPathRoot(path)
	require.NoError(t, err)

	assert.Equal(t, path, fmt.Sprintf("%s", pr))
	assert.Equal(t, path, fmt.Sprintf("%v", pr))
}

func TestMarshalJSON(t *testing.T) {
	path := "/var/lib/containers/storage/overlay/abc/merged"
	pr, err := NewPathRoot(path)
	require.NoError(t, err)

	data, err := json.Marshal(pr)
	require.NoError(t, err)
	assert.Equal(t, `"`+path+`"`, string(data))
}

func TestUnmarshalJSON(t *testing.T) {
	path := "/var/lib/containers/storage/overlay/abc/merged"

	var pr PathRoot
	require.NoError(t, json.Unmarshal([]byte(`"`+path+`"`), &pr))
	assert.Equal(t, path, pr.PathWithoutProtection())
}

func TestUnmarshalJSONRejectsEmpty(t *testing.T) {
	var pr PathRoot
	assert.Error(t, json.Unmarshal([]byte(`""`), &pr))
}
