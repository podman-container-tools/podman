package socket

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/digitalocean/go-libvirt/internal/constants"
)

const disconnectTimeout = 5 * time.Second

// request and response statuses
const (
	// StatusOK is always set for method calls or events.
	// For replies it indicates successful completion of the method.
	// For streams it indicates confirmation of the end of file on the stream.
	StatusOK = iota

	// StatusError for replies indicates that the method call failed
	// and error information is being returned. For streams this indicates
	// that not all data was sent and the stream has aborted.
	StatusError

	// StatusContinue is only used for streams.
	// This indicates that further data packets will be following.
	StatusContinue
)

// request and response types
const (
	// Call is used when making calls to the remote server.
	Call = iota

	// Reply indicates a server reply.
	Reply

	// Message is an asynchronous notification.
	Message

	// Stream represents a stream data packet.
	Stream

	// CallWithFDs is used by a client to indicate the request has
	// arguments with file descriptors.
	CallWithFDs

	// ReplyWithFDs is used by a server to indicate the request has
	// arguments with file descriptors.
	ReplyWithFDs
)

// Dialer is an interface for connecting to libvirt's underlying socket.
type Dialer interface {
	Dial() (net.Conn, error)
}

// Router is an interface used to route packets to the appropriate clients.
type Router interface {
	Route(*Header, []byte)
}

// Socket represents a libvirt Socket and its connection state
type Socket struct {
	dialer Dialer
	router Router

	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	// used to serialize any Socket writes and any updates to conn, r, or w
	mu *sync.Mutex

	// disconnected is closed when the listen goroutine associated with a
	// Socket connection has returned.
	disconnected chan struct{}
}

// packet represents a RPC request or response.
type packet struct {
	// Size of packet, in bytes, including length.
	// Len + Header + Payload
	Len    uint32
	Header Header
}

// Global packet instance, for use with unsafe.Sizeof()
var _p packet

// Header is a libvirt rpc packet header
type Header struct {
	// Program identifier
	Program uint32

	// Program version
	Version uint32

	// Remote procedure identifier
	Procedure uint32

	// Call type, e.g., Reply
	Type uint32

	// Call serial number
	Serial int32

	// Request status, e.g., StatusOK
	Status uint32
}

// New initializes a new type for managing the Socket.
func New(dialer Dialer, router Router) *Socket {
	s := &Socket{
		dialer:       dialer,
		router:       router,
		disconnected: make(chan struct{}),
		mu:           &sync.Mutex{},
	}

	// we start with a closed channel since that indicates no connection
	close(s.disconnected)

	return s
}

// Connect uses the dialer provided on creation to establish
// underlying physical connection to the desired libvirt.
func (s *Socket) Connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isDisconnected() {
		return errors.New("already connected to socket")
	}
	conn, err := s.dialer.Dial()
	if err != nil {
		return err
	}

	s.conn = conn
	s.reader = bufio.NewReader(conn)
	s.writer = bufio.NewWriter(conn)
	s.disconnected = make(chan struct{})

	go s.listenAndRoute()

	return nil
}

// Disconnect closes the Socket connection to libvirt and waits for the reader
// gorouting to shut down.
func (s *Socket) Disconnect() error {
	// just return if we're already disconnected
	if s.isDisconnected() {
		return nil
	}

	err := s.conn.Close()
	if err != nil {
		return err
	}

	// now we wait for the reader to return so as not to avoid it nil
	// referencing
	// Put this in a select,
	// and have it only nil out the conn value if it doesn't fail
	select {
	case <-s.disconnected:
	case <-time.After(disconnectTimeout):
		return errors.New("timed out waiting for Disconnect cleanup")
	}

	return nil
}

// Disconnected returns a channel that will be closed once the current
// connection is closed.  This can happen due to an explicit call to Disconnect
// from the client, or due to non-temporary Read or Write errors encountered.
func (s *Socket) Disconnected() <-chan struct{} {
	return s.disconnected
}

// isDisconnected is a non-blocking function to query whether a connection
// is disconnected or not.
func (s *Socket) isDisconnected() bool {
	select {
	case <-s.disconnected:
		return true
	default:
		return false
	}
}

// listenAndRoute reads packets from the Socket and calls the provided
// Router function to route them
func (s *Socket) listenAndRoute() {
	// only returns once it detects a non-temporary error related to the
	// underlying connection
	listen(s.reader, s.router)

	// signal any clients listening that the connection has been disconnected
	close(s.disconnected)
}

// listen processes incoming data and routes
// responses to their respective callback handler.
func listen(s io.Reader, router Router) {
	for {
		// response packet length
		length, err := pktlen(s)
		if err != nil {
			if isTemporary(err) {
				continue
			}
			// connection is no longer valid, so shutdown
			return
		}

		// response header
		h, err := extractHeader(s)
		if err != nil {
			// invalid packet
			continue
		}

		// payload: packet length minus what was previously read
		size := int(length) - int(unsafe.Sizeof(_p))
		buf := make([]byte, size)
		_, err = io.ReadFull(s, buf)
		if err != nil {
			// invalid packet
			continue
		}

		// route response to caller
		router.Route(h, buf)
	}
}

// isTemporary returns true if the error returned from a read is transient.
// If the error type is an OpError, check whether the net connection
// error condition is temporary (which means we can keep using the
// connection).
// Errors not of the net.OpError type tend to be things like io.EOF,
// syscall.EINVAL, or io.ErrClosedPipe (i.e. all things that
// indicate the connection in use is no longer valid.)
func isTemporary(err error) bool {
	opErr, ok := err.(*net.OpError)
	if ok {
		return opErr.Temporary()
	}
	return false
}

// pktlen returns the length of an incoming RPC packet.  Read errors will
// result in a returned response length of 0 and a non-nil error.
func pktlen(r io.Reader) (uint32, error) {
	buf := make([]byte, unsafe.Sizeof(_p.Len))

	// extract the packet's length from the header
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(buf), nil
}

// extractHeader returns the decoded header from an incoming response.
func extractHeader(r io.Reader) (*Header, error) {
	buf := make([]byte, unsafe.Sizeof(_p.Header))

	// extract the packet's header from r
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	return &Header{
		Program:   binary.BigEndian.Uint32(buf[0:4]),
		Version:   binary.BigEndian.Uint32(buf[4:8]),
		Procedure: binary.BigEndian.Uint32(buf[8:12]),
		Type:      binary.BigEndian.Uint32(buf[12:16]),
		Serial:    int32(binary.BigEndian.Uint32(buf[16:20])),
		Status:    binary.BigEndian.Uint32(buf[20:24]),
	}, nil
}

// SendPacket sends a packet to libvirt on the socket connection.
func (s *Socket) SendPacket(
	serial int32,
	proc uint32,
	program uint32,
	payload []byte,
	typ uint32,
	status uint32,
) error {
	p := packet{
		Header: Header{
			Program:   program,
			Version:   constants.ProtocolVersion,
			Procedure: proc,
			Type:      typ,
			Serial:    serial,
			Status:    status,
		},
	}

	size := int(unsafe.Sizeof(p.Len)) + int(unsafe.Sizeof(p.Header))
	if payload != nil {
		size += len(payload)
	}
	p.Len = uint32(size)

	if s.isDisconnected() {
		// this mirrors what a lot of net code return on use of a no
		// longer valid connection
		return syscall.EINVAL
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := binary.Write(s.writer, binary.BigEndian, p)
	if err != nil {
		return err
	}

	// write payload
	if payload != nil {
		err = binary.Write(s.writer, binary.BigEndian, payload)
		if err != nil {
			return err
		}
	}

	return s.writer.Flush()
}

// SendStream sends a stream of packets to libvirt on the socket connection.
func (s *Socket) SendStream(serial int32, proc uint32, program uint32,
	stream io.Reader, abort chan bool) error {
	// Keep total packet length under 4 MiB to follow possible limitation in libvirt server code
	buf := make([]byte, 4*MiB-unsafe.Sizeof(_p))
	for {
		select {
		case <-abort:
			return s.SendPacket(serial, proc, program, nil, Stream, StatusError)
		default:
		}
		n, err := stream.Read(buf)
		if n > 0 {
			err2 := s.SendPacket(serial, proc, program, buf[:n], Stream, StatusContinue)
			if err2 != nil {
				return err2
			}
		}
		if err != nil {
			if err == io.EOF {
				return s.SendPacket(serial, proc, program, nil, Stream, StatusOK)
			}
			// keep original error
			err2 := s.SendPacket(serial, proc, program, nil, Stream, StatusError)
			if err2 != nil {
				return err2
			}
			return err
		}
	}
}
