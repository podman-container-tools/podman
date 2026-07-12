package msg

import (
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/sys/unix"

	"github.com/rootless-containers/rootlesskit/v2/pkg/lowlevelmsgutil"
	"github.com/rootless-containers/rootlesskit/v2/pkg/port"
)

const (
	RequestTypeInit    = "init"
	RequestTypeConnect = "connect"
)

// Request and Response are encoded as JSON with uint32le length header.
type Request struct {
	Type          string // "init" or "connect"
	Proto         string // "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"
	IP            string
	Port          int
	ParentIP      string
	HostGatewayIP string
}

// Reply may contain FD as OOB
type Reply struct {
	Error string
}

// Initiate sends "init" request to the child UNIX socket.
func Initiate(c *net.UnixConn) error {
	req := Request{
		Type: RequestTypeInit,
	}
	if _, err := lowlevelmsgutil.MarshalToWriter(c, &req); err != nil {
		return err
	}
	if err := c.CloseWrite(); err != nil {
		return err
	}
	var rep Reply
	if _, err := lowlevelmsgutil.UnmarshalFromReader(c, &rep); err != nil {
		return err
	}
	return c.CloseRead()
}

func hostGatewayIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}

// ConnectToChild connects to the child UNIX socket, and obtains TCP or UDP socket FD
// that corresponds to the port spec.
func ConnectToChild(c *net.UnixConn, spec port.Spec) (int, error) {
	req := Request{
		Type:          RequestTypeConnect,
		Proto:         spec.Proto,
		Port:          spec.ChildPort,
		IP:            spec.ChildIP,
		ParentIP:      spec.ParentIP,
		HostGatewayIP: hostGatewayIP(),
	}
	if _, err := lowlevelmsgutil.MarshalToWriter(c, &req); err != nil {
		return 0, err
	}
	if err := c.CloseWrite(); err != nil {
		return 0, err
	}
	oobSpace := unix.CmsgSpace(4)
	oob := make([]byte, oobSpace)
	var (
		oobN int
		err  error
	)
	for {
		_, oobN, _, _, err = c.ReadMsgUnix(nil, oob)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		return 0, err
	}
	if oobN != oobSpace {
		return 0, fmt.Errorf("expected OOB space %d, got %d", oobSpace, oobN)
	}
	oob = oob[:oobN]
	fd, err := parseFDFromOOB(oob)
	if err != nil {
		return 0, err
	}
	if err := c.CloseRead(); err != nil {
		return 0, err
	}
	return fd, nil
}

// ConnectToChildWithSocketPath wraps ConnectToChild
func ConnectToChildWithSocketPath(socketPath string, spec port.Spec) (int, error) {
	var dialer net.Dialer
	conn, err := dialer.Dial("unix", socketPath)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	c := conn.(*net.UnixConn)
	return ConnectToChild(c, spec)
}

// ConnectToChildWithRetry retries ConnectToChild every (i*5) milliseconds.
func ConnectToChildWithRetry(socketPath string, spec port.Spec, retries int) (int, error) {
	for i := 0; i < retries; i++ {
		fd, err := ConnectToChildWithSocketPath(socketPath, spec)
		if i == retries-1 && err != nil {
			return 0, err
		}
		if err == nil {
			return fd, err
		}
		// TODO: backoff
		time.Sleep(time.Duration(i*5) * time.Millisecond)
	}
	// NOT REACHED
	return 0, errors.New("reached max retry")
}

func parseFDFromOOB(oob []byte) (int, error) {
	scms, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return 0, err
	}
	if len(scms) != 1 {
		return 0, fmt.Errorf("unexpected scms: %v", scms)
	}
	scm := scms[0]
	fds, err := unix.ParseUnixRights(&scm)
	if err != nil {
		return 0, err
	}
	if len(fds) != 1 {
		return 0, fmt.Errorf("unexpected fds: %v", fds)
	}
	return fds[0], nil
}
