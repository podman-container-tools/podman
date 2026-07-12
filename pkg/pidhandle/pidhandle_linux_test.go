//go:build linux

package pidhandle

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestNewPIDHandle(t *testing.T) {
	// Mock the pidfdOpen
	original := pidfdOpen
	defer func() { pidfdOpen = original }()
	pidfdOpen = func(_ int, _ int) (int, error) {
		return 255, nil
	}

	// Mock the nameToHandleAt
	original_nameToHandleAt := nameToHandleAt
	defer func() { nameToHandleAt = original_nameToHandleAt }()
	nameToHandleAt = func(_ int, _ string, _ int) (handle unix.FileHandle, mountID int, err error) {
		return newFileHandle(254, []byte("test")), 1, nil
	}

	h, err := NewPIDHandle(os.Getpid())
	assert.NoError(t, err)
	defer h.Close()

	pidData, err := h.String()
	assert.NoError(t, err)
	assert.Equal(t, nameToHandlePrefix+"254 74657374", pidData)
	assert.Equal(t, h.PID(), os.Getpid())
}

func TestNewPIDHandlepidfdOpenNotSupported(t *testing.T) {
	// Mock the pidfdOpen
	original := pidfdOpen
	defer func() { pidfdOpen = original }()
	pidfdOpen = func(_ int, _ int) (int, error) {
		return -1, unix.ENOSYS
	}

	h, err := NewPIDHandle(os.Getpid())
	assert.NoError(t, err)
	defer h.Close()

	pidData, err := h.String()
	assert.NoError(t, err)
	assert.Contains(t, pidData, startTimePrefix)
	assert.Equal(t, h.PID(), os.Getpid())
}

func TestPIDHandleStringnameToHandleAtNotSupported(t *testing.T) {
	// Mock the pidfdOpen
	original := pidfdOpen
	defer func() { pidfdOpen = original }()
	pidfdOpen = func(_ int, _ int) (int, error) {
		return 254, nil
	}

	// Mock the nameToHandleAt
	original_nameToHandleAt := nameToHandleAt
	defer func() { nameToHandleAt = original_nameToHandleAt }()
	nameToHandleAt = func(_ int, _ string, _ int) (handle unix.FileHandle, mountID int, err error) {
		return newFileHandle(-1, []byte("")), 1, unix.ENOTSUP
	}

	h, err := NewPIDHandle(os.Getpid())
	assert.NoError(t, err)
	defer h.Close()

	pidData, err := h.String()
	assert.NoError(t, err)
	assert.Contains(t, pidData, startTimePrefix)
}

func TestNewPIDHandleFromString(t *testing.T) {
	// Mock the pidfdOpen
	original := pidfdOpen
	defer func() { pidfdOpen = original }()
	pidfdOpen = func(_ int, _ int) (int, error) {
		return 254, nil
	}

	// Mock the newFileHandle
	original_newFileHandle := newFileHandle
	defer func() { newFileHandle = original_newFileHandle }()
	newFileHandle = func(_ int32, _ []byte) unix.FileHandle {
		return unix.NewFileHandle(254, []byte("test"))
	}

	// Mock the openByHandleAt
	original_openByHandleAt := openByHandleAt
	defer func() { openByHandleAt = original_openByHandleAt }()
	openByHandleAt = func(_ int, _ unix.FileHandle, _ int) (fd int, err error) {
		return 255, nil
	}

	// Mock the nameToHandleAt
	original_nameToHandleAt := nameToHandleAt
	defer func() { nameToHandleAt = original_nameToHandleAt }()
	nameToHandleAt = func(_ int, _ string, _ int) (handle unix.FileHandle, mountID int, err error) {
		return newFileHandle(255, []byte("test")), 1, nil
	}

	h, err := NewPIDHandleFromString(os.Getpid(), nameToHandlePrefix+"254 74657374")
	assert.NoError(t, err)
	defer h.Close()

	pidData, err := h.String()
	assert.NoError(t, err)
	assert.Equal(t, nameToHandlePrefix+"254 74657374", pidData)
	assert.Equal(t, h.PID(), os.Getpid())
}

func TestNewPIDHandleFromStringWrongPidData(t *testing.T) {
	// Mock the pidfdOpen
	original := pidfdOpen
	defer func() { pidfdOpen = original }()
	pidfdOpen = func(_ int, _ int) (int, error) {
		return 254, nil
	}

	// Mock the newFileHandle
	original_newFileHandle := newFileHandle
	defer func() { newFileHandle = original_newFileHandle }()
	newFileHandle = func(_ int32, _ []byte) unix.FileHandle {
		return unix.NewFileHandle(254, []byte("test"))
	}

	// Mock the openByHandleAt
	original_openByHandleAt := openByHandleAt
	defer func() { openByHandleAt = original_openByHandleAt }()
	openByHandleAt = func(_ int, _ unix.FileHandle, _ int) (fd int, err error) {
		return 255, nil
	}

	values := []string{
		nameToHandlePrefix + "foo",
		nameToHandlePrefix + "254 foo",
		nameToHandlePrefix + "254 foo bar",
		nameToHandlePrefix + "foo 1245",
	}

	for _, s := range values {
		_, err := NewPIDHandleFromString(os.Getpid(), s)
		assert.Error(t, err)
	}
}

func TestPIDHandlePidfdStartTime(t *testing.T) {
	h, err := NewPIDHandleFromString(os.Getpid(), "start-time:1234567890")
	assert.NoError(t, err)
	defer h.Close()
}

func TestPIDHandleKill(t *testing.T) {
	// Mock the pidfdOpen
	original := pidfdOpen
	defer func() { pidfdOpen = original }()
	pidfdOpen = func(_ int, _ int) (int, error) {
		return 254, nil
	}

	// Mock the pidfdSendSignal
	original_pidfdSendSignal := pidfdSendSignal
	defer func() { pidfdSendSignal = original_pidfdSendSignal }()
	pidfdSendSignal = func(_ int, _ unix.Signal, _ *unix.Siginfo, _ int) (err error) {
		return unix.ESRCH
	}

	// Mock the nameToHandleAt
	original_nameToHandleAt := nameToHandleAt
	defer func() { nameToHandleAt = original_nameToHandleAt }()
	nameToHandleAt = func(_ int, _ string, _ int) (handle unix.FileHandle, mountID int, err error) {
		return newFileHandle(254, []byte("test")), 1, nil
	}

	h, err := NewPIDHandle(os.Getpid())
	assert.NoError(t, err)
	defer h.Close()

	err = h.Kill(0)
	assert.ErrorIs(t, err, unix.ESRCH)
}

func TestPIDHandleKillPidfdNotSupported(t *testing.T) {
	// Mock the pidfdOpen
	original := pidfdOpen
	defer func() { pidfdOpen = original }()
	pidfdOpen = func(_ int, _ int) (int, error) {
		return -1, unix.ENOSYS
	}

	h, err := NewPIDHandle(os.Getpid())
	assert.NoError(t, err)
	defer h.Close()

	pidData, err := h.String()
	assert.NoError(t, err)
	h, err = NewPIDHandleFromString(os.Getpid(), pidData)
	assert.NoError(t, err)

	err = h.Kill(0)
	assert.NoError(t, err)
	isAlive, err := h.IsAlive()
	assert.NoError(t, err)
	assert.True(t, isAlive)
}

func TestPIDHandleKillPidfdNotSupportedStartTimeNotMatch(t *testing.T) {
	// Mock the pidfdOpen
	original := pidfdOpen
	defer func() { pidfdOpen = original }()
	pidfdOpen = func(_ int, _ int) (int, error) {
		return -1, unix.ENOSYS
	}

	h, err := NewPIDHandle(os.Getpid())
	assert.NoError(t, err)
	defer h.Close()

	h, err = NewPIDHandleFromString(os.Getpid(), "start-time:1234567890")
	assert.NoError(t, err)

	err = h.Kill(0)
	assert.ErrorIs(t, err, unix.ESRCH)

	isAlive, err := h.IsAlive()
	assert.NoError(t, err)
	assert.False(t, isAlive)
}
