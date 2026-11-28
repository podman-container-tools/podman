package file

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test that creating and destroying locks work
func TestCreateAndDeallocate(t *testing.T) {
	d := t.TempDir()

	_, err := OpenFileLock(filepath.Join(d, "locks"))
	assert.Error(t, err)

	l, err := CreateFileLock(filepath.Join(d, "locks"))
	assert.NoError(t, err)

	lock, err := l.AllocateLock()
	assert.NoError(t, err)

	err = l.AllocateGivenLock(lock)
	assert.Error(t, err)

	err = l.DeallocateLock(lock)
	assert.NoError(t, err)

	err = l.AllocateGivenLock(lock)
	assert.NoError(t, err)

	err = l.DeallocateAllLocks()
	assert.NoError(t, err)

	err = l.AllocateGivenLock(lock)
	assert.NoError(t, err)

	err = l.DeallocateAllLocks()
	assert.NoError(t, err)
}

// Test that DeallocateAllLocks reports a lock it could not remove instead of
// returning success.
func TestDeallocateAllLocksError(t *testing.T) {
	d := t.TempDir()

	lockDir := filepath.Join(d, "locks")
	l, err := CreateFileLock(lockDir)
	assert.NoError(t, err)

	lock, err := l.AllocateLock()
	assert.NoError(t, err)

	// Make one entry impossible to remove. os.Remove refuses to remove a
	// non-empty directory, which fails no matter which user the tests run
	// as - a permission-based failure would be skipped by root. The name
	// sorts before the numeric lock files so that DeallocateAllLocks(),
	// which walks the directory in sorted order, hits it first.
	stuck := filepath.Join(lockDir, "!stuck")
	assert.NoError(t, os.Mkdir(stuck, 0o700))
	assert.NoError(t, os.WriteFile(filepath.Join(stuck, "child"), nil, 0o600))

	err = l.DeallocateAllLocks()
	assert.ErrorContains(t, err, stuck)

	// The failure must not stop the remaining locks from being deallocated.
	// AllocateGivenLock() creates the lock file with O_EXCL, so it only
	// succeeds if the lock really was removed.
	assert.NoError(t, l.AllocateGivenLock(lock))
}

// Test that creating and destroying locks work
func TestLockAndUnlock(t *testing.T) {
	d := t.TempDir()

	l, err := CreateFileLock(filepath.Join(d, "locks"))
	assert.NoError(t, err)

	lock, err := l.AllocateLock()
	assert.NoError(t, err)

	err = l.LockFileLock(lock)
	assert.NoError(t, err)

	lslocks, err := exec.LookPath("lslocks")
	if err == nil {
		lockPath := l.getLockPath(lock)
		out, err := exec.Command(lslocks, "--json", "-p", strconv.Itoa(os.Getpid())).CombinedOutput()
		assert.NoError(t, err)

		assert.Contains(t, string(out), lockPath)
	}

	err = l.UnlockFileLock(lock)
	assert.NoError(t, err)
}
