package bindings

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func isUnknownChannelTypeErr(err error) bool {
	var openChannelErr *ssh.OpenChannelError
	if errors.As(err, &openChannelErr) && openChannelErr.Reason == ssh.UnknownChannelType {
		return true
	}
	return false
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

	cmd := fmt.Sprintf("podman --url unix://%s system dial-stdio", path)
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
		if err := session.Wait(); err != nil {
			logrus.Errorf("ssh session error: %v", err)
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

// The deadline methods are implemented as no-ops as this connection is returned and wrapped by http.Transport,
// which implements its own timeouts and does not rely on the deadline functionality of the underlying connection.

func (c *sshStdioConn) SetDeadline(_ time.Time) error {
	return nil
}

func (c *sshStdioConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (c *sshStdioConn) SetWriteDeadline(_ time.Time) error {
	return nil
}
