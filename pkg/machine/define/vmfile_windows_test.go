package define

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/storage/pkg/ioutils"
)

// TestVMFileReplaceWhileOpen checks that a file held open the way Read
// opens it can still be replaced by ioutils.AtomicWriteFile.
// See https://github.com/podman-container-tools/podman/issues/28873
func TestVMFileReplaceWhileOpen(t *testing.T) {
	m := VMFile{Path: filepath.Join(t.TempDir(), "machine.json")}
	require.NoError(t, os.WriteFile(m.GetPath(), []byte("old"), 0o644))

	reader, err := openShareDelete(m.GetPath())
	require.NoError(t, err)
	defer reader.Close()

	require.NoError(t, ioutils.AtomicWriteFile(m.GetPath(), []byte("new"), 0o644))

	got, err := m.Read()
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))

	old, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "old", string(old), "open reader should keep the replaced content")
}

// TestVMFileReadNotExist checks that Read of a missing file returns an
// *os.PathError that wraps os.ErrNotExist, like os.ReadFile. Callers such as
// LoadMachineByName rely on errors.Is(err, fs.ErrNotExist), and the path makes
// the error message useful.
func TestVMFileReadNotExist(t *testing.T) {
	m := VMFile{Path: filepath.Join(t.TempDir(), "missing.json")}
	_, err := m.Read()
	require.ErrorIs(t, err, os.ErrNotExist)

	var pathErr *os.PathError
	require.ErrorAs(t, err, &pathErr)
	assert.Equal(t, m.GetPath(), pathErr.Path)
}
