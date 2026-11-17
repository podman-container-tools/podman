//go:build !linux

package splitfdstreamserver

import (
	"errors"
	"os"
)

// Server is not supported on this platform.
type Server struct{}

// NewServer returns a server that cannot serve anything.
func NewServer() *Server { return &Server{} }

// Connection is not supported on this platform.
func (s *Server) Connection(backend ImageBackend, tmpDir string) (*os.File, error) {
	return nil, errors.New("splitfdstream is not supported on this platform")
}

// Stop does nothing.
func (s *Server) Stop() {}
