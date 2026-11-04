//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerConfigHandlers(r *mux.Router) error {
	// swagger:operation POST /libpod/configs/create libpod ConfigCreateLibpod
	// ---
	// tags:
	//  - configs
	// summary: Create a config
	// parameters:
	//   - in: query
	//     name: name
	//     type: string
	//     description: User-defined name of the config.
	//     required: true
	//   - in: query
	//     name: driver
	//     type: string
	//     description: Config driver
	//     default: "file"
	//   - in: query
	//     name: driveropts
	//     type: string
	//     description: JSON-encoded string containing config driver options as a `map[string]string`.
	//   - in: query
	//     name: labels
	//     type: string
	//     description: JSON-encoded string containing labels as a `map[string]string`.
	//   - in: query
	//     name: replace
	//     type: boolean
	//     default: false
	//     description: Replace an existing config with the same name.
	//   - in: query
	//     name: ignore
	//     type: boolean
	//     default: false
	//     description: Ignore the request if a config with the same name already exists.
	//   - in: body
	//     name: request
	//     description: Config
	//     schema:
	//       type: string
	// produces:
	// - application/json
	// responses:
	//   '201':
	//     $ref: "#/responses/ConfigCreateResponse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/configs/create"), s.APIHandler(libpod.CreateConfig)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/configs/json libpod ConfigListLibpod
	// ---
	// tags:
	//  - configs
	// summary: List configs
	// description: Returns a list of configs
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a `map[string][]string`) to process on the configs list. Currently available filters:
	//        - `name=[name]` Matches configs name (accepts regex).
	//        - `id=[id]` Matches for full or partial ID.
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     "$ref": "#/responses/ConfigListResponse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/configs/json"), s.APIHandler(compat.ListConfigs)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/configs/{name}/json libpod ConfigInspectLibpod
	// ---
	// tags:
	//  - configs
	// summary: Inspect config
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the config
	//  - in: query
	//    name: showconfig
	//    type: boolean
	//    description: Display Config
	//    default: false
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     "$ref": "#/responses/ConfigInspectResponse"
	//   '404':
	//     "$ref": "#/responses/NoSuchConfig"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/configs/{name}/json"), s.APIHandler(compat.InspectConfig)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/configs/{name}/exists libpod ConfigExistsLibpod
	// ---
	// tags:
	//  - configs
	// summary: Config exists
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the config
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: config exists
	//   404:
	//     $ref: '#/responses/NoSuchConfig'
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/configs/{name}/exists"), s.APIHandler(libpod.ConfigExists)).Methods(http.MethodGet)
	// swagger:operation DELETE /libpod/configs/{name} libpod ConfigDeleteLibpod
	// ---
	// tags:
	//  - configs
	// summary: Remove config
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the config
	//  - in: query
	//    name: all
	//    type: boolean
	//    description: Remove all configs
	//    default: false
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//   '404':
	//     "$ref": "#/responses/NoSuchConfig"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/configs/{name}"), s.APIHandler(compat.RemoveConfig)).Methods(http.MethodDelete)

	/*
	 * Docker compatibility endpoints
	 */
	// swagger:operation GET /configs compat ConfigList
	// ---
	// tags:
	//  - configs (compat)
	// summary: List configs
	// description: Returns a list of configs
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a `map[string][]string`) to process on the configs list. Currently available filters:
	//        - `name=[name]` Matches configs name (accepts regex).
	//        - `id=[id]` Matches for full or partial ID.
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     "$ref": "#/responses/ConfigListCompatResponse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/configs"), s.APIHandler(compat.ListConfigs)).Methods(http.MethodGet)
	r.Handle("/configs", s.APIHandler(compat.ListConfigs)).Methods(http.MethodGet)
	// swagger:operation POST /configs/create compat ConfigCreate
	// ---
	// tags:
	//  - configs (compat)
	// summary: Create a config
	// parameters:
	//  - in: body
	//    name: create
	//    description: |
	//      attributes for creating a config
	//    schema:
	//      $ref: "#/definitions/ConfigCreate"
	// produces:
	// - application/json
	// responses:
	//   '201':
	//     $ref: "#/responses/ConfigCreateResponse"
	//   '409':
	//     "$ref": "#/responses/ConfigInUse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/configs/create"), s.APIHandler(compat.CreateConfig)).Methods(http.MethodPost)
	r.Handle("/configs/create", s.APIHandler(compat.CreateConfig)).Methods(http.MethodPost)
	// swagger:operation GET /configs/{name} compat ConfigInspect
	// ---
	// tags:
	//  - configs (compat)
	// summary: Inspect config
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the config
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     "$ref": "#/responses/ConfigInspectCompatResponse"
	//   '404':
	//     "$ref": "#/responses/NoSuchConfig"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/configs/{name}"), s.APIHandler(compat.InspectConfig)).Methods(http.MethodGet)
	r.Handle("/configs/{name}", s.APIHandler(compat.InspectConfig)).Methods(http.MethodGet)
	// swagger:operation DELETE /configs/{name} compat ConfigDelete
	// ---
	// tags:
	//  - configs (compat)
	// summary: Remove config
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the config
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//   '404':
	//     "$ref": "#/responses/NoSuchConfig"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/configs/{name}"), s.APIHandler(compat.RemoveConfig)).Methods(http.MethodDelete)
	r.Handle("/configs/{name}", s.APIHandler(compat.RemoveConfig)).Methods(http.MethodDelete)

	r.Handle(VersionedPath("/configs/{name}/update"), s.APIHandler(compat.UpdateConfig)).Methods(http.MethodPost)
	r.Handle("/configs/{name}/update", s.APIHandler(compat.UpdateConfig)).Methods(http.MethodPost)
	return nil
}
