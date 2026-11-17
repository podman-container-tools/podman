//go:build linux || freebsd

package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"golang.org/x/sys/unix"
)

var (
	sdNotifyListenDescription = `Listen on a systemd notify socket and print every message received on it,
followed by a newline. Barriers (BARRIER=1) are not printed. File descriptors
passed with a message are closed right away, so a sender waiting for a
barrier returns at once.`
	sdNotifyListenCmd = &cobra.Command{
		Use:               "sd-notify-listen SOCKET",
		Args:              cobra.ExactArgs(1),
		Short:             "Listen on a systemd notify socket",
		Long:              sdNotifyListenDescription,
		RunE:              sdNotifyListen,
		ValidArgsFunction: completion.AutocompleteDefault,
		Example:           `podman-testing sd-notify-listen /tmp/notify.sock`,
		// Needs no storage, and must not hold it while the tests run podman.
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
)

func init() {
	mainCmd.AddCommand(sdNotifyListenCmd)
}

func sdNotifyListen(cmd *cobra.Command, args []string) error {
	// Exit cleanly when stopped, so that the parent of a rootless
	// reexec exits cleanly too. This is set up before the socket exists,
	// which is what the tests wait for.
	ctx, stop := signal.NotifyContext(cmd.Context(), unix.SIGTERM, unix.SIGINT)
	defer stop()

	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: args[0], Net: "unixgram"})
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buf := make([]byte, 64*1024)
	oob := make([]byte, unix.CmsgSpace(4*1024))
	for {
		n, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if err := closeFds(oob[:oobn]); err != nil {
			return err
		}
		if msg := string(buf[:n]); msg != "BARRIER=1" {
			if _, err := fmt.Fprintln(os.Stdout, msg); err != nil {
				return err
			}
		}
	}
}

// closeFds closes the file descriptors passed in the control messages.
func closeFds(oob []byte) error {
	scms, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return err
	}
	for i := range scms {
		fds, err := unix.ParseUnixRights(&scms[i])
		if err != nil {
			continue
		}
		for _, fd := range fds {
			if err := unix.Close(fd); err != nil {
				return err
			}
		}
	}
	return nil
}
