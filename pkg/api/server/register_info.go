//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerInfoHandlers(r *mux.Router) error {
	// swagger:operation GET /info compat SystemInfo
	// ---
	// tags:
	//  - system (compat)
	// summary: Get info
	// description: Returns information on the system and libpod configuration
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: to be determined
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/info"), s.APIHandler(compat.GetInfo)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/info", s.APIHandler(compat.GetInfo)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/info libpod SystemInfoLibpod
	// ---
	// tags:
	//  - system
	// summary: Get info
	// description: Returns information on the system and libpod configuration
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/infoResponse"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/info"), s.APIHandler(libpod.GetInfo)).Methods(http.MethodGet)
	return nil
}
