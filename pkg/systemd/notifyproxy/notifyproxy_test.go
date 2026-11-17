//go:build !windows

package notifyproxy

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// Helper function to send the specified message over the socket of the proxy.
func sendMessage(t *testing.T, proxy *NotifyProxy, message string) {
	err := SendMessage(proxy.SocketPath(), message)
	require.NoError(t, err)
}

func TestSendMessageWaitsForBarrier(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "notify.sock")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: socketPath, Net: "unixgram"})
	require.NoError(t, err)
	defer conn.Close()

	sent := make(chan error, 1)
	go func() {
		sent <- SendMessageWithBarrier(socketPath, daemon.SdNotifyReady)
	}()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(_notifyBarrierTimeout)))
	buf := make([]byte, _notifyBufferMax)
	oob := make([]byte, unix.CmsgSpace(4))
	n, err := conn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, daemon.SdNotifyReady, string(buf[:n]))

	n, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
	require.NoError(t, err)
	scms, err := unix.ParseSocketControlMessage(oob[:oobn])
	require.NoError(t, err)
	require.Len(t, scms, 1)
	fds, err := unix.ParseUnixRights(&scms[0])
	require.NoError(t, err)
	barriers := make([]*os.File, len(fds))
	for i, fd := range fds {
		barriers[i] = os.NewFile(uintptr(fd), "barrier")
		defer barriers[i].Close()
	}
	require.Len(t, barriers, 1)
	require.Equal(t, _notifyBarrierMsg, string(buf[:n]))

	select {
	case err := <-sent:
		t.Fatalf("SendMessage returned before the barrier was closed: %v", err)
	default:
	}

	require.NoError(t, barriers[0].Close())
	select {
	case err := <-sent:
		require.NoError(t, err)
	case <-time.After(_notifyBarrierTimeout):
		t.Fatal("SendMessage did not return after the barrier was closed")
	}
}

func TestSendMessageWithoutBarrierSupport(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "notify.sock")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: socketPath, Net: "unixgram"})
	require.NoError(t, err)
	defer conn.Close()

	// Nobody reads the socket, so the barrier fd is never closed.
	start := time.Now()
	require.NoError(t, SendMessageWithBarrier(socketPath, daemon.SdNotifyReady))
	require.GreaterOrEqual(t, time.Since(start), _notifyBarrierTimeout)
}

func TestProxyClosesBarrierFd(t *testing.T) {
	proxy, err := New("")
	require.NoError(t, err)
	defer proxy.Close()

	start := time.Now()
	require.NoError(t, SendMessageWithBarrier(proxy.SocketPath(), "foo"))
	require.Less(t, time.Since(start), _notifyBarrierTimeout)
}

func TestNotifyProxy(t *testing.T) {
	proxy, err := New("")
	require.NoError(t, err)
	require.FileExists(t, proxy.SocketPath())
	require.NoError(t, proxy.Close())
	require.NoFileExists(t, proxy.SocketPath())
}

func TestWaitAndClose(t *testing.T) {
	proxy, err := New("")
	require.NoError(t, err)
	require.FileExists(t, proxy.SocketPath())

	ch := make(chan error)
	defer func() {
		err := proxy.Close()
		require.NoError(t, err, "proxy should close successfully")
	}()
	go func() {
		ch <- proxy.Wait()
	}()

	sendMessage(t, proxy, "foo\n")
	time.Sleep(250 * time.Millisecond)
	select {
	case err := <-ch:
		t.Fatalf("Should still be waiting but received %v", err)
	default:
	}

	sendMessage(t, proxy, daemon.SdNotifyReady+"\nsomething else\n")
	done := func() bool {
		for i := range 10 {
			select {
			case err := <-ch:
				require.NoError(t, err, "Waiting should succeed")
				return true
			default:
				time.Sleep(time.Duration(i*250) * time.Millisecond)
			}
		}
		return false
	}()
	require.True(t, done, "READY MESSAGE SHOULD HAVE ARRIVED")
}
