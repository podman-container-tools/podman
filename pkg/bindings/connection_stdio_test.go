package bindings

import (
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

func TestIsUnknownChannelTypeErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "UnknownChannelType",
			err:  &ssh.OpenChannelError{Reason: ssh.UnknownChannelType, Message: "unknown channel type (unsupported channel type)"},
			want: true,
		},
		{
			name: "wrapped UnknownChannelType",
			err:  fmt.Errorf("dial failed: %w", &ssh.OpenChannelError{Reason: ssh.UnknownChannelType}),
			want: true,
		},
		{
			name: "prohibited",
			err:  &ssh.OpenChannelError{Reason: ssh.Prohibited, Message: "prohibited"},
			want: false,
		},
		{
			name: "connection refused",
			err:  errors.New("connection refused"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUnknownChannelTypeErr(tt.err))
		})
	}
}

type mockSession struct {
	Closed bool
}

func (m *mockSession) Close() error {
	m.Closed = true
	return nil
}

type newTestStdioConnOptions struct {
	sessionDone  chan struct{}
	closeTimeout time.Duration
	session      io.Closer
}

func newTestStdioConn(opts newTestStdioConnOptions) (*sshStdioConn, net.Conn) {
	local, remote := net.Pipe()

	if opts.sessionDone == nil {
		opts.sessionDone = make(chan struct{})
		close(opts.sessionDone)
	}

	if opts.closeTimeout == 0 {
		opts.closeTimeout = 5 * time.Second
	}

	if opts.session == nil {
		opts.session = &mockSession{}
	}

	c := &sshStdioConn{
		writer:       local,
		reader:       local,
		path:         "/run/podman/podman.sock",
		sessionDone:  opts.sessionDone,
		session:      opts.session,
		closeTimeout: opts.closeTimeout,
	}
	return c, remote
}

func TestSshStdioReadWrite(t *testing.T) {
	conn, remote := newTestStdioConn(newTestStdioConnOptions{})
	defer conn.Close()
	defer remote.Close()

	go func() {
		buf := make([]byte, 32)
		n, err := remote.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, "request", string(buf[:n]))

		_, err = remote.Write([]byte("response"))
		assert.NoError(t, err)
	}()

	n, err := conn.Write([]byte("request"))
	assert.NoError(t, err)
	assert.Equal(t, 7, n)

	buf := make([]byte, 32)
	n, err = conn.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, "response", string(buf[:n]))
}

func TestSshStdioConnClose(t *testing.T) {
	conn, remote := newTestStdioConn(newTestStdioConnOptions{})
	defer remote.Close()

	err := conn.Close()
	assert.NoError(t, err)
	assert.True(t, conn.session.(*mockSession).Closed, "session should be closed")
}

func TestSshStdioConnCloseTimeout(t *testing.T) {
	mock := &mockSession{}
	conn, remote := newTestStdioConn(newTestStdioConnOptions{
		sessionDone:  make(chan struct{}), // left unclosed to force timeout
		session:      mock,
		closeTimeout: 50 * time.Millisecond,
	})

	defer remote.Close()
	start := time.Now()
	err := conn.Close()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond, "Close should wait for the full timeout")
	assert.Less(t, time.Since(start), time.Second, "Close should not block indefinitely")
	assert.True(t, mock.Closed, "session.Close should be called after timeout")
}

func TestSshStdioConnCloseWaitsForSession(t *testing.T) {
	session := make(chan struct{})
	conn, remote := newTestStdioConn(newTestStdioConnOptions{
		sessionDone:  session,
		session:      &mockSession{},
		closeTimeout: 1 * time.Second,
	})

	defer remote.Close()
	done := make(chan struct{})
	go func() {
		conn.Close()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Close returned before session was signaled")
	case <-time.After(50 * time.Millisecond):
	}

	close(session)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close did not return after session was signaled")
	}

	assert.True(t, conn.session.(*mockSession).Closed, "session should be closed")
}

func TestSshStdioConnAddrs(t *testing.T) {
	conn, remote := newTestStdioConn(newTestStdioConnOptions{})
	defer conn.Close()
	defer remote.Close()

	assert.Equal(t, &net.UnixAddr{Name: "@", Net: "unix"}, conn.LocalAddr())
	assert.Equal(t, &net.UnixAddr{Name: "/run/podman/podman.sock", Net: "unix"}, conn.RemoteAddr())
}
