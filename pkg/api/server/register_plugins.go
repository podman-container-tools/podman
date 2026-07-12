//go:build !remote && (linux || freebsd)

package server

import (
	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
)

func (s *APIServer) registerPluginsHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/plugins"), s.APIHandler(compat.UnsupportedHandler))
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/plugins", s.APIHandler(compat.UnsupportedHandler))
	return nil
}
