package bindings

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	commonssh "go.podman.io/common/pkg/ssh"
	"go.podman.io/storage/pkg/stringutils"
	"golang.org/x/crypto/ssh"
)

func isFallbackableChannelErr(err error) bool {
	var openChannelErr *ssh.OpenChannelError
	if !errors.As(err, &openChannelErr) {
		return false
	}

	return openChannelErr.Reason == ssh.UnknownChannelType || openChannelErr.Reason == ssh.ConnectionFailed
}

func newSSHDialContext(client *ssh.Client, _url *url.URL) func(context.Context, string, string) (net.Conn, error) {
	var useFallback atomic.Bool
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		if useFallback.Load() {
			return dialSSHStdio(client, _url.Path)
		}

		conn, err := commonssh.DialNetContext(ctx, client, "unix", _url)
		if err == nil || !isFallbackableChannelErr(err) {
			return conn, err
		}

		logrus.Debugf("direct-streamlocal channel refused (%v), trying dial-stdio fallback", err)
		fallbackConn, fallbackErr := dialSSHStdio(client, _url.Path)
		if fallbackErr != nil {
			return nil, errors.Join(err, fallbackErr)
		}

		useFallback.Store(true)
		return fallbackConn, nil
	}
}

func dialSSHStdio(client *ssh.Client, path string) (net.Conn, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, err
	}

	var stderr bytes.Buffer
	session.Stderr = &stderr

	cmd := stringutils.ShellQuoteArguments([]string{"podman", "--url", "unix://" + path, "system", "dial-stdio"})
	if err := session.Start(cmd); err != nil {
		session.Close()
		return nil, err
	}

	conn := &sshStdioConn{
		path:         path,
		session:      session,
		writer:       stdin,
		reader:       stdout,
		sessionDone:  make(chan struct{}),
		closeTimeout: 5 * time.Second,
	}

	go func() {
		defer close(conn.sessionDone)
		waitErr := session.Wait()
		stderrOut := strings.TrimRight(stderr.String(), "\n")
		switch {
		case waitErr != nil && stderrOut != "":
			logrus.Errorf("ssh session error: %v: %s", waitErr, stderrOut)
		case waitErr != nil:
			logrus.Errorf("ssh session error: %v", waitErr)
		case stderrOut != "":
			logrus.Debugf("dial-stdio stderr: %s", stderrOut)
		}
	}()

	return conn, nil
}

type sshStdioConn struct {
	path         string
	session      io.Closer
	writer       io.WriteCloser
	reader       io.Reader
	sessionDone  chan struct{}
	closeTimeout time.Duration
}

func (c *sshStdioConn) Close() error {
	err := c.writer.Close()

	select {
	case <-c.sessionDone:
	case <-time.After(c.closeTimeout):
		logrus.Debugf("timed out waiting for dial-stdio session to exit")
	}

	if sessionErr := c.session.Close(); sessionErr != nil && !errors.Is(sessionErr, io.EOF) {
		err = errors.Join(err, sessionErr)
	}

	return err
}

func (c *sshStdioConn) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "@", Net: "unix"}
}

func (c *sshStdioConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: c.path, Net: "unix"}
}

func (c *sshStdioConn) Read(b []byte) (n int, err error) {
	return c.reader.Read(b)
}

func (c *sshStdioConn) Write(b []byte) (n int, err error) {
	return c.writer.Write(b)
}

// http.Transport wraps this connection and manages its own timeouts, so the deadline methods are no-ops.

func (c *sshStdioConn) SetDeadline(_ time.Time) error {
	logrus.Debugf("SetDeadline not implemented for sshStdioConn")
	return nil
}

func (c *sshStdioConn) SetReadDeadline(_ time.Time) error {
	logrus.Debugf("SetReadDeadline not implemented for sshStdioConn")
	return nil
}

func (c *sshStdioConn) SetWriteDeadline(_ time.Time) error {
	logrus.Debugf("SetWriteDeadline not implemented for sshStdioConn")
	return nil
}
