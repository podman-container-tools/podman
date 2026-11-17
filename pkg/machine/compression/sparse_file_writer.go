package compression

import (
	"errors"
	"fmt"
	"io"
)

const zerosThreshold = 1024

type WriteSeekCloser interface {
	io.Closer
	io.WriteSeeker
}

type sparseWriter struct {
	file io.WriteSeeker
	// Invariant between method calls:
	// The contents of the file match the contents passed to Write, except that pendingZeroes trailing zeroes have not been written.
	// Also, the data that _has_ been written does not end with a zero byte (i.e. pendingZeroes is the largest possible value.
	pendingZeroes int64
}

// NewSparseWriter returns a WriteCloser for underlying file which creates
// holes where appropriate.
// NOTE: The caller must .Close() both the returned sparseWriter AND the underlying file,
// in that order.
func NewSparseWriter(file io.WriteSeeker) *sparseWriter {
	return &sparseWriter{
		file:          file,
		pendingZeroes: 0,
	}
}

func (sw *sparseWriter) createHole(size int64) error {
	_, err := sw.file.Seek(size, io.SeekCurrent)
	return err
}

func zeroSpanEnd(b []byte, i int) int {
	for i < len(b) && b[i] == 0 {
		i++
	}
	return i
}

func nonzeroSpanEnd(b []byte, i int) int {
	for i < len(b) && b[i] != 0 {
		i++
	}
	return i
}

// Write writes data to the file, creating holes for long sequences of zeros.
func (sw *sparseWriter) Write(data []byte) (int, error) {
	initialZeroSpanLength := zeroSpanEnd(data, 0)
	if initialZeroSpanLength == len(data) {
		sw.pendingZeroes += int64(initialZeroSpanLength)
		return initialZeroSpanLength, nil
	}

	// We have _some_ non-zero data to write.
	// Think of the input as an alternating sequence of spans of zeroes / non-zeroes 0a0b…c0,
	// where the starting/ending span of zeroes may be empty.

	pendingWriteOffset := 0
	// The expected condition for creating a hole would be sw.pendingZeroes + initialZeroSpanLength >= zerosThreshold; but
	// if sw.pendingZeroes != 0, we are going to spend a syscall to deal with sw.pendingZeroes either way.
	// We might just as well make it a createHole(), even if the hole size is below zeroThreshold.
	if sw.pendingZeroes != 0 || initialZeroSpanLength >= zerosThreshold {
		if err := sw.createHole(sw.pendingZeroes + int64(initialZeroSpanLength)); err != nil {
			return -1, err
		}
		// We could set sw.pendingZeroes = 0 now; it would always be overwritten on successful return from this function.
		pendingWriteOffset = initialZeroSpanLength
	}

	current := initialZeroSpanLength
	for {
		// Invariant at this point of this loop:
		// - pendingWriteOffset <= current < len(data)
		// - data[current] != 0
		// - data[pendingWriteOffset:current] has not yet been written
		if pendingWriteOffset > current || current >= len(data) {
			return -1, fmt.Errorf("internal error: sparseWriter invariant violation: %d <= %d < %d", pendingWriteOffset, current, len(data))
		}
		if b := data[current]; b == 0 {
			return -1, fmt.Errorf("internal error: sparseWriter invariant violation: %d@%d", b, current)
		}

		nonzeroSpanEnd := nonzeroSpanEnd(data, current)
		if nonzeroSpanEnd == current {
			return -1, fmt.Errorf("internal error: sparseWriter’s nonzeroSpanEnd didn’t advance")
		}
		zeroSpanEnd := zeroSpanEnd(data, nonzeroSpanEnd) // possibly == nonzeroSpanEnd
		zeroSpanLength := zeroSpanEnd - nonzeroSpanEnd
		if zeroSpanEnd < len(data) && zeroSpanLength < zerosThreshold {
			// Too small a hole, keep going
			current = zeroSpanEnd
			continue
		}

		// We have either reached the end, or found an interesting hole. Issue a write.
		if _, err := sw.file.Write(data[pendingWriteOffset:nonzeroSpanEnd]); err != nil {
			return -1, err
		}
		if zeroSpanEnd == len(data) {
			sw.pendingZeroes = int64(zeroSpanLength)
			return zeroSpanEnd, nil
		}

		if err := sw.createHole(int64(zeroSpanLength)); err != nil {
			return -1, err
		}
		pendingWriteOffset = zeroSpanEnd
		current = zeroSpanEnd
	}
}

// Close closes the SparseWriter's underlying file.
func (sw *sparseWriter) Close() error {
	if sw.file == nil {
		return errors.New("file is already closed")
	}
	if sw.pendingZeroes != 0 {
		if holeSize := sw.pendingZeroes - 1; holeSize >= zerosThreshold {
			if err := sw.createHole(holeSize); err != nil {
				return err
			}
			sw.pendingZeroes -= holeSize
		}
		var zeroArray [zerosThreshold]byte
		if _, err := sw.file.Write(zeroArray[:sw.pendingZeroes]); err != nil {
			return err
		}
	}
	sw.file = nil
	return nil
}
