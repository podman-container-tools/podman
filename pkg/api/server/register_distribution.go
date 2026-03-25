//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
)

func (s *APIServer) registerDistributionHandlers(r *mux.Router) error {
	// swagger:operation GET /distribution/{name}/json compat DistributionInspect
	// ---
	// tags:
	//  - distribution (compat)
	// summary: Get image information from the registry
	// description: Return image digest and platform information by contacting the registry.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name of the image
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/distributionInspectResponse"
	//   401:
	//     $ref: "#/responses/distributionUnauthorized"
	//   404:
	//     $ref: "#/responses/imageNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/distribution/{name:.*}/json"), s.APIHandler(compat.DistributionInspect)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/distribution/{name:.*}/json", s.APIHandler(compat.DistributionInspect)).Methods(http.MethodGet)
	return nil
}
