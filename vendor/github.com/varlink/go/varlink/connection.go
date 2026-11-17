package varlink

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/varlink/go/varlink/internal/ctxio"
)

// ErrFDsUnsupported is returned when file-descriptor passing is attempted
// over a transport that does not support SCM_RIGHTS (i.e. non-unix sockets).
var ErrFDsUnsupported = ctxio.ErrFDsUnsupported

// FDReadWriter extends ReadWriterContext with fd-passing capabilities.
// Conn implements this interface on unix transports.
type FDReadWriter interface {
	WriteWithFDs(ctx context.Context, buf []byte, fds []*os.File) (int, error)
	ReadBytesWithFDs(ctx context.Context, delim byte) ([]byte, []*os.File, error)
}

// Message flags for Send(). More indicates that the client accepts more than one method
// reply to this call. Oneway requests, that the service must not send a method reply to
// this call. Continues indicates that the service will send more than one reply.
const (
	More      = 1 << iota
	Oneway    = 1 << iota
	Continues = 1 << iota
	Upgrade   = 1 << iota
)

// Error is a varlink error returned from a method call.
type Error struct {
	Name       string
	Parameters interface{}
}

func (e *Error) DispatchError() error {
	errorRawParameters := e.Parameters.(*json.RawMessage)

	switch e.Name {
	case "org.varlink.service.InterfaceNotFound":
		var param InterfaceNotFound
		if errorRawParameters != nil {
			err := json.Unmarshal(*errorRawParameters, &param)
			if err != nil {
				return e
			}
		}
		return &param
	case "org.varlink.service.MethodNotFound":
		var param MethodNotFound
		if errorRawParameters != nil {
			err := json.Unmarshal(*errorRawParameters, &param)
			if err != nil {
				return e
			}
		}
		return &param
	case "org.varlink.service.MethodNotImplemented":
		var param MethodNotImplemented
		if errorRawParameters != nil {
			err := json.Unmarshal(*errorRawParameters, &param)
			if err != nil {
				return e
			}
		}
		return &param
	case "org.varlink.service.InvalidParameter":
		var param InvalidParameter
		if errorRawParameters != nil {
			err := json.Unmarshal(*errorRawParameters, &param)
			if err != nil {
				return e
			}
		}
		return &param
	}
	return e
}

// Error returns the fully-qualified varlink error name.
func (e *Error) Error() string {
	return e.Name
}

// ReadWriterContext describes the capabilities of the
// underlying varlink connection.
type ReadWriterContext interface {
	Write(context.Context, []byte) (int, error)
	Read(context.Context, []byte) (int, error)
	ReadBytes(ctx context.Context, delim byte) ([]byte, error)
}

// GetNetConn allows access to the underlying net.Conn (where one exists)
// You shouldn't use this for I/O - but might use it to do things like access
// peer credentials on a unix socket
type GetNetConn interface {
	NetConn() net.Conn
}

// ContextDialer is an interface for network dialers that support context-aware dialing.
// The standard *net.Dialer implements this interface. Custom implementations can be used
// to dial through proxies, SSH tunnels, or with custom network configurations.
type ContextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Connection is a connection from a client to a service.
type Connection struct {
	io.Closer
	address string
	conn    *ctxio.Conn
}

// Send sends a method call. It returns a receive() function which is called to retrieve the method reply.
// If Send() is called with the `More` flag and the receive() function carries the `Continues` flag, receive()
// can be called multiple times to retrieve multiple replies.
func (c *Connection) Send(ctx context.Context, method string, parameters interface{}, flags uint64) (func(context.Context, interface{}) (uint64, error), error) {
	receive, err := c.SendWithFDs(ctx, method, parameters, flags, nil)
	if err != nil {
		return nil, err
	}
	// Wrap the fd-aware receive closure into the simpler (uint64, error) signature.
	// Close any reply fds: Send callers don't expect or own them, so leaking them
	// would be silent. A well-behaved service never sends fds to a plain Send caller,
	// but we defend here regardless.
	return func(ctx context.Context, out interface{}) (uint64, error) {
		flags, rxFDs, err := receive(ctx, out)
		for _, f := range rxFDs {
			if f != nil {
				f.Close()
			}
		}
		return flags, err
	}, nil
}

// SendWithFDs sends a method call with optional file descriptors attached as SCM_RIGHTS
// ancillary data. It returns a receive function that yields the reply, any reply fds,
// and the flags.
//
// On non-unix transports, passing len(fds) > 0 returns ErrFDsUnsupported. With zero fds
// the behaviour is identical to Send.
//
// Ownership: the caller retains ownership of fds; the library never closes them.
// Returned *os.File values from the receive closure are owned by the caller.
func (c *Connection) SendWithFDs(ctx context.Context, method string, parameters interface{}, flags uint64, fds []*os.File) (func(context.Context, interface{}) (uint64, []*os.File, error), error) {
	type call struct {
		Method     string      `json:"method"`
		Parameters interface{} `json:"parameters,omitempty"`
		More       bool        `json:"more,omitempty"`
		Oneway     bool        `json:"oneway,omitempty"`
		Upgrade    bool        `json:"upgrade,omitempty"`
	}

	if (flags&More != 0) && (flags&Oneway != 0) {
		return nil, &Error{
			Name:       "org.varlink.InvalidParameter",
			Parameters: "oneway",
		}
	}

	if (flags&More != 0) && (flags&Upgrade != 0) {
		return nil, &Error{
			Name:       "org.varlink.InvalidParameter",
			Parameters: "more",
		}
	}

	m := call{
		Method:     method,
		Parameters: parameters,
		More:       flags&More != 0,
		Oneway:     flags&Oneway != 0,
		Upgrade:    flags&Upgrade != 0,
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	b = append(b, 0)

	if c.conn.CanPassFDs() {
		_, err = c.conn.WriteWithFDs(ctx, b, fds)
	} else {
		if len(fds) > 0 {
			return nil, ErrFDsUnsupported
		}
		_, err = c.conn.Write(ctx, b)
	}
	if err != nil {
		if err == io.EOF {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}

	receive := func(ctx context.Context, outParameters interface{}) (uint64, []*os.File, error) {
		type reply struct {
			Parameters *json.RawMessage `json:"parameters"`
			Continues  bool             `json:"continues"`
			Error      string           `json:"error"`
		}

		var (
			out     []byte
			rxFDs   []*os.File
			readErr error
		)

		if c.conn.CanPassFDs() {
			out, rxFDs, readErr = c.conn.ReadBytesWithFDs(ctx, '\x00')
		} else {
			out, readErr = c.conn.ReadBytes(ctx, '\x00')
		}
		if readErr != nil {
			if readErr == io.EOF {
				return 0, rxFDs, io.ErrUnexpectedEOF
			}
			return 0, rxFDs, readErr
		}

		var m reply
		err = json.Unmarshal(out[:len(out)-1], &m)
		if err != nil {
			// Return rxFDs so the caller can close them; do not leak here.
			return 0, rxFDs, err
		}

		if m.Error != "" {
			e := &Error{
				Name:       m.Error,
				Parameters: m.Parameters,
			}
			// Return rxFDs so the caller can close them; do not leak here.
			return 0, rxFDs, e.DispatchError()
		}

		if m.Parameters != nil {
			if err := json.Unmarshal(*m.Parameters, outParameters); err != nil {
				// Return rxFDs so the caller can close them; do not leak here.
				return 0, rxFDs, err
			}
		}

		if m.Continues {
			return Continues, rxFDs, nil
		}

		return 0, rxFDs, nil
	}

	return receive, nil
}

// Call sends a method call and returns the method reply.
func (c *Connection) Call(ctx context.Context, method string, parameters interface{}, outParameters interface{}) error {
	receive, err := c.Send(ctx, method, &parameters, 0)
	if err != nil {
		return err
	}

	_, err = receive(ctx, outParameters)
	return err
}

// CallWithFDs sends a method call with optional file descriptors and returns the method reply
// along with any file descriptors returned by the service.
//
// On non-unix transports, passing len(fds) > 0 returns ErrFDsUnsupported.
//
// Ownership: the caller retains ownership of the sent fds. Returned *os.File values are
// owned by the caller, who must Close them — even when err is non-nil (partial fd
// delivery on error paths must still be closed by the caller).
func (c *Connection) CallWithFDs(ctx context.Context, method string, parameters interface{}, outParameters interface{}, fds []*os.File) ([]*os.File, error) {
	receive, err := c.SendWithFDs(ctx, method, &parameters, 0, fds)
	if err != nil {
		return nil, err
	}
	_, rxFDs, err := receive(ctx, outParameters)
	return rxFDs, err
}

// GetInterfaceDescription requests the interface description string from the service.
func (c *Connection) GetInterfaceDescription(ctx context.Context, name string) (string, error) {
	type request struct {
		Interface string `json:"interface"`
	}
	type reply struct {
		Description string `json:"description"`
	}

	var r reply
	err := c.Call(ctx, "org.varlink.service.GetInterfaceDescription", request{Interface: name}, &r)
	if err != nil {
		return "", err
	}

	return r.Description, nil
}

// GetInfo requests information about the service.
func (c *Connection) GetInfo(ctx context.Context, vendor *string, product *string, version *string, url *string, interfaces *[]string) error {
	type reply struct {
		Vendor     string   `json:"vendor"`
		Product    string   `json:"product"`
		Version    string   `json:"version"`
		URL        string   `json:"url"`
		Interfaces []string `json:"interfaces"`
	}

	var r reply
	err := c.Call(ctx, "org.varlink.service.GetInfo", nil, &r)
	if err != nil {
		return err
	}

	if vendor != nil {
		*vendor = r.Vendor
	}
	if product != nil {
		*product = r.Product
	}
	if version != nil {
		*version = r.Version
	}
	if url != nil {
		*url = r.URL
	}
	if interfaces != nil {
		*interfaces = r.Interfaces
	}

	return nil
}

// Upgrade attempts to upgrade the connection using the provided method and parameters.
// If successful, the connection cannot be reused later, and must be closed.
func (c *Connection) Upgrade(ctx context.Context, method string, parameters interface{}) (func(context.Context, interface{}) (uint64, ReadWriterContext, error), error) {
	reply, err := c.Send(ctx, method, parameters, Upgrade)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, out interface{}) (uint64, ReadWriterContext, error) {
		flags, err := reply(ctx, out)
		if err != nil {
			return 0, nil, err
		}

		return flags, c.conn, nil
	}, nil
}

// Close terminates the connection.
func (c *Connection) Close() error {
	return c.conn.Close()
}

// NewConnection returns a new connection to the given address. The context
// is used when dialling. Once successfully connected, any expiration
// of the context will not affect the connection.
func NewConnection(ctx context.Context, address string) (*Connection, error) {
	return newConnectionWithDialer(ctx, address, &net.Dialer{})
}

// NewConnectionWithDialer returns a new connection to the given address using a custom dialer.
// The dialer parameter allows using custom network configurations such as:
//   - Dialing through SOCKS or HTTP proxies
//   - Custom timeout and keepalive settings
//   - Dialing through SSH tunnels
//   - Custom DNS resolution
//
// The context is used when dialling. Once successfully connected, any expiration
// of the context will not affect the connection.
//
// Example with custom timeout:
//
//	dialer := &net.Dialer{
//		Timeout:   30 * time.Second,
//		KeepAlive: 30 * time.Second,
//	}
//	conn, err := varlink.NewConnectionWithDialer(ctx, "tcp:localhost:8080", dialer)
func NewConnectionWithDialer(ctx context.Context, address string, dialer ContextDialer) (*Connection, error) {
	if dialer == nil {
		return nil, fmt.Errorf("dialer cannot be nil")
	}
	return newConnectionWithDialer(ctx, address, dialer)
}

// newConnectionWithDialer is the private implementation used by both NewConnection
// and NewConnectionWithDialer.
func newConnectionWithDialer(ctx context.Context, address string, dialer ContextDialer) (*Connection, error) {
	words := strings.SplitN(address, ":", 2)

	if len(words) != 2 {
		return nil, fmt.Errorf("Protocol missing")
	}

	protocol := words[0]
	addr := words[1]

	// Ignore parameters after ';'
	words = strings.SplitN(addr, ";", 2)
	if words != nil {
		addr = words[0]
	}

	switch protocol {
	case "unix":
		break

	case "tcp":
		break

	default:
		return nil, fmt.Errorf("unknown protocol %s", protocol)
	}

	conn, err := dialer.DialContext(ctx, protocol, addr)
	if err != nil {
		return nil, err
	}

	c := Connection{
		address: address,
		conn:    ctxio.NewConn(conn),
	}

	return &c, nil
}
