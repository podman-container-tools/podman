//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerSecretHandlers(r *mux.Router) error {
	// swagger:operation POST /libpod/secrets/create libpod SecretCreateLibpod
	// ---
	// tags:
	//  - secrets
	// summary: Create a secret
	// parameters:
	//   - in: query
	//     name: name
	//     type: string
	//     description: User-defined name of the secret.
	//     required: true
	//   - in: query
	//     name: driver
	//     type: string
	//     description: Secret driver
	//     default: "file"
	//   - in: query
	//     name: driveropts
	//     type: string
	//     description: JSON-encoded string containing secret driver options as a `map[string]string`.
	//   - in: query
	//     name: labels
	//     type: string
	//     description: JSON-encoded string containing labels as a `map[string]string`.
	//   - in: query
	//     name: replace
	//     type: boolean
	//     default: false
	//     description: Replace an existing secret with the same name.
	//   - in: query
	//     name: ignore
	//     type: boolean
	//     default: false
	//     description: Ignore the request if a secret with the same name already exists.
	//   - in: body
	//     name: request
	//     description: Secret
	//     schema:
	//       type: string
	// produces:
	// - application/json
	// responses:
	//   '201':
	//     $ref: "#/responses/SecretCreateResponse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/secrets/create"), s.APIHandler(libpod.CreateSecret)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/secrets/json libpod SecretListLibpod
	// ---
	// tags:
	//  - secrets
	// summary: List secrets
	// description: Returns a list of secrets
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a `map[string][]string`) to process on the secrets list. Currently available filters:
	//        - `name=[name]` Matches secrets name (accepts regex).
	//        - `id=[id]` Matches for full or partial ID.
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     "$ref": "#/responses/SecretListResponse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/secrets/json"), s.APIHandler(compat.ListSecrets)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/secrets/{name}/json libpod SecretInspectLibpod
	// ---
	// tags:
	//  - secrets
	// summary: Inspect secret
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the secret
	//  - in: query
	//    name: showsecret
	//    type: boolean
	//    description: Display Secret
	//    default: false
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     "$ref": "#/responses/SecretInspectResponse"
	//   '404':
	//     "$ref": "#/responses/NoSuchSecret"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/secrets/{name}/json"), s.APIHandler(compat.InspectSecret)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/secrets/{name}/exists libpod SecretExistsLibpod
	// ---
	// tags:
	//  - secrets
	// summary: Secret exists
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the secret
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: secret exists
	//   404:
	//     $ref: '#/responses/NoSuchSecret'
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/secrets/{name}/exists"), s.APIHandler(libpod.SecretExists)).Methods(http.MethodGet)
	// swagger:operation DELETE /libpod/secrets/{name} libpod SecretDeleteLibpod
	// ---
	// tags:
	//  - secrets
	// summary: Remove secret
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the secret
	//  - in: query
	//    name: all
	//    type: boolean
	//    description: Remove all secrets
	//    default: false
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//   '404':
	//     "$ref": "#/responses/NoSuchSecret"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/secrets/{name}"), s.APIHandler(compat.RemoveSecret)).Methods(http.MethodDelete)

	/*
	 * Docker compatibility endpoints
	 */
	// swagger:operation GET /secrets compat SecretList
	// ---
	// tags:
	//  - secrets (compat)
	// summary: List secrets
	// description: Returns a list of secrets
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a `map[string][]string`) to process on the secrets list. Currently available filters:
	//        - `name=[name]` Matches secrets name (accepts regex).
	//        - `id=[id]` Matches for full or partial ID.
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     "$ref": "#/responses/SecretListCompatResponse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/secrets"), s.APIHandler(compat.ListSecrets)).Methods(http.MethodGet)
	r.Handle("/secrets", s.APIHandler(compat.ListSecrets)).Methods(http.MethodGet)
	// swagger:operation POST /secrets/create compat SecretCreate
	// ---
	// tags:
	//  - secrets (compat)
	// summary: Create a secret
	// parameters:
	//  - in: body
	//    name: create
	//    description: |
	//      attributes for creating a secret
	//    schema:
	//      $ref: "#/definitions/SecretCreate"
	// produces:
	// - application/json
	// responses:
	//   '201':
	//     $ref: "#/responses/SecretCreateResponse"
	//   '409':
	//     "$ref": "#/responses/SecretInUse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/secrets/create"), s.APIHandler(compat.CreateSecret)).Methods(http.MethodPost)
	r.Handle("/secrets/create", s.APIHandler(compat.CreateSecret)).Methods(http.MethodPost)
	// swagger:operation GET /secrets/{name} compat SecretInspect
	// ---
	// tags:
	//  - secrets (compat)
	// summary: Inspect secret
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the secret
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     "$ref": "#/responses/SecretInspectCompatResponse"
	//   '404':
	//     "$ref": "#/responses/NoSuchSecret"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/secrets/{name}"), s.APIHandler(compat.InspectSecret)).Methods(http.MethodGet)
	r.Handle("/secrets/{name}", s.APIHandler(compat.InspectSecret)).Methods(http.MethodGet)
	// swagger:operation DELETE /secrets/{name} compat SecretDelete
	// ---
	// tags:
	//  - secrets (compat)
	// summary: Remove secret
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the secret
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//   '404':
	//     "$ref": "#/responses/NoSuchSecret"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/secrets/{name}"), s.APIHandler(compat.RemoveSecret)).Methods(http.MethodDelete)
	r.Handle("/secrets/{name}", s.APIHandler(compat.RemoveSecret)).Methods(http.MethodDelete)

	r.Handle(VersionedPath("/secrets/{name}/update"), s.APIHandler(compat.UpdateSecret)).Methods(http.MethodPost)
	r.Handle("/secrets/{name}/update", s.APIHandler(compat.UpdateSecret)).Methods(http.MethodPost)
	return nil
}
