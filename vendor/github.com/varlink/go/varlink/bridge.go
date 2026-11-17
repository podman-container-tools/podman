package varlink

import (
	"io"
	"net"
	"os"
	"os/exec"
	"time"
)

var _ net.Conn = &PipeCon{}

type PipeCon struct {
	cmd    *exec.Cmd
	reader io.ReadCloser
	writer io.WriteCloser
}

func (p PipeCon) Read(b []byte) (n int, err error) {
	return p.reader.Read(b)
}

func (p PipeCon) Write(b []byte) (n int, err error) {
	return p.writer.Write(b)
}

func (p PipeCon) LocalAddr() net.Addr {
	panic("implement me")
}

func (p PipeCon) RemoteAddr() net.Addr {
	panic("implement me")
}

func (p PipeCon) SetDeadline(t time.Time) error {
	panic("implement me")
}

func (p PipeCon) SetReadDeadline(t time.Time) error {
	return nil
}

func (p PipeCon) SetWriteDeadline(t time.Time) error {
	return nil
}

func (p PipeCon) Close() error {
	err1 := (p.reader).Close()
	err2 := (p.writer).Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	p.cmd.Wait()

	return nil
}

// NewBridge returns a new connection with the given bridge.
func NewBridge(bridge string) (*Connection, error) {
	return NewBridgeWithStderr(bridge, os.Stderr)
}
