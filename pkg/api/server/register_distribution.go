//go:build !remote && (linux || freebsd)

package server

import (
	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
)

func (s *APIServer) registerDistributionHandlers(r *mux.Router) error {
	r.HandleFunc(VersionedPath("/distribution/{name}/json"), compat.UnsupportedHandler)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/distribution/{name}/json", compat.UnsupportedHandler)
	return nil
}
