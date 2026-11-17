//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
)

func (s *APIServer) registerArchiveHandlers(r *mux.Router) error {
	// swagger:operation PUT /containers/{name}/archive compat PutContainerArchive
	// ---
	//  summary: Put files into a container
	//  description: Put a tar archive of files into a container
	//  tags:
	//   - containers (compat)
	//  produces:
	//  - application/json
	//  parameters:
	//   - in: path
	//     name: name
	//     type: string
	//     description: container name or id
	//     required: true
	//   - in: query
	//     name: path
	//     type: string
	//     description: Path to a directory in the container to extract
	//     required: true
	//   - in: query
	//     name: noOverwriteDirNonDir
	//     type: string
	//     description: if unpacking the given content would cause an existing directory to be replaced with a non-directory and vice versa (1 or true)
	//   - in: query
	//     name: copyUIDGID
	//     type: string
	//     description: copy UID/GID maps to the dest file or di (1 or true)
	//   - in: body
	//     name: request
	//     description: tarfile of files to copy into the container
	//     schema:
	//       type: string
	//  responses:
	//    200:
	//      description: no error
	//    400:
	//      $ref: "#/responses/badParamError"
	//    403:
	//      description: the container rootfs is read-only
	//    404:
	//      $ref: "#/responses/containerNotFound"
	//    500:
	//      $ref: "#/responses/internalError"

	// swagger:operation GET /containers/{name}/archive compat ContainerArchive
	// ---
	//  summary: Get files from a container
	//  description: Get a tar archive of files from a container
	//  tags:
	//   - containers (compat)
	//  produces:
	//  - application/json
	//  parameters:
	//   - in: path
	//     name: name
	//     type: string
	//     description: container name or id
	//     required: true
	//   - in: query
	//     name: path
	//     type: string
	//     description: Path to a directory in the container to extract
	//     required: true
	//  responses:
	//    200:
	//      description: no error
	//      schema:
	//       type: string
	//       format: binary
	//    400:
	//      $ref: "#/responses/badParamError"
	//    404:
	//      $ref: "#/responses/containerNotFound"
	//    500:
	//      $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/archive"), s.APIHandler(compat.Archive)).Methods(http.MethodGet, http.MethodPut, http.MethodHead)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/archive", s.APIHandler(compat.Archive)).Methods(http.MethodGet, http.MethodPut, http.MethodHead)

	/*
		Libpod
	*/

	// swagger:operation PUT /libpod/containers/{name}/archive libpod PutContainerArchiveLibpod
	// ---
	//  summary: Copy files into a container
	//  description: Copy a tar archive of files into a container
	//  tags:
	//   - containers
	//  produces:
	//  - application/json
	//  parameters:
	//   - in: path
	//     name: name
	//     type: string
	//     description: container name or id
	//     required: true
	//   - in: query
	//     name: path
	//     type: string
	//     description: Path to a directory in the container to extract
	//     required: true
	//   - in: query
	//     name: pause
	//     type: boolean
	//     description: pause the container while copying (defaults to true)
	//     default: true
	//   - in: body
	//     name: request
	//     description: tarfile of files to copy into the container
	//     schema:
	//       type: string
	//  responses:
	//    200:
	//      description: no error
	//    400:
	//      $ref: "#/responses/badParamError"
	//    403:
	//      description: the container rootfs is read-only
	//    404:
	//      $ref: "#/responses/containerNotFound"
	//    500:
	//      $ref: "#/responses/internalError"

	// swagger:operation GET /libpod/containers/{name}/archive libpod ContainerArchiveLibpod
	// ---
	//  summary: Copy files from a container
	//  description: Copy a tar archive of files from a container
	//  tags:
	//   - containers (compat)
	//  produces:
	//  - application/json
	//  parameters:
	//   - in: path
	//     name: name
	//     type: string
	//     description: container name or id
	//     required: true
	//   - in: query
	//     name: path
	//     type: string
	//     description: Path to a directory in the container to extract
	//     required: true
	//   - in: query
	//     name: rename
	//     type: string
	//     description: JSON encoded map[string]string to translate paths
	//  responses:
	//    200:
	//      description: no error
	//      schema:
	//       type: string
	//       format: binary
	//    400:
	//      $ref: "#/responses/badParamError"
	//    404:
	//      $ref: "#/responses/containerNotFound"
	//    500:
	//      $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/archive"), s.APIHandler(compat.Archive)).Methods(http.MethodGet, http.MethodPut, http.MethodHead)

	return nil
}
