//go:build linux

package splitfdstreamserver

import (
	"errors"
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// lease keeps a layer open for as long as the consumer is using it.
//
// The stream refers to the files of the layer, which may only be accessed
// with the layer store locked, and the consumer opens them after the reply,
// so the lock has to be held until the consumer says it is done.  It does
// that by closing the write end of a pipe it is handed, and we see the end
// of file on the read end.
type lease struct {
	layer Layer
	keepR *os.File
}

// newLease returns a lease for layer, and the lifetime descriptor to hand
// to the consumer alongside it.  Once run, the lease owns layer.
func newLease(layer Layer) (*lease, *os.File, error) {
	keepR, keepW, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	return &lease{layer: layer, keepR: keepR}, keepW, nil
}

// cancel stops the lease waiting for the consumer, so that the store can
// shut down without waiting for it too.
func (l *lease) cancel() {
	// run may have finished in the meantime and closed the pipe.
	if err := l.keepR.SetReadDeadline(time.Now()); err != nil && !errors.Is(err, os.ErrClosed) {
		logrus.Debugf("splitfdstream: cancelling a lease: %v", err)
	}
}

// run waits until every copy of the lifetime descriptor is closed, or the
// lease is cancelled, and then releases the layer.
func (l *lease) run() {
	if _, err := io.Copy(io.Discard, l.keepR); err != nil {
		logrus.Debugf("splitfdstream: waiting for the consumer: %v", err)
	}
	if err := l.keepR.Close(); err != nil {
		logrus.Debugf("splitfdstream: closing the lifetime pipe: %v", err)
	}
	// Closing the layer is what releases the layer store lock.
	if err := l.layer.Close(); err != nil {
		logrus.Debugf("splitfdstream: closing layer: %v", err)
	}
}
