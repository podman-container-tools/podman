package sockets

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/storage/pkg/fileutils"
)

// ListenAndWaitOnSocket waits for a new connection to the listener and sends
// any error back through the channel. ListenAndWaitOnSocket is intended to be
// used as a goroutine
func ListenAndWaitOnSocket(errChan chan<- error, listener net.Listener) {
	conn, err := listener.Accept()
	if err != nil {
		logrus.Debug("failed to connect to ready socket")
		errChan <- err
		return
	}
	_, err = bufio.NewReader(conn).ReadString('\n')
	logrus.Debug("ready ack received")

	if closeErr := conn.Close(); closeErr != nil {
		errChan <- closeErr
		return
	}

	errChan <- err
}

// DialSocketWithBackoffs attempts to connect to the socket in maxBackoffs attempts
func DialSocketWithBackoffs(maxBackoffs int, backoff time.Duration, socketPath string) (conn net.Conn, err error) {
	for i := range maxBackoffs {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			return conn, nil
		}
	}
	return nil, err
}

// DialSocketWithBackoffsAndProcCheck attempts to connect to the socket in
// maxBackoffs attempts. After every failure to connect, it makes sure the
// specified process is alive
func DialSocketWithBackoffsAndProcCheck(
	maxBackoffs int,
	backoff time.Duration,
	socketPath string,
	checkProccessStatus func(string, int, *bytes.Buffer) error,
	procHint string,
	procPid int,
	errBuf *bytes.Buffer,
) (conn net.Conn, err error) {
	for i := range maxBackoffs {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			return conn, nil
		}

		// check to make sure process denoted by procHint is alive
		err = checkProccessStatus(procHint, procPid, errBuf)
		if err != nil {
			return nil, err
		}
	}
	return nil, err
}

// WaitForSocketWithBackoffs attempts to discover listening socket in maxBackoffs attempts
func WaitForSocketWithBackoffs(maxBackoffs int, backoff time.Duration, socketPath string, name string) error {
	backoffWait := backoff
	logrus.Debugf("checking that %q socket is ready", name)
	for range maxBackoffs {
		err := fileutils.Exists(socketPath)
		if err == nil {
			return nil
		}
		time.Sleep(backoffWait)
		backoffWait *= 2
	}
	return fmt.Errorf("unable to connect to %q socket at %q", name, socketPath)
}

// ToUnixURL converts `socketLoc` into URL representation
func ToUnixURL(socketLoc *define.VMFile) (*url.URL, error) {
	p := socketLoc.GetPath()
	if !filepath.IsAbs(p) {
		return nil, fmt.Errorf("socket path must be absolute %q", p)
	}
	s, err := url.Parse("unix:///")
	if err != nil {
		return nil, err
	}
	s = s.JoinPath(filepath.ToSlash(p))
	return s, nil
}
