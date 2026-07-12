//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
)

func (s *APIServer) registerEventsHandlers(r *mux.Router) error {
	// swagger:operation GET /events system SystemEvents
	// ---
	// tags:
	//   - system (compat)
	// summary: Get events
	// description: Returns events filtered on query parameters
	// produces:
	// - application/json
	// parameters:
	// - name: since
	//   type: string
	//   in: query
	//   description: start streaming events from this time
	// - name: until
	//   type: string
	//   in: query
	//   description: stop streaming events later than this
	// - name: filters
	//   type: string
	//   in: query
	//   description: JSON encoded map[string][]string of constraints
	// responses:
	//   200:
	//     description: returns a string of json data describing an event
	//   500:
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/events"), s.StreamBufferedAPIHandler(compat.GetEvents)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/events", s.StreamBufferedAPIHandler(compat.GetEvents)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/events system SystemEventsLibpod
	// ---
	// tags:
	//   - system
	// summary: Get events
	// description: Returns events filtered on query parameters
	// produces:
	// - application/json
	// parameters:
	// - name: since
	//   type: string
	//   in: query
	//   description: start streaming events from this time
	// - name: until
	//   type: string
	//   in: query
	//   description: stop streaming events later than this
	// - name: filters
	//   type: string
	//   in: query
	//   description: JSON encoded map[string][]string of constraints
	// - name: stream
	//   type: boolean
	//   in: query
	//   default: true
	//   description: when false, do not follow events
	// responses:
	//   200:
	//     description: returns a string of json data describing an event
	//   500:
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/events"), s.APIHandler(compat.GetEvents)).Methods(http.MethodGet)
	return nil
}
