//go:build !remote && (linux || freebsd)

package server

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/api/server/idle"
)

// serveTest starts the API http.Server on a loopback listener and returns its
// address.
func serveTest(t *testing.T, handler http.Handler, readHeaderTimeout time.Duration) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := newHTTPServer(handler, idle.NewTracker(time.Minute), readHeaderTimeout)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
	})

	return listener.Addr().String()
}

// stallMidHeader sends a request header block that is never terminated and
// reports whether the service closed the connection within timeout.
func stallMidHeader(t *testing.T, addr string, timeout time.Duration) bool {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	// Note the missing final CRLF, the client never finishes the headers.
	_, err = io.WriteString(conn, "GET /v5.0.0/libpod/_ping HTTP/1.1\r\nHost: podman\r\n")
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(timeout)))
	_, err = conn.Read(make([]byte, 1))

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		// Our own deadline expired, the service is still holding the connection.
		return false
	}
	return err != nil
}

func TestNewHTTPServerReadHeaderTimeout(t *testing.T) {
	assert.NotZero(t, DefaultReadHeaderTimeout,
		"the service must bound how long a client may take to send its headers")

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("closes a connection stalled mid-header", func(t *testing.T) {
		addr := serveTest(t, handler, 250*time.Millisecond)
		assert.True(t, stallMidHeader(t, addr, 30*time.Second),
			"connection stalled mid-header was not closed")
	})

	t.Run("without a timeout the connection is held forever", func(t *testing.T) {
		// Guards the regression: with ReadHeaderTimeout unset a client can pin a
		// file descriptor and a goroutine per connection for the life of the
		// service.
		addr := serveTest(t, handler, 0)
		assert.False(t, stallMidHeader(t, addr, time.Second),
			"connection was closed although no ReadHeaderTimeout was configured")
	})
}

// TestNewHTTPServerStreamsAfterHeaderTimeout covers the endpoints that hijack
// the connection, such as attach and exec: once the headers have been read the
// deadline no longer applies and the connection may stream indefinitely.
func TestNewHTTPServerStreamsAfterHeaderTimeout(t *testing.T) {
	const readHeaderTimeout = 250 * time.Millisecond
	const payload = "streamed after the header timeout\n"

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := w.(http.Hijacker).Hijack()
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close()

		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\n\r\n")
		assert.NoError(t, buf.Flush())

		// Sit idle well past the header timeout before streaming, the way an
		// attached container that produces no output does.
		time.Sleep(4 * readHeaderTimeout)

		_, _ = buf.WriteString(payload)
		assert.NoError(t, buf.Flush())
	})

	addr := serveTest(t, handler, readHeaderTimeout)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()

	_, err = io.WriteString(conn, "GET /v5.0.0/libpod/containers/ctr/attach HTTP/1.1\r\nHost: podman\r\n\r\n")
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(30*time.Second)))

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		require.NoError(t, err, "hijacked connection was closed while streaming")
		if line == payload {
			return
		}
	}
}
