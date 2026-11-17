//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
)

func (s *APIServer) registerVersionHandlers(r *mux.Router) error {
	// swagger:operation GET /version compat SystemVersion
	// ---
	// summary: Component Version information
	// tags:
	// - system (compat)
	// produces:
	// - application/json
	// responses:
	//   200:
	//    $ref: "#/responses/versionResponse"
	r.Handle("/version", s.APIHandler(compat.VersionHandler)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/version"), s.APIHandler(compat.VersionHandler)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/version libpod SystemVersionLibpod
	// ---
	// summary: Component Version information
	// tags:
	// - system
	// produces:
	// - application/json
	// responses:
	//   200:
	//    $ref: "#/responses/versionResponse"
	r.Handle(VersionedPath("/libpod/version"), s.APIHandler(compat.VersionHandler)).Methods(http.MethodGet)
	return nil
}
