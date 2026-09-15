//go:build amd64 || arm64

package machine

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	gvproxyTypes "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGvProxyNotifier_Ready(t *testing.T) {
	dir := t.TempDir()
	notifier, err := NewGvProxyNotifier(dir)
	require.NoError(t, err)
	defer notifier.Close()

	go notifier.Start(t.Context())

	conn, err := net.Dial("unix", notifier.SocketPath())
	require.NoError(t, err)
	defer conn.Close()

	err = json.NewEncoder(conn).Encode(gvproxyTypes.NotificationMessage{
		NotificationType: gvproxyTypes.Ready,
	})
	require.NoError(t, err)

	waitCtx, waitCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer waitCancel()
	err = notifier.WaitReady(waitCtx)
	assert.NoError(t, err)
}

func TestGvProxyNotifier_HypervisorError(t *testing.T) {
	dir := t.TempDir()
	notifier, err := NewGvProxyNotifier(dir)
	require.NoError(t, err)
	defer notifier.Close()

	go notifier.Start(t.Context())

	conn, err := net.Dial("unix", notifier.SocketPath())
	require.NoError(t, err)
	defer conn.Close()

	err = json.NewEncoder(conn).Encode(gvproxyTypes.NotificationMessage{
		NotificationType: gvproxyTypes.HypervisorError,
	})
	require.NoError(t, err)

	waitCtx, waitCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer waitCancel()
	err = notifier.WaitReady(waitCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hypervisor error")
}

func TestGvProxyNotifier_Timeout(t *testing.T) {
	dir := t.TempDir()
	notifier, err := NewGvProxyNotifier(dir)
	require.NoError(t, err)
	defer notifier.Close()

	go notifier.Start(t.Context())

	waitCtx, waitCancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer waitCancel()
	err = notifier.WaitReady(waitCtx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestCleanupGvProxyNotifySocket(t *testing.T) {
	dir := t.TempDir()

	notifier, err := NewGvProxyNotifier(dir)
	require.NoError(t, err)
	socketPath := notifier.SocketPath()
	notifier.Close()

	_, err = os.Stat(socketPath)
	assert.NoError(t, err, "socket file should still exist after Close()")

	CleanupGvProxyNotifySocket(dir)
	_, err = os.Stat(socketPath)
	assert.True(t, errors.Is(err, os.ErrNotExist), "socket file should be gone after cleanup")
}
