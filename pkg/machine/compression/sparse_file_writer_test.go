package compression

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type memorySparseFile struct {
	buffer bytes.Buffer
	pos    int64
	sparse int64
}

func (m *memorySparseFile) Seek(offset int64, whence int) (int64, error) {
	logrus.Debugf("Seek %d %d", offset, whence)
	var newPos int64
	switch whence {
	case io.SeekStart:
		panic("unexpected")
	case io.SeekCurrent:
		newPos = m.pos + offset
		if offset < -1 {
			panic("unexpected")
		}
		m.sparse += offset
	case io.SeekEnd:
		newPos = int64(m.buffer.Len()) + offset
	default:
		return 0, errors.New("unsupported seek whence")
	}

	if newPos < 0 {
		return 0, errors.New("negative position is not allowed")
	}

	m.pos = newPos
	return newPos, nil
}

func (m *memorySparseFile) Write(b []byte) (n int, err error) {
	logrus.Debugf("Write %d", len(b))
	if int64(m.buffer.Len()) < m.pos {
		padding := make([]byte, m.pos-int64(m.buffer.Len()))
		_, err := m.buffer.Write(padding)
		if err != nil {
			return 0, err
		}
	}

	n, err = m.buffer.Write(b)
	m.pos += int64(n)
	return n, err
}

func testInputWithWriteLen(t *testing.T, input []byte, minSparse int64, chunkSize int) {
	m := &memorySparseFile{}
	sparseWriter := NewSparseWriter(m)

	for i := 0; i < len(input); i += chunkSize {
		end := min(i+chunkSize, len(input))
		_, err := sparseWriter.Write(input[i:end])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	}
	err := sparseWriter.Close()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	assert.Equal(t, string(input), m.buffer.String())
	assert.GreaterOrEqual(t, m.sparse, minSparse)
}

func testInput(t *testing.T, name string, inputBytes []byte, minSparse int64) {
	currentLen := 1
	for {
		t.Run(fmt.Sprintf("%s@%d", name, currentLen), func(t *testing.T) {
			testInputWithWriteLen(t, inputBytes, minSparse, currentLen)
		})
		currentLen <<= 1
		if currentLen > len(inputBytes) {
			break
		}
	}
}

func TestSparseWriter(t *testing.T) {
	testInput(t, "small contents", []byte("hello"), 0)
	testInput(t, "small zeroes", append(make([]byte, 100), []byte("hello")...), 0)
	testInput(t, "empty", []byte(""), 0)
	testInput(t, "small iterated", []byte{'a', 0, 'a', 0, 'a', 0}, 0)
	testInput(t, "small iterated2", []byte{0, 'a', 0, 'a', 0, 'a'}, 0)

	// add "hello" at the beginning
	const largeSize = 1024 * 1024
	largeInput := make([]byte, largeSize)
	copy(largeInput, []byte("hello"))
	testInput(t, "sparse end", largeInput, largeSize-5-1) // -1 for the final byte establishing file size

	// add "hello" at the end
	largeInput = make([]byte, largeSize)
	copy(largeInput[largeSize-5:], []byte("hello"))
	testInput(t, "sparse beginning", largeInput, largeSize-5)

	// add "hello" in the middle
	largeInput = make([]byte, largeSize)
	copy(largeInput[len(largeInput)/2:], []byte("hello"))
	testInput(t, "sparse both ends", largeInput, largeSize-5-1) // -1 for the final byte establishing file size

	largeInput = make([]byte, largeSize)
	copy(largeInput[0:5], []byte("hello"))
	copy(largeInput[largeSize-5:], []byte("HELLO"))
	testInput(t, "sparse middle", largeInput, largeSize-10)
}
