//go:build linux

package splitfdstreamserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/varlink/go/varlink"
	"golang.org/x/sys/unix"
)

// Server serves org.composefs.Oci on the connections it hands out.
//
// It exists so that a store can stop serving: a connection being served
// holds the locks of the store it serves from, so the store cannot shut
// down while one is in flight.
type Server struct {
	mu      sync.Mutex
	stopped bool                       // Protected by mu.
	conns   map[*net.UnixConn]struct{} // Protected by mu.
	leases  map[*lease]struct{}        // Protected by mu.
	// wg counts the connections being served, leaseWG the leases
	// outstanding.  They are separate: a connection is done once it has
	// replied, while a lease outlives the reply.
	wg      sync.WaitGroup
	leaseWG sync.WaitGroup
}

// NewServer returns a Server that is not serving any connection.
func NewServer() *Server {
	return &Server{
		conns:  map[*net.UnixConn]struct{}{},
		leases: map[*lease]struct{}{},
	}
}

// Connection returns one end of a new socket pair, and serves
// org.composefs.Oci for one image from backend on the other end, writing
// the streams it hands out to unnamed files in tmpDir.  The caller is
// responsible for closing the returned file.
func (s *Server) Connection(backend ImageBackend, tmpDir string) (*os.File, error) {
	svc, err := varlink.NewService(
		"containers",
		"storage",
		"1.0",
		"https://github.com/containers/container-libs",
	)
	if err != nil {
		return nil, fmt.Errorf("creating varlink service: %w", err)
	}
	if err := svc.RegisterInterface(&ociInterface{server: s, backend: backend, tmpDir: tmpDir}); err != nil {
		return nil, fmt.Errorf("registering interface: %w", err)
	}

	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("creating socket pair: %w", err)
	}
	clientFile := os.NewFile(uintptr(fds[0]), "splitfdstream-client")
	serverFile := os.NewFile(uintptr(fds[1]), "splitfdstream-server")
	serverConn, err := net.FileConn(serverFile)
	serverFile.Close()
	if err != nil {
		clientFile.Close()
		return nil, fmt.Errorf("creating server connection: %w", err)
	}
	conn, ok := serverConn.(*net.UnixConn)
	if !ok {
		clientFile.Close()
		serverConn.Close()
		return nil, fmt.Errorf("unexpected connection type %T", serverConn)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		cancel()
		clientFile.Close()
		conn.Close()
		return nil, errors.New("the store is no longer serving layers")
	}
	s.conns[conn] = struct{}{}
	s.wg.Go(func() {
		defer cancel()
		defer func() {
			s.mu.Lock()
			delete(s.conns, conn)
			s.mu.Unlock()
		}()
		svc.ServeConn(ctx, conn)
	})
	s.mu.Unlock()
	return clientFile, nil
}

// startLease takes over l and waits, in the background, for the consumer
// to be done with the layer.  It reports false, having taken nothing over,
// if the server is already stopping.
func (s *Server) startLease(l *lease) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return false
	}
	s.leases[l] = struct{}{}
	s.leaseWG.Go(func() {
		l.run()
		s.mu.Lock()
		delete(s.leases, l)
		s.mu.Unlock()
	})
	return true
}

// Stop stops serving and waits for what is in flight to finish.
//
// Outstanding leases are cut short rather than waited for: a consumer is
// under no obligation to be quick, and the store cannot shut down while a
// lease holds a layer store lock.  A consumer that had not finished sees
// the names it was given disappear, which it has to handle anyway.
func (s *Server) Stop() {
	s.mu.Lock()
	s.stopped = true
	for conn := range s.conns {
		// Shutting down only the read side lets a request in flight
		// still send its reply.
		if err := conn.CloseRead(); err != nil {
			logrus.Debugf("splitfdstream: shutting down a connection: %v", err)
		}
	}
	leases := make([]*lease, 0, len(s.leases))
	for l := range s.leases {
		leases = append(leases, l)
	}
	s.mu.Unlock()

	// Cancel before waiting for the connections: a request still being
	// served may be blocked acquiring a lock one of these leases holds,
	// and would never reach its reply.
	for _, l := range leases {
		l.cancel()
	}
	s.wg.Wait()
	s.leaseWG.Wait()
}
