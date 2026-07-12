package plan9

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/hugelgupf/p9/fsimpl/localfs"
	"github.com/hugelgupf/p9/p9"
	"github.com/sirupsen/logrus"
)

type Server struct {
	server *p9.Server
	// TODO: Once server has a proper Close() we don't need this.
	// This is basically just a short-circuit to actually close the server
	// without that ability.
	listener net.Listener
	// Errors from the server being started will come out here.
	errChan chan error
}

// Expose a single directory (and all children) via the given net.Listener.
// Directory given must be an absolute path and must exist.
func New9pServer(listener net.Listener, exposeDir string) (*Server, error) {
	// Verify that exposeDir makes sense.
	if !filepath.IsAbs(exposeDir) {
		return nil, fmt.Errorf("path to expose to machine must be absolute: %s", exposeDir)
	}
	stat, err := os.Stat(exposeDir)
	if err != nil {
		return nil, fmt.Errorf("cannot stat path to expose to machine: %w", err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("path to expose to machine must be a directory: %s", exposeDir)
	}

	server := p9.NewServer(localfs.Attacher(exposeDir), []p9.ServerOpt{}...)
	if server == nil {
		return nil, fmt.Errorf("p9.NewServer returned nil")
	}

	errChan := make(chan error)

	// TODO: Use a channel to pass back this if it occurs within a
	// reasonable timeframe.
	go func() {
		errChan <- server.Serve(listener)
		close(errChan)
	}()

	toReturn := new(Server)
	toReturn.listener = listener
	toReturn.server = server
	toReturn.errChan = errChan

	// Just before returning, check to see if we got an error off server
	// startup.
	select {
	case err := <-errChan:
		return nil, fmt.Errorf("starting 9p server: %w", err)
	default:
		logrus.Infof("Successfully started 9p server for directory %s", exposeDir)
	}

	return toReturn, nil
}

// Stop a running server.
// Please note that this does *BAD THINGS* to clients if they are still running
// when the server stops. Processes get stuck in I/O deep sleep and zombify, and
// nothing I do save restarting the VM can remove the zombies.
func (s *Server) Stop() error {
	if s.server != nil {
		if err := s.listener.Close(); err != nil {
			return err
		}
		s.server = nil
	}

	return nil
}

// Wait for an error from a running server.
func (s *Server) WaitForError() error {
	if s.server != nil {
		err := <-s.errChan
		return err
	}

	// Server already down, return nil
	return nil
}
