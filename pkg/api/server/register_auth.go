//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
)

func (s *APIServer) registerAuthHandlers(r *mux.Router) error {
	// swagger:operation POST /auth compat SystemAuth
	// ---
	//   summary: Check auth configuration
	//   tags:
	//    - system (compat)
	//   produces:
	//   - application/json
	//   parameters:
	//    - in: body
	//      name: authConfig
	//      description: Authentication to check
	//      schema:
	//        $ref: "#/definitions/AuthConfig"
	//   responses:
	//     200:
	//       $ref: "#/responses/systemAuthResponse"
	//     500:
	//       $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/auth"), s.APIHandler(compat.Auth)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/auth", s.APIHandler(compat.Auth)).Methods(http.MethodPost)
	return nil
}
