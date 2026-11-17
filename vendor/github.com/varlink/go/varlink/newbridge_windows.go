package varlink

import (
	"github.com/varlink/go/varlink/internal/ctxio"
	"io"
	"os/exec"
)

// NewBridgeWithStderr returns a new connection with the given bridge.
func NewBridgeWithStderr(bridge string, stderr io.Writer) (*Connection, error) {
	c := Connection{}
	cmd := exec.Command("cmd", "/C", bridge)
	cmd.Stderr = stderr
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	c.conn = ctxio.NewConn(PipeCon{cmd, r, w})
	c.address = ""

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	return &c, nil
}
