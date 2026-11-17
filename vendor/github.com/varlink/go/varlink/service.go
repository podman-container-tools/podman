package varlink

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/varlink/go/varlink/internal/ctxio"
)

type dispatcher interface {
	VarlinkDispatch(ctx context.Context, c Call, methodname string) error
	VarlinkGetName() string
	VarlinkGetDescription() string
}

type serviceCall struct {
	Method     string           `json:"method"`
	Parameters *json.RawMessage `json:"parameters,omitempty"`
	More       bool             `json:"more,omitempty"`
	Oneway     bool             `json:"oneway,omitempty"`
	Upgrade    bool             `json:"upgrade,omitempty"`
}

type serviceReply struct {
	Parameters interface{} `json:"parameters,omitempty"`
	Continues  bool        `json:"continues,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// Service represents an active varlink service. In addition to the registered custom varlink Interfaces, every service
// implements the org.varlink.service interface which allows clients to retrieve information about the
// running service.
type Service struct {
	vendor       string
	product      string
	version      string
	url          string
	interfaces   map[string]dispatcher
	names        []string
	descriptions map[string]string
	running      bool
	listener     net.Listener
	conncounter  int64
	mutex        sync.Mutex
	protocol     string
	address      string
}

// ServiceTimeoutError helps API users to special-case timeouts.
type ServiceTimeoutError struct{}

func (ServiceTimeoutError) Error() string {
	return "service timeout"
}

func (s *Service) getInfo(ctx context.Context, c Call) error {
	return c.replyGetInfo(ctx, s.vendor, s.product, s.version, s.url, s.names)
}

func (s *Service) getInterfaceDescription(ctx context.Context, c Call, name string) error {
	if name == "" {
		return c.ReplyInvalidParameter(ctx, "interface")
	}

	description, ok := s.descriptions[name]
	if !ok {
		return c.ReplyInvalidParameter(ctx, "interface")
	}

	return c.replyGetInterfaceDescription(ctx, description)
}

// HandleMessage dispatches a single varlink message. It is equivalent to
// HandleMessageWithFDs with a nil fds argument.
func (s *Service) HandleMessage(ctx context.Context, conn ReadWriterContext, request []byte) error {
	return s.HandleMessageWithFDs(ctx, conn, request, nil)
}

// HandleMessageWithFDs dispatches a single varlink message, attaching any file
// descriptors received with it to the Call so handlers can retrieve them via
// Call.RequestFDs(). Any fds not claimed by the handler are closed by this function
// to prevent descriptor leaks.
func (s *Service) HandleMessageWithFDs(ctx context.Context, conn ReadWriterContext, request []byte, fds []*os.File) error {
	var in serviceCall

	if err := json.Unmarshal(request, &in); err != nil {
		// Close request fds on malformed-message early exit; nobody will claim them.
		closeFDs(fds)
		return err
	}

	c := Call{
		Conn:    conn,
		In:      &in,
		Request: &request,
		// inFDs is a pointer shared across every value-copy of this Call (see
		// the field doc), so a handler's RequestFDs() is visible here too.
		inFDs: &fds,
	}

	r := strings.LastIndex(in.Method, ".")
	if r <= 0 {
		// Close unconsumed fds before returning.
		closeFDs(c.RequestFDs())
		return c.ReplyInvalidParameter(ctx, "method")
	}

	interfacename := in.Method[:r]
	methodname := in.Method[r+1:]

	var dispErr error
	if interfacename == "org.varlink.service" {
		dispErr = s.orgvarlinkserviceDispatch(ctx, c, methodname)
	} else {
		// Find the interface and method in our service
		iface, ok := s.interfaces[interfacename]
		if !ok {
			closeFDs(c.RequestFDs())
			return c.ReplyInterfaceNotFound(ctx, interfacename)
		}
		dispErr = iface.VarlinkDispatch(ctx, c, methodname)
	}

	// Close any fds the handler did not claim.
	closeFDs(c.RequestFDs())
	return dispErr
}

// closeFDs closes a slice of files, ignoring errors. Used to avoid fd leaks
// when a handler does not claim the received file descriptors.
func closeFDs(fds []*os.File) {
	for _, f := range fds {
		if f != nil {
			f.Close()
		}
	}
}

// Shutdown shuts down the listener of a running service.
func (s *Service) Shutdown() error {
	s.running = false
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}

func (s *Service) handleConnection(ctx context.Context, conn net.Conn, wg *sync.WaitGroup) {
	defer func() { s.mutex.Lock(); s.conncounter--; s.mutex.Unlock(); wg.Done() }()
	s.serveConn(ctx, conn)
}

// ServeConn serves requests on conn, an already connected socket, until
// the peer closes it, a request fails, or ctx is done, and then closes
// conn.  It blocks until then.
//
// It is for connections the Service did not accept itself, such as one end
// of a socket pair, or a socket handed over by another process.  File
// descriptors can be passed if conn is a *net.UnixConn.  Requests on conn
// are served by the interfaces registered with the Service, exactly as on
// connections accepted by Listen, which ServeConn can be used with or
// without.
func (s *Service) ServeConn(ctx context.Context, conn net.Conn) {
	s.mutex.Lock()
	s.conncounter++
	s.mutex.Unlock()
	defer func() { s.mutex.Lock(); s.conncounter--; s.mutex.Unlock() }()
	s.serveConn(ctx, conn)
}

func (s *Service) serveConn(ctx context.Context, conn net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctxConn := ctxio.NewConn(conn)

	for {
		var (
			request []byte
			fds     []*os.File
			err     error
		)

		if ctxConn.CanPassFDs() {
			request, fds, err = ctxConn.ReadBytesWithFDs(ctx, '\x00')
		} else {
			request, err = ctxConn.ReadBytes(ctx, '\x00')
		}
		if err != nil {
			break
		}

		err = s.HandleMessageWithFDs(ctx, ctxConn, request[:len(request)-1], fds)
		if err != nil {
			// FIXME: report error
			// fmt.Fprintf(os.Stderr, "handleMessage: %v", err)
			break
		}
	}

	ctxConn.Close()
}

func (s *Service) teardown() {
	s.mutex.Lock()
	s.listener = nil
	s.running = false
	s.protocol = ""
	s.address = ""
	s.mutex.Unlock()
}

func (s *Service) parseAddress(address string) error {
	words := strings.SplitN(address, ":", 2)
	if len(words) != 2 {
		return fmt.Errorf("Unknown protocol")
	}

	s.protocol = words[0]
	s.address = words[1]

	// Ignore parameters after ';'
	words = strings.SplitN(s.address, ";", 2)
	if words != nil {
		s.address = words[0]
	}

	switch s.protocol {
	case "unix":
		break
	case "tcp":
		break

	default:
		return fmt.Errorf("Unknown protocol")
	}

	return nil
}

func (s *Service) GetListener() (net.Listener, error) {
	s.mutex.Lock()
	l := s.listener
	s.mutex.Unlock()
	return l, nil
}

func (s *Service) setListener(ctx context.Context) error {
	l := activationListener()
	if l == nil {
		if s.protocol == "unix" && s.address[0] != '@' {
			os.Remove(s.address)
		}

		var err error
		l, err = listen(ctx, s.protocol, s.address)
		if err != nil {
			return err
		}

		if s.protocol == "unix" && s.address[0] != '@' {
			l.(*net.UnixListener).SetUnlinkOnClose(true)
		}
	}

	s.mutex.Lock()
	s.listener = l
	s.mutex.Unlock()

	return nil
}

func (s *Service) refreshTimeout(timeout time.Duration) error {
	type setDeadliner interface {
		SetDeadline(time.Time) error
	}
	switch l := s.listener.(type) {
	case setDeadliner:
		if err := l.SetDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
	}
	return nil
}

// Bind binds the service to an address.
func (s *Service) Bind(ctx context.Context, address string) error {
	s.mutex.Lock()
	if s.running {
		s.mutex.Unlock()
		return fmt.Errorf("Init(): already running")
	}
	s.mutex.Unlock()

	s.parseAddress(address)

	err := s.setListener(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Listen starts a Service.
func (s *Service) Listen(ctx context.Context, address string, timeout time.Duration) error {
	var wg sync.WaitGroup
	defer func() { s.teardown(); wg.Wait() }()

	err := s.Bind(ctx, address)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	s.running = true
	l := s.listener
	s.mutex.Unlock()

	for s.running {
		if timeout != 0 {
			if err := s.refreshTimeout(timeout); err != nil {
				return err
			}
		}
		conn, err := l.Accept()
		if err != nil {
			if err.(net.Error).Timeout() {
				s.mutex.Lock()
				if s.conncounter == 0 {
					s.mutex.Unlock()
					return ServiceTimeoutError{}
				}
				s.mutex.Unlock()
				continue
			}
			if !s.running {
				return nil
			}
			return err
		}
		s.mutex.Lock()
		s.conncounter++
		s.mutex.Unlock()
		wg.Add(1)
		go s.handleConnection(ctx, conn, &wg)
	}

	return nil
}

// DoListen starts a Service.
func (s *Service) DoListen(ctx context.Context, timeout time.Duration) error {
	var wg sync.WaitGroup
	defer func() { s.teardown(); wg.Wait() }()

	s.mutex.Lock()
	l := s.listener
	s.mutex.Unlock()

	if l == nil {
		return fmt.Errorf("No listener set")
	}

	s.mutex.Lock()
	s.running = true
	s.mutex.Unlock()

	for s.running {
		if timeout != 0 {
			if err := s.refreshTimeout(timeout); err != nil {
				return err
			}
		}
		conn, err := l.Accept()
		if err != nil {
			if err.(net.Error).Timeout() {
				s.mutex.Lock()
				if s.conncounter == 0 {
					s.mutex.Unlock()
					return ServiceTimeoutError{}
				}
				s.mutex.Unlock()
				continue
			}
			if !s.running {
				return nil
			}
			return err
		}
		s.mutex.Lock()
		s.conncounter++
		s.mutex.Unlock()
		wg.Add(1)
		go s.handleConnection(ctx, conn, &wg)
	}

	return nil
}

// RegisterInterface registers a varlink.Interface containing struct to the Service
func (s *Service) RegisterInterface(iface dispatcher) error {
	name := iface.VarlinkGetName()
	if _, ok := s.interfaces[name]; ok {
		return fmt.Errorf("interface '%s' already registered", name)
	}

	if s.running {
		return fmt.Errorf("service is already running")
	}
	s.interfaces[name] = iface
	s.descriptions[name] = iface.VarlinkGetDescription()
	s.names = append(s.names, name)

	return nil
}

// NewService creates a new Service which implements the list of given varlink interfaces.
func NewService(vendor string, product string, version string, url string) (*Service, error) {
	s := Service{
		vendor:       vendor,
		product:      product,
		version:      version,
		url:          url,
		interfaces:   make(map[string]dispatcher),
		descriptions: make(map[string]string),
	}
	err := s.RegisterInterface(orgvarlinkserviceNew())

	return &s, err
}
