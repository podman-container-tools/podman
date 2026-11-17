package machine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	gvProxyTypes "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/sirupsen/logrus"
)

const notifySocketName = "gvproxy-notify.sock"

// GvProxyNotifier listens on a unix socket for JSON notification messages
// from gvproxy.
type GvProxyNotifier struct {
	listener   *net.UnixListener
	socketPath string
	readyCh    chan struct{}
	errorCh    chan error
}

// SocketPath returns the path to the notification unix socket.
func (n *GvProxyNotifier) SocketPath() string {
	return n.socketPath
}

// NewGvProxyNotifier creates a notification listener socket in the given runtime directory.
//
// The socket begins accepting connections at the kernel level immediately upon creation
// (via net.Listen), so gvproxy can be started and connect before Start() is called without
// losing messages. The kernel backlog buffers both the connection and any data sent on it
// until Accept()/Read() consume them.
//
// The notifier is only used during machine start. Close() stops the listener but
// intentionally leaves the socket file on disk because gvproxy remains running after
// start completes and may still dial the socket for later notifications. The socket
// file is removed by CleanupGvProxyNotifySocket, which is called from CleanupGVProxy
// during machine stop after gvproxy has exited. Any stale socket from a previous run
// is removed at the top of this constructor before creating a new listener.
func NewGvProxyNotifier(runtimeDir string) (*GvProxyNotifier, error) {
	socketPath := filepath.Join(runtimeDir, notifySocketName)

	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("removing old notification socket: %w", err)
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("creating notification listener: %w", err)
	}

	// Prevent Close() from removing the socket file. gvproxy may still
	// dial it for notifications after the listener is closed. The file is
	// removed by CleanupGvProxyNotifySocket during machine stop.
	listener.SetUnlinkOnClose(false)

	return &GvProxyNotifier{
		listener:   listener,
		socketPath: socketPath,
		readyCh:    make(chan struct{}, 1),
		errorCh:    make(chan error, 1),
	}, nil
}

// CleanupGvProxyNotifySocket removes the notification socket file from the given runtime directory.
//
// This is called from CleanupGVProxy during machine stop, at which point the notifier
// (which only runs during machine start) has long since closed its listener.
func CleanupGvProxyNotifySocket(runtimeDir string) {
	socketPath := filepath.Join(runtimeDir, notifySocketName)
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logrus.Debugf("failed to remove gvproxy notification socket: %v", err)
	}
}

// Close stops the notifier listener.
//
// The socket file is intentionally left on disk because gvproxy may still dial it for later notifications
// (connection_established, connection_closed).
// The file is cleaned up by CleanupGvProxyNotifySocket after gvproxy exits.
func (n *GvProxyNotifier) Close() {
	n.listener.Close()
}

// Start begins accepting connections and processing notifications.
//
// It blocks until the context is canceled or the listener is closed,
// intended to be run as a goroutine.
func (n *GvProxyNotifier) Start(ctx context.Context) {
	for {
		conn, err := n.listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			logrus.Errorf("notification listener accept error: %v", err)
			return
		}
		n.handleConnection(ctx, conn)
	}
}

func (n *GvProxyNotifier) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)

	for {
		if ctx.Err() != nil {
			return
		}

		var msg gvProxyTypes.NotificationMessage
		if err := decoder.Decode(&msg); err != nil {
			if ctx.Err() != nil {
				return
			}
			logrus.Debugf("notification decode error: %v", err)
			return
		}

		logrus.Debugf("gvproxy notification received: type=%s mac=%s", msg.NotificationType, msg.MacAddress)

		switch msg.NotificationType {
		case gvProxyTypes.Ready:
			select {
			case n.readyCh <- struct{}{}:
			default:
			}
		case gvProxyTypes.ConnectionEstablished:
			logrus.Debugf("gvproxy: VM connected (mac=%s)", msg.MacAddress)
		case gvProxyTypes.HypervisorError:
			select {
			case n.errorCh <- fmt.Errorf("gvproxy reported hypervisor error"):
			default:
			}
		case gvProxyTypes.ConnectionClosed:
			select {
			case n.errorCh <- fmt.Errorf("gvproxy: VM disconnected unexpectedly (mac=%s)", msg.MacAddress):
			default:
			}
		}
	}
}

// WaitReady blocks until the "ready" notification is received or the context expires.
func (n *GvProxyNotifier) WaitReady(ctx context.Context) error {
	select {
	case <-n.readyCh:
		return nil
	case err := <-n.errorCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for gvproxy ready notification: %w", ctx.Err())
	}
}
