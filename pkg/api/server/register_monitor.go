//go:build !remote && (linux || freebsd)

package server

import (
	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
)

func (s *APIServer) registerMonitorHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/monitor"), s.APIHandler(compat.UnsupportedHandler))
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/monitor", s.APIHandler(compat.UnsupportedHandler))
	return nil
}
