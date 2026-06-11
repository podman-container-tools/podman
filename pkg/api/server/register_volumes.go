//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerVolumeHandlers(r *mux.Router) error {
	// swagger:operation POST /libpod/volumes/create libpod VolumeCreateLibpod
	// ---
	// tags:
	//  - volumes
	// summary: Create a volume
	// parameters:
	//  - in: body
	//    name: create
	//    description: attributes for creating a volume
	//    schema:
	//      $ref: "#/definitions/VolumeCreateOptions"
	// produces:
	// - application/json
	// responses:
	//   '201':
	//     $ref: "#/responses/volumeCreateResponse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/volumes/create"), s.APIHandler(libpod.CreateVolume)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/volumes/{name}/exists libpod VolumeExistsLibpod
	// ---
	// tags:
	//  - volumes
	// summary: Volume exists
	// description: Check if a volume exists
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name of the volume
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: volume exists
	//   404:
	//     $ref: '#/responses/volumeNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/volumes/{name}/exists"), s.APIHandler(libpod.ExistsVolume)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/volumes/json libpod VolumeListLibpod
	// ---
	// tags:
	//  - volumes
	// summary: List volumes
	// description: Returns a list of volumes
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a map[string][]string) to process on the volumes list. Available filters:
	//        - driver=<volume-driver-name> Matches volumes based on their driver.
	//        - label=<key> or label=<key>:<value> Matches volumes based on the presence of a label alone or a label and a value.
	//        - name=<volume-name> Matches all of volume name.
	//        - opt=<driver-option> Matches a storage driver options
	//        - `until=<timestamp>` List volumes created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	// responses:
	//   '200':
	//     "$ref": "#/responses/volumeListLibpod"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/volumes/json"), s.APIHandler(libpod.ListVolumes)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/volumes/prune libpod VolumePruneLibpod
	// ---
	// tags:
	//  - volumes
	// summary: Prune volumes
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of filters (a map[string][]string) to match volumes against before pruning.
	//      Available filters:
	//        - `all` When true, prune all unused volumes; when false or unset, only anonymous unused volumes.
	//        - `anonymous` When true/false, restrict to anonymous or named volumes only.
	//        - `until=<timestamp>` Prune volumes created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//        - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune volumes with (or without, in case `label!=...` is used) the specified labels.
	//  - in: query
	//    name: dryrun
	//    type: boolean
	//    required: false
	//    default: false
	//    description: Show which volumes would be pruned without removing them.
	// responses:
	//   '200':
	//      "$ref": "#/responses/volumePruneLibpod"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/volumes/prune"), s.APIHandler(libpod.PruneVolumes)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/volumes/{name}/json libpod VolumeInspectLibpod
	// ---
	// tags:
	//  - volumes
	// summary: Inspect volume
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the volume
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/volumeCreateResponse"
	//   404:
	//     $ref: "#/responses/volumeNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/volumes/{name}/json"), s.APIHandler(libpod.InspectVolume)).Methods(http.MethodGet)
	// swagger:operation DELETE /libpod/volumes/{name} libpod VolumeDeleteLibpod
	// ---
	// tags:
	//  - volumes
	// summary: Remove volume
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the volume
	//  - in: query
	//    name: force
	//    type: boolean
	//    description: force removal
	//  - in: query
	//    name: timeout
	//    type: integer
	//    description: timeout before forcibly killing any containers using the volume
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/volumeNotFound"
	//   409:
	//     description: Volume is in use and cannot be removed
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/volumes/{name}"), s.APIHandler(libpod.RemoveVolume)).Methods(http.MethodDelete)

	// swagger:operation GET /libpod/volumes/{name}/export libpod VolumeExportLibpod
	// ---
	// tags:
	//  - volumes
	// summary: Export a volume
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the volume
	// produces:
	// - application/x-tar
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   404:
	//     $ref: "#/responses/volumeNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/volumes/{name}/export"), s.APIHandler(libpod.ExportVolume)).Methods(http.MethodGet)

	// swagger:operation POST /libpod/volumes/{name}/import libpod VolumeImportLibpod
	// ---
	// tags:
	//  - volumes
	// summary: Populate a volume by importing provided tar
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the volume
	//  - in: body
	//    name: inputStream
	//    description: |
	//      An uncompressed tar archive
	//    schema:
	//      type: string
	//      format: binary
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: Successful import
	//   404:
	//     $ref: "#/responses/volumeNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/volumes/{name}/import"), s.APIHandler(libpod.ImportVolume)).Methods(http.MethodPost)

	// swagger:operation POST /libpod/volumes/{name}/rename libpod VolumeRenameLibpod
	// ---
	// tags:
	//  - volumes
	// summary: Rename an existing volume
	// description: |
	//   Rename a volume when the source volume exists, the new name is valid and unused,
	//   the volume is not mounted or used by any container, and the volume uses the
	//   local driver.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the volume
	//  - in: query
	//    name: newName
	//    type: string
	//    required: true
	//    description: new volume name
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: Volume successfully renamed
	//   404:
	//     $ref: "#/responses/volumeNotFound"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   409:
	//     description: Volume is in use or new name already exists
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/volumes/{name}/rename"), s.APIHandler(libpod.RenameVolume)).Methods(http.MethodPost)

	/*
	 * Docker compatibility endpoints
	 */

	// swagger:operation GET /volumes compat VolumeList
	// ---
	// tags:
	//  - volumes (compat)
	// summary: List volumes
	// description: Returns a list of volume
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a map[string][]string) to process on the volumes list. Available filters:
	//        - driver=<volume-driver-name> Matches volumes based on their driver.
	//        - label=<key> or label=<key>:<value> Matches volumes based on the presence of a label alone or a label and a value.
	//        - name=<volume-name> Matches all of volume name.
	//        - `until=<timestamp>` List volumes created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//
	//      Note:
	//        The boolean `dangling` filter is not yet implemented for this endpoint.
	// responses:
	//   '200':
	//     "$ref": "#/responses/volumeList"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/volumes"), s.APIHandler(compat.ListVolumes)).Methods(http.MethodGet)
	r.Handle("/volumes", s.APIHandler(compat.ListVolumes)).Methods(http.MethodGet)

	// swagger:operation POST /volumes/create compat VolumeCreate
	// ---
	// tags:
	//  - volumes (compat)
	// summary: Create a volume
	// parameters:
	//  - in: body
	//    name: create
	//    description: |
	//      attributes for creating a volume.
	//      Note: If a volume by the same name exists, a 201 response with that volume's information will be generated.
	//    schema:
	//      $ref: "#/definitions/volumeCreate"
	// produces:
	// - application/json
	// responses:
	//   '201':
	//     "$ref": "#/responses/volumeInspect"
	//   '500':
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/volumes/create"), s.APIHandler(compat.CreateVolume)).Methods(http.MethodPost)
	r.Handle("/volumes/create", s.APIHandler(compat.CreateVolume)).Methods(http.MethodPost)

	// swagger:operation GET /volumes/{name} compat VolumeInspect
	// ---
	// tags:
	//  - volumes (compat)
	// summary: Inspect volume
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the volume
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/volumeInspect"
	//   40':
	//     $ref: "#/responses/volumeNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/volumes/{name}"), s.APIHandler(compat.InspectVolume)).Methods(http.MethodGet)
	r.Handle("/volumes/{name}", s.APIHandler(compat.InspectVolume)).Methods(http.MethodGet)

	// swagger:operation DELETE /volumes/{name} compat VolumeDelete
	// ---
	// tags:
	//  - volumes (compat)
	// summary: Remove volume
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the volume
	//  - in: query
	//    name: force
	//    type: boolean
	//    description: |
	//      Force removal of the volume. This actually only causes errors due
	//      to the names volume not being found to be suppressed, which is the
	//      behaviour Docker implements.
	//  - in: query
	//    name: timeout
	//    type: integer
	//    description: timeout before forcibly killing any containers using the volume
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/volumeNotFound"
	//   409:
	//     description: Volume is in use and cannot be removed
	//   500:
	//     "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/volumes/{name}"), s.APIHandler(compat.RemoveVolume)).Methods(http.MethodDelete)
	r.Handle("/volumes/{name}", s.APIHandler(compat.RemoveVolume)).Methods(http.MethodDelete)

	// swagger:operation POST /volumes/prune compat VolumePrune
	// ---
	// tags:
	//  - volumes (compat)
	// summary: Prune volumes
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of filters (a map[string][]string). Docker API 1.42+ - by default only anonymous (unnamed) unused volumes are pruned; use filter all=true to prune all unused volumes.
	//      Available filters:
	//        - `all` When true, prune all unused volumes (anonymous and named). When false or unset, only anonymous unused volumes are pruned.
	//        - `until=<timestamp>` Prune volumes created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//        - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune volumes with (or without, in case `label!=...` is used) the specified labels.
	// responses:
	//   '200':
	//      "$ref": "#/responses/volumePruneResponse"
	//   '500':
	//      "$ref": "#/responses/internalError"
	r.Handle(VersionedPath("/volumes/prune"), s.APIHandler(compat.PruneVolumes)).Methods(http.MethodPost)
	r.Handle("/volumes/prune", s.APIHandler(compat.PruneVolumes)).Methods(http.MethodPost)

	return nil
}
