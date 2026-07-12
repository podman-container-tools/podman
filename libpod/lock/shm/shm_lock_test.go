//go:build (linux || freebsd) && cgo

package shm

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All tests here are in the same process, which somewhat limits their utility
// The big intent of this package it multiprocess locking, which is really hard
// to test without actually having multiple processes...
// We can at least verify that the locks work within the local process.

// 4 * BITMAP_SIZE to ensure we have to traverse bitmaps
var numLocks = 4 * BitmapSize

const lockPath = "/libpod_test"

// We need a test main to ensure that the SHM is created before the tests run
func TestMain(m *testing.M) {
	// Remove prior /libpod_test
	if err := unlinkSHMLock(lockPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "Error cleaning SHM for tests: %v\n", err)
		os.Exit(-1)
	}
	shmLock, err := CreateSHMLock(lockPath, numLocks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating SHM for tests: %v\n", err)
		os.Exit(-1)
	}

	// Close the SHM - every subsequent test will reopen
	if err := shmLock.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Error closing SHM locks: %v\n", err)
		os.Exit(-1)
	}

	exitCode := m.Run()

	// We need to remove the SHM segment to clean up after ourselves
	os.RemoveAll("/dev/shm/libpod_lock")

	os.Exit(exitCode)
}

func runLockTest(t *testing.T, testFunc func(*testing.T, *SHMLocks)) {
	locks, err := OpenSHMLock(lockPath, numLocks)
	if err != nil {
		t.Fatalf("Error opening locks: %v", err)
	}
	defer func() {
		// Deallocate all locks
		if err := locks.DeallocateAllSemaphores(); err != nil {
			t.Fatalf("Error deallocating semaphores: %v", err)
		}

		if err := locks.Close(); err != nil {
			t.Fatalf("Error closing locks: %v", err)
		}
	}()

	success := t.Run("locks", func(t *testing.T) {
		testFunc(t, locks)
	})
	if !success {
		t.Fail()
	}
}

// Test that creating an SHM with a bad size rounds up to a good size
func TestCreateNewSHMBadSizeRoundsUp(t *testing.T) {
	// Remove prior /test1
	if err := unlinkSHMLock("/test1"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Error cleaning SHM for tests: %v\n", err)
	}
	// Odd number, not a power of 2, should never be a word size on a system
	lock, err := CreateSHMLock("/test1", 7)
	assert.NoError(t, err)
	assert.NotNil(t, lock)

	assert.Equal(t, lock.GetMaxLocks(), BitmapSize)

	if err := lock.Close(); err != nil {
		t.Fatalf("Error closing locks: %v", err)
	}
}

// Test that creating an SHM with 0 size fails
func TestCreateNewSHMZeroSize(t *testing.T) {
	_, err := CreateSHMLock("/test2", 0)
	assert.Error(t, err)
}

// Test that deallocating an unallocated lock errors
func TestDeallocateUnallocatedLockErrors(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		err := locks.DeallocateSemaphore(0)
		assert.Error(t, err)
	})
}

// Test that unlocking an unlocked lock fails
func TestUnlockingUnlockedLockFails(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		err := locks.UnlockSemaphore(0)
		assert.Error(t, err)
	})
}

// Test that locking and double-unlocking fails
func TestDoubleUnlockFails(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		err := locks.LockSemaphore(0)
		assert.NoError(t, err)

		err = locks.UnlockSemaphore(0)
		assert.NoError(t, err)

		err = locks.UnlockSemaphore(0)
		assert.Error(t, err)
	})
}

// Test allocating - lock - unlock - deallocate cycle, single lock
func TestLockLifecycleSingleLock(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		sem, err := locks.AllocateSemaphore()
		require.NoError(t, err)

		err = locks.LockSemaphore(sem)
		assert.NoError(t, err)

		err = locks.UnlockSemaphore(sem)
		assert.NoError(t, err)

		err = locks.DeallocateSemaphore(sem)
		assert.NoError(t, err)
	})
}

// Test allocate two locks returns different locks
func TestAllocateTwoLocksGetsDifferentLocks(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		sem1, err := locks.AllocateSemaphore()
		assert.NoError(t, err)

		sem2, err := locks.AllocateSemaphore()
		assert.NoError(t, err)

		assert.NotEqual(t, sem1, sem2)
	})
}

// Test allocate all locks successful and all are unique
func TestAllocateAllLocksSucceeds(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		sems := make(map[uint32]bool)
		for range numLocks {
			sem, err := locks.AllocateSemaphore()
			assert.NoError(t, err)

			// Ensure the allocate semaphore is unique
			_, ok := sems[sem]
			assert.False(t, ok)

			sems[sem] = true
		}
	})
}

// Test allocating more than the given max fails
func TestAllocateTooManyLocksFails(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		// Allocate all locks
		for range numLocks {
			_, err := locks.AllocateSemaphore()
			assert.NoError(t, err)
		}

		// Try and allocate one more
		_, err := locks.AllocateSemaphore()
		assert.Error(t, err)
	})
}

// Test allocating max locks, deallocating one, and then allocating again succeeds
func TestAllocateDeallocateCycle(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		// Allocate all locks
		for range numLocks {
			_, err := locks.AllocateSemaphore()
			assert.NoError(t, err)
		}

		// Now loop through again, deallocating and reallocating.
		// Each time we free 1 semaphore, allocate again, and make sure
		// we get the same semaphore back.
		for j := range numLocks {
			err := locks.DeallocateSemaphore(j)
			assert.NoError(t, err)

			newSem, err := locks.AllocateSemaphore()
			assert.NoError(t, err)
			assert.Equal(t, j, newSem)
		}
	})
}

// Test that DeallocateAllSemaphores deallocates all semaphores
func TestDeallocateAllSemaphoresDeallocatesAll(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		// Allocate a lock
		locks1, err := locks.AllocateSemaphore()
		assert.NoError(t, err)

		// Free all locks
		err = locks.DeallocateAllSemaphores()
		assert.NoError(t, err)

		// Allocate another lock
		locks2, err := locks.AllocateSemaphore()
		assert.NoError(t, err)

		assert.Equal(t, locks1, locks2)
	})
}

// Test that locks actually lock
func TestLockSemaphoreActuallyLocks(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		// This entire test is very ugly - lots of sleeps to try and get
		// things to occur in the right order.
		// It also doesn't even exercise the multiprocess nature of the
		// locks.

		// Get the current time
		startTime := time.Now()

		// Start a goroutine to take the lock and then release it after
		// a second.
		go func() {
			err := locks.LockSemaphore(0)
			assert.NoError(t, err)

			time.Sleep(1 * time.Second)

			err = locks.UnlockSemaphore(0)
			assert.NoError(t, err)
		}()

		// Sleep for a quarter of a second to give the goroutine time
		// to kick off and grab the lock
		time.Sleep(250 * time.Millisecond)

		// Take the lock
		err := locks.LockSemaphore(0)
		assert.NoError(t, err)

		// Get the current time
		endTime := time.Now()

		// Verify that at least 1 second has passed since start
		duration := endTime.Sub(startTime)
		assert.True(t, duration.Seconds() > 1.0)
	})
}

// Test that locking and unlocking two semaphores succeeds
// Ensures that runtime.LockOSThread() is doing its job
func TestLockAndUnlockTwoSemaphore(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		err := locks.LockSemaphore(5)
		assert.NoError(t, err)

		err = locks.LockSemaphore(6)
		assert.NoError(t, err)

		err = locks.UnlockSemaphore(6)
		assert.NoError(t, err)

		// Now yield scheduling
		// To try and get us on another OS thread
		runtime.Gosched()

		// And unlock the last semaphore
		// If we are in a different OS thread, this should fail.
		// However, runtime.UnlockOSThread() should guarantee we are not
		err = locks.UnlockSemaphore(5)
		assert.NoError(t, err)
	})
}
