package channel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWriterWriteQueuesMessage(t *testing.T) {
	ch := make(chan []byte, 1)
	w := NewWriter(ch)

	n, err := w.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, []byte("hello"), <-ch)
}

func TestWriterChanReturnsUnderlyingChannel(t *testing.T) {
	ch := make(chan []byte, 1)
	w := NewWriter(ch)

	_, err := w.Write([]byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), <-w.Chan())
}

func TestWriterWriteCopiesInput(t *testing.T) {
	ch := make(chan []byte, 1)
	w := NewWriter(ch)

	input := []byte("abc")
	_, err := w.Write(input)
	require.NoError(t, err)

	// Mutating the caller's buffer after Write must not affect the queued message.
	input[0] = 'x'
	assert.Equal(t, []byte("abc"), <-ch)
}

func TestWriterPreservesMessageOrder(t *testing.T) {
	ch := make(chan []byte, 3)
	w := NewWriter(ch)

	for _, msg := range []string{"one", "two", "three"} {
		_, err := w.Write([]byte(msg))
		require.NoError(t, err)
	}
	assert.Equal(t, []byte("one"), <-ch)
	assert.Equal(t, []byte("two"), <-ch)
	assert.Equal(t, []byte("three"), <-ch)
}

func TestWriterCloseClosesChannel(t *testing.T) {
	ch := make(chan []byte, 1)
	w := NewWriter(ch)

	require.NoError(t, w.Close())

	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after Close")
}

func TestWriterWriteAfterCloseFails(t *testing.T) {
	ch := make(chan []byte, 1)
	w := NewWriter(ch)
	require.NoError(t, w.Close())

	n, err := w.Write([]byte("data"))
	assert.Equal(t, 0, n)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel is closed for Write")
}

func TestWriterWriteNilReceiverFails(t *testing.T) {
	var w *writeCloser
	n, err := w.Write([]byte("data"))
	assert.Equal(t, 0, n)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel.NewWriter()")
}
