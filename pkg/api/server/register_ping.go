//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
)

func (s *APIServer) registerPingHandlers(r *mux.Router) error {
	r.Handle("/_ping", s.APIHandler(compat.Ping)).Methods(http.MethodGet, http.MethodHead)
	r.Handle(VersionedPath("/_ping"), s.APIHandler(compat.Ping)).Methods(http.MethodGet, http.MethodHead)
	// swagger:operation GET /libpod/_ping libpod SystemPing
	// ---
	//   summary: Ping service
	//   description: |
	//     Return protocol information in response headers.
	//     `HEAD /libpod/_ping` is also supported.
	//     `/_ping` is available for compatibility with other engines.
	//     The '_ping' endpoints are not versioned.
	//   tags:
	//   - system (compat)
	//   - system
	//   produces:
	//   - text/plain
	//   responses:
	//     200:
	//       description: Success
	//       schema:
	//         description: OK
	//         type: string
	//         example: "OK"
	//       headers:
	//         API-Version:
	//           type: string
	//           description: Max compatibility API Version the server supports
	//         BuildKit-Version:
	//           type: string
	//           description: Default version of docker image builder
	//         Docker-Experimental:
	//           type: boolean
	//           description: If the server is running with experimental mode enabled, always true
	//         Cache-Control:
	//           type: string
	//           description: always no-cache
	//         Pragma:
	//           type: string
	//           description: always no-cache
	//         Libpod-API-Version:
	//           type: string
	//           description: |
	//             Max Podman API Version the server supports.
	//             Available if service is backed by Podman, therefore may be used to
	//             determine if talking to Podman engine or another engine
	//         Libpod-Buildah-Version:
	//           type: string
	//           description: |
	//             Default version of libpod image builder.
	//               Available if service is backed by Podman, therefore may be used to
	//               determine if talking to Podman engine or another engine
	//     500:
	//       $ref: "#/responses/internalError"
	r.Handle("/libpod/_ping", s.APIHandler(compat.Ping)).Methods(http.MethodGet, http.MethodHead)
	r.Handle(VersionedPath("/libpod/_ping"), s.APIHandler(compat.Ping)).Methods(http.MethodGet, http.MethodHead)
	return nil
}
