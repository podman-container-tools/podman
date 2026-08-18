//go:build (linux || freebsd) && !remote

package terminal

import (
	"errors"
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/shutdown"
	"go.podman.io/podman/v6/pkg/signal"
)

// ProxyExecSignals forwards signals Podman receives to an exec session, via
// Container.ExecKill so delivery goes through libpod's own PID bookkeeping.
func ProxyExecSignals(ctr *libpod.Container, sessionID string) {
	// Stop catching the shutdown signals (SIGINT, SIGTERM) - they're going
	// to the exec session now.
	shutdown.Stop() //nolint: errcheck

	sigBuffer := make(chan os.Signal, signal.SignalBufferSize)
	signal.CatchAll(sigBuffer)

	logrus.Debugf("Enabling signal proxying to exec session %s", sessionID)

	go func() {
		for s := range sigBuffer {
			syscallSignal := s.(syscall.Signal)

			if err := ctr.ExecKill(sessionID, uint(syscallSignal)); err != nil {
				if !errors.Is(err, define.ErrExecSessionStateInvalid) && !errors.Is(err, define.ErrNoSuchExecSession) {
					logrus.Errorf("forwarding signal %d to exec session %s: %v", s, sessionID, err)
					continue
				}
				// Session is gone: send this one to ourselves rather than
				// lose it, and let the defaults play out.
				logrus.Infof("Ceasing signal forwarding, exec session %s has stopped", sessionID)
				signal.StopCatch(sigBuffer)
				if err := syscall.Kill(syscall.Getpid(), syscallSignal); err != nil {
					logrus.Errorf("Failed to kill pid %d", syscall.Getpid())
				}
				return
			}
		}
	}()
}

// ProxySignals ...
func ProxySignals(ctr *libpod.Container) {
	// Stop catching the shutdown signals (SIGINT, SIGTERM) - they're going
	// to the container now.
	shutdown.Stop() //nolint: errcheck

	sigBuffer := make(chan os.Signal, signal.SignalBufferSize)
	signal.CatchAll(sigBuffer)

	logrus.Debugf("Enabling signal proxying")

	go func() {
		for s := range sigBuffer {
			syscallSignal := s.(syscall.Signal)

			if err := ctr.Kill(uint(syscallSignal)); err != nil {
				// If the container is no longer running/removed do not log it as error.
				if errors.Is(err, define.ErrCtrStateInvalid) || errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrRemoved) {
					logrus.Infof("Ceasing signal forwarding to container %s as it has stopped", ctr.ID())
				} else {
					logrus.Errorf("forwarding signal %d to container %s: %v", s, ctr.ID(), err)
				}
				// If the container dies, and we find out here,
				// we need to forward that one signal to
				// ourselves so that it is not lost, and then
				// we terminate the proxy and let the defaults
				// play out.
				signal.StopCatch(sigBuffer)
				if err := syscall.Kill(syscall.Getpid(), s.(syscall.Signal)); err != nil {
					logrus.Errorf("Failed to kill pid %d", syscall.Getpid())
				}
				return
			}
		}
	}()
}
