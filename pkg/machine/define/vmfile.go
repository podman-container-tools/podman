package define

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

const MaxSocketPathLength int = 103

type VMFile struct {
	// Path is the fully qualified path to a file
	Path string
	// Symlink is a shortened version of Path by using
	// a symlink
	Symlink *string `json:"symlink,omitempty"`
}

// GetPath returns the working path for a machinefile.  it returns
// the symlink unless one does not exist
func (m *VMFile) GetPath() string {
	if m.Symlink == nil {
		return m.Path
	}
	return *m.Symlink
}

// Delete removes the machinefile symlink (if it exists) and
// the actual path
func (m *VMFile) Delete() error {
	if m.Symlink != nil {
		if err := os.Remove(*m.Symlink); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Errorf("unable to remove symlink %q", *m.Symlink)
		}
	}
	if err := os.Remove(m.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Read the contents of a given file and return in []bytes
func (m *VMFile) Read() ([]byte, error) {
	return os.ReadFile(m.GetPath())
}

// Read the first n bytes of a given file and return in []bytes
func (m *VMFile) ReadMagicNumber(n int) ([]byte, error) {
	f, err := os.Open(m.GetPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := make([]byte, n)
	n, err = io.ReadFull(f, b)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return b[:n], err
	} else {
		return b[:n], nil
	}
}

// ReadPIDFrom a file and return as int. -1 means the pid file could not
// be read or had something that could not be converted to an int in it
func (m *VMFile) ReadPIDFrom() (int, error) {
	vmPidString, err := m.Read()
	if err != nil {
		return -1, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(vmPidString)))
	if err != nil {
		return -1, err
	}

	// Not returning earlier because -1 means something
	return pid, nil
}

// NewMachineFile is a constructor for VMFile
func NewMachineFile(path string, symlink *string) (*VMFile, error) {
	if len(path) < 1 {
		return nil, errors.New("invalid machine file path")
	}
	if symlink != nil && len(*symlink) < 1 {
		return nil, errors.New("invalid symlink path")
	}
	mf := VMFile{Path: path}
	logrus.Debugf("socket length for %s is %d", path, len(path))
	if symlink != nil && len(path) > MaxSocketPathLength {
		if err := mf.makeSymlink(symlink); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return &mf, nil
}

// makeSymlink for macOS creates a symlink in $HOME/.podman/
// for a machinefile like a socket
func (m *VMFile) makeSymlink(symlink *string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sl := filepath.Join(homeDir, ".podman", *symlink)
	// make the symlink dir and throw away if it already exists
	if err := os.MkdirAll(filepath.Dir(sl), 0o700); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m.Symlink = &sl
	return os.Symlink(m.Path, sl)
}

// AppendToNewVMFile takes a given path and appends it to the existing vmfile path.  The new
// VMFile is returned
func (m *VMFile) AppendToNewVMFile(additionalPath string, symlink *string) (*VMFile, error) {
	return NewMachineFile(filepath.Join(m.Path, additionalPath), symlink)
}
