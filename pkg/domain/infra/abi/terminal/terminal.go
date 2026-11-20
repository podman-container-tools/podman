//go:build !remote && (linux || freebsd)

package terminal

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"slices"

	"github.com/moby/term"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/resize"
	lsignal "go.podman.io/podman/v6/pkg/signal"
)

// RawTtyFormatter ...
type RawTtyFormatter struct{}

// getResize returns a TerminalSize command matching stdin's current
// size on success, and nil on errors.
func getResize() *resize.TerminalSize {
	winsize, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		logrus.Warnf("Could not get terminal size %v", err)
		return nil
	}
	return &resize.TerminalSize{
		Width:  winsize.Width,
		Height: winsize.Height,
	}
}

// Helper for prepareAttach - set up a goroutine to generate terminal resize events
func resizeTty(ctx context.Context, resize chan resize.TerminalSize) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, lsignal.SIGWINCH)
	go func() {
		defer close(resize)
		// Update the terminal size immediately without waiting
		// for a SIGWINCH to get the correct initial size.
		resizeEvent := getResize()
		for {
			if resizeEvent == nil {
				select {
				case <-ctx.Done():
					return
				case <-sigchan:
					resizeEvent = getResize()
				}
			} else {
				select {
				case <-ctx.Done():
					return
				case <-sigchan:
					resizeEvent = getResize()
				case resize <- *resizeEvent:
					resizeEvent = nil
				}
			}
		}
	}()
}

func restoreTerminal(state *term.State, normalLogWriter io.Writer) error {
	log.SetOutput(normalLogWriter)
	return term.RestoreTerminal(os.Stdin.Fd(), state)
}

// Format ...
func (f *RawTtyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	textFormatter := logrus.TextFormatter{}
	bytes, err := textFormatter.Format(entry)

	if err == nil {
		bytes = append(bytes, '\r')
	}

	return bytes, err
}

type rawWriter struct {
	w io.Writer
}

func (w *rawWriter) Write(p []byte) (int, error) {
	buf := slices.Concat(p, []byte("\r"))
	n, err := w.w.Write(buf)
	if n > len(p) {
		n = len(p)
	}
	return n, err
}

func handleTerminalAttach(ctx context.Context, resize chan resize.TerminalSize) (context.CancelFunc, *term.State, io.Writer, error) {
	logrus.Debugf("Handling terminal attach")

	subCtx, cancel := context.WithCancel(ctx)

	resizeTty(subCtx, resize)

	oldTermState, err := term.SaveState(os.Stdin.Fd())
	if err != nil {
		// allow caller to not have to do any cleaning up if we error here
		cancel()
		return nil, nil, nil, fmt.Errorf("unable to save terminal state: %w", err)
	}

	normalLogWriter := log.Writer()
	log.SetOutput(&rawWriter{w: normalLogWriter})
	if _, err := term.SetRawTerminal(os.Stdin.Fd()); err != nil {
		return cancel, nil, nil, err
	}

	return cancel, oldTermState, normalLogWriter, nil
}
