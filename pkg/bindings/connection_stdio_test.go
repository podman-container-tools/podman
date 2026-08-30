package bindings

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestIsFallbackableChannelErr(t *testing.T) {
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
			name: "ConnectionFailed",
			err:  &ssh.OpenChannelError{Reason: ssh.ConnectionFailed, Message: "open failed"},
			want: true,
		},
		{
			name: "wrapped ConnectionFailed",
			err:  fmt.Errorf("dial failed: %w", &ssh.OpenChannelError{Reason: ssh.ConnectionFailed}),
			want: true,
		},
		{
			// An explicit administrative denial, routing around it is not our call.
			name: "prohibited",
			err:  &ssh.OpenChannelError{Reason: ssh.Prohibited, Message: "prohibited"},
			want: false,
		},
		{
			name: "resource shortage",
			err:  &ssh.OpenChannelError{Reason: ssh.ResourceShortage, Message: "resource shortage"},
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
			assert.Equal(t, tt.want, isFallbackableChannelErr(tt.err))
		})
	}
}

const allowStreamlocal = ssh.RejectionReason(0)

type stdioTestServerOptions struct {
	streamlocalReason ssh.RejectionReason
	rejectSessions    bool
	backend           string
}

type stdioTestServer struct {
	opts stdioTestServerOptions
	addr string

	mu                  sync.Mutex
	streamlocalAttempts int
	execCommands        []string
}

func (s *stdioTestServer) StreamlocalAttempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streamlocalAttempts
}

func (s *stdioTestServer) ExecCommands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.execCommands...)
}

func newStdioTestServer(t *testing.T, opts stdioTestServerOptions) (*stdioTestServer, *ssh.Client) {
	t.Helper()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "pong %s", r.URL.Path)
	}))
	t.Cleanup(backend.Close)

	opts.backend = backend.Listener.Addr().String()
	s := &stdioTestServer{opts: opts}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)

	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	s.addr = listener.Addr().String()

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				return
			}
			go s.handleConn(nConn, serverConfig)
		}
	}()

	client, err := ssh.Dial("tcp", s.addr, &ssh.ClientConfig{
		User:            "test",
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
		Timeout:         10 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	return s, client
}

func (s *stdioTestServer) handleConn(nConn net.Conn, config *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		nConn.Close()
		return
	}
	defer conn.Close()

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		switch newChannel.ChannelType() {
		case "direct-streamlocal@openssh.com":
			s.mu.Lock()
			s.streamlocalAttempts++
			s.mu.Unlock()

			if s.opts.streamlocalReason != allowStreamlocal {
				_ = newChannel.Reject(s.opts.streamlocalReason, "streamlocal refused")
				continue
			}

			channel, requests, err := newChannel.Accept()
			if err != nil {
				continue
			}
			go ssh.DiscardRequests(requests)
			go s.bridge(channel, false)
		case "session":
			if s.opts.rejectSessions {
				_ = newChannel.Reject(ssh.Prohibited, "sessions refused")
				continue
			}

			channel, requests, err := newChannel.Accept()
			if err != nil {
				continue
			}
			go s.handleSession(channel, requests)
		default:
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
		}
	}
}

func (s *stdioTestServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	for req := range requests {
		if req.Type != "exec" {
			_ = req.Reply(false, nil)
			continue
		}

		var payload struct{ Command string }
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			_ = req.Reply(false, nil)
			continue
		}

		s.mu.Lock()
		s.execCommands = append(s.execCommands, payload.Command)
		s.mu.Unlock()

		_ = req.Reply(true, nil)
		go s.bridge(channel, true)
	}
}

func (s *stdioTestServer) bridge(channel ssh.Channel, exec bool) {
	defer channel.Close()

	backend, err := net.Dial("tcp", s.opts.backend)
	if err != nil {
		return
	}
	defer backend.Close()

	go func() {
		_, _ = io.Copy(backend, channel)
		backend.Close()
	}()
	_, _ = io.Copy(channel, backend)

	if exec {
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: 0}))
	}
}

func newTestSocketURL(path string) *url.URL {
	return &url.URL{
		Scheme: "ssh",
		User:   url.User("test"),
		Host:   "localhost",
		Path:   path,
	}
}

const testSocketPath = "/run/podman/podman.sock"

func newSSHTestHTTPClient(client *ssh.Client) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext:       newSSHDialContext(client, newTestSocketURL(testSocketPath)),
			DisableKeepAlives: true,
		},
		Timeout: 10 * time.Second,
	}
}

func getPing(client *http.Client) (string, error) {
	resp, err := client.Get("http://d/ping")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func TestSSHDialContextFallsBackOnChannelRejection(t *testing.T) {
	tests := []struct {
		name   string
		reason ssh.RejectionReason
	}{
		{name: "server does not implement streamlocal", reason: ssh.UnknownChannelType},
		{name: "server refuses streamlocal forwarding", reason: ssh.ConnectionFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := newStdioTestServer(t, stdioTestServerOptions{streamlocalReason: tt.reason})
			httpClient := newSSHTestHTTPClient(client)

			body, err := getPing(httpClient)
			require.NoError(t, err)
			assert.Equal(t, "pong /ping", body)

			assert.Equal(t, 1, server.StreamlocalAttempts(), "the direct channel should be tried first")
			assert.Equal(t,
				[]string{"podman --url unix://" + testSocketPath + " system dial-stdio"},
				server.ExecCommands(),
			)
		})
	}
}

func TestSSHDialContextPrefersDirectChannel(t *testing.T) {
	server, client := newStdioTestServer(t, stdioTestServerOptions{streamlocalReason: allowStreamlocal})
	httpClient := newSSHTestHTTPClient(client)

	body, err := getPing(httpClient)
	require.NoError(t, err)
	assert.Equal(t, "pong /ping", body)

	assert.Equal(t, 1, server.StreamlocalAttempts())
	assert.Empty(t, server.ExecCommands(), "the fallback should not run when the direct channel works")
}

func TestSSHDialContextDoesNotFallBackOnProhibited(t *testing.T) {
	server, client := newStdioTestServer(t, stdioTestServerOptions{streamlocalReason: ssh.Prohibited})
	httpClient := newSSHTestHTTPClient(client)

	_, err := getPing(httpClient)
	require.Error(t, err)
	assert.ErrorContains(t, err, "streamlocal refused")
	assert.Empty(t, server.ExecCommands(), "an administrative denial should not be routed around")
}

func TestSSHDialContextLatchesAfterSuccessfulFallback(t *testing.T) {
	server, client := newStdioTestServer(t, stdioTestServerOptions{streamlocalReason: ssh.ConnectionFailed})
	httpClient := newSSHTestHTTPClient(client)

	for range 3 {
		body, err := getPing(httpClient)
		require.NoError(t, err)
		assert.Equal(t, "pong /ping", body)
	}

	assert.Equal(t, 1, server.StreamlocalAttempts(), "the direct channel should only be tried until it is known to fail")
	assert.Len(t, server.ExecCommands(), 3)
}

func TestSSHDialContextDoesNotLatchWhenFallbackFails(t *testing.T) {
	server, client := newStdioTestServer(t, stdioTestServerOptions{
		streamlocalReason: ssh.ConnectionFailed,
		rejectSessions:    true,
	})
	httpClient := newSSHTestHTTPClient(client)

	for range 2 {
		_, err := getPing(httpClient)
		require.Error(t, err)
		assert.ErrorContains(t, err, "streamlocal refused")
		assert.ErrorContains(t, err, "sessions refused")
	}

	assert.Equal(t, 2, server.StreamlocalAttempts(), "a failed fallback should not latch")
}

func TestDialSSHStdioQuotesSocketPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "plain path",
			path: "/run/user/1000/podman/podman.sock",
			want: "podman --url unix:///run/user/1000/podman/podman.sock system dial-stdio",
		},
		{
			name: "command substitution",
			path: "/run/podman/$(touch pwned).sock",
			want: `podman --url 'unix:///run/podman/$(touch pwned).sock' system dial-stdio`,
		},
		{
			name: "embedded single quote",
			path: "/run/podman/p'wn.sock",
			want: `podman --url 'unix:///run/podman/p'\''wn.sock' system dial-stdio`,
		},
		{
			name: "command separator",
			path: "/run/podman/x; touch pwned",
			want: `podman --url 'unix:///run/podman/x; touch pwned' system dial-stdio`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := newStdioTestServer(t, stdioTestServerOptions{streamlocalReason: ssh.ConnectionFailed})

			conn, err := dialSSHStdio(client, tt.path)
			require.NoError(t, err)
			defer conn.Close()

			assert.Eventually(t, func() bool {
				return len(server.ExecCommands()) == 1
			}, 5*time.Second, 10*time.Millisecond)
			assert.Equal(t, []string{tt.want}, server.ExecCommands())
		})
	}
}

type mockSession struct {
	Closed   bool
	CloseErr error
}

func (m *mockSession) Close() error {
	m.Closed = true
	return m.CloseErr
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

func TestSshStdioConnCloseJoinsSessionError(t *testing.T) {
	session := &mockSession{CloseErr: errors.New("session close failed")}
	conn, remote := newTestStdioConn(newTestStdioConnOptions{session: session})
	defer remote.Close()

	err := conn.Close()
	assert.ErrorContains(t, err, "session close failed")
	assert.True(t, session.Closed)
}

func TestSshStdioConnCloseIgnoresEOF(t *testing.T) {
	session := &mockSession{CloseErr: io.EOF}
	conn, remote := newTestStdioConn(newTestStdioConnOptions{session: session})
	defer remote.Close()

	err := conn.Close()
	assert.NoError(t, err)
	assert.True(t, session.Closed)
}

func TestSshStdioConnAddrs(t *testing.T) {
	conn, remote := newTestStdioConn(newTestStdioConnOptions{})
	defer conn.Close()
	defer remote.Close()

	assert.Equal(t, &net.UnixAddr{Name: "@", Net: "unix"}, conn.LocalAddr())
	assert.Equal(t, &net.UnixAddr{Name: "/run/podman/podman.sock", Net: "unix"}, conn.RemoteAddr())
}
