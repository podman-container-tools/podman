//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerHealthCheckHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/containers/{name}/healthcheck libpod ContainerHealthcheckLibpod
	// ---
	// tags:
	//  - containers
	// summary: Run a container's healthcheck
	// description: Execute the defined healthcheck and return information about the results
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/healthCheck"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   409:
	//     description: container has no healthcheck or is not running
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/containers/{name:.*}/healthcheck"), s.APIHandler(libpod.RunHealthCheck)).Methods(http.MethodGet)
	return nil
}
