package channel

import (
	"errors"
	"io"
	"sync"
)

// WriteCloser is an io.WriteCloser that proxies Write() calls to a channel
// The []byte buffer of the Write() is queued on the channel as one message.
type WriteCloser interface {
	io.WriteCloser
	Chan() <-chan []byte
}

type writeCloser struct {
	ch  chan []byte
	mux sync.Mutex
}

// NewWriter initializes a new channel writer
func NewWriter(c chan []byte) WriteCloser {
	return &writeCloser{
		ch: c,
	}
}

// Chan returns the R/O channel behind WriteCloser
func (w *writeCloser) Chan() <-chan []byte {
	return w.ch
}

// Write method for WriteCloser
func (w *writeCloser) Write(b []byte) (bLen int, err error) {
	if w == nil {
		return 0, errors.New("use channel.NewWriter() to initialize a WriteCloser")
	}

	w.mux.Lock()
	defer w.mux.Unlock()

	if w.ch == nil {
		return 0, errors.New("the channel is closed for Write")
	}

	buf := make([]byte, len(b))
	copy(buf, b)
	w.ch <- buf

	return len(b), nil
}

// Close method for WriteCloser
func (w *writeCloser) Close() error {
	w.mux.Lock()
	defer w.mux.Unlock()

	close(w.ch)
	w.ch = nil
	return nil
}
