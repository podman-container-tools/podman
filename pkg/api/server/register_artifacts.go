//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerArtifactHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/artifacts/{name}/json libpod ArtifactInspectLibpod
	// ---
	// tags:
	//   - artifacts
	// summary: Inspect an artifact
	// description: |
	//   Retrieve detailed information about a specific OCI artifact by name or ID.
	// produces:
	//   - application/json
	// parameters:
	//   - name: name
	//     in: path
	//     description: Name or ID of the artifact
	//     required: true
	//     type: string
	// responses:
	//   200:
	//     $ref: "#/responses/inspectArtifactResponse"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/artifacts/{name:.*}/json"), s.APIHandler(libpod.InspectArtifact)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/artifacts/json libpod ArtifactListLibpod
	// ---
	// tags:
	//   - artifacts
	// summary: List artifacts
	// description: Return a list of all OCI artifacts in local storage.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/artifactListResponse"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/artifacts/json"), s.APIHandler(libpod.ListArtifact)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/artifacts/pull libpod ArtifactPullLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Pull an artifact
	// description: Pull an OCI artifact from a remote registry to local storage.
	// produces:
	// - application/json
	// parameters:
	//   - name: name
	//     in: query
	//     description: Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)
	//     required: true
	//     type: string
	//   - name: retry
	//     in: query
	//     description: Number of times to retry in case of failure when performing pull
	//     type: integer
	//     default: 3
	//   - name: retryDelay
	//     in: query
	//     description: Delay between retries in case of pull failures (e.g., 10s)
	//     type: string
	//     default: 1s
	//   - name: tlsVerify
	//     in: query
	//     description: Require TLS verification
	//     type: boolean
	//     default: true
	//   - name: X-Registry-Auth
	//     in: header
	//     description: |
	//       base-64 encoded auth config.
	//       Must include the following four values: username, password, email and server address
	//       OR simply just an identity token.
	//     type: string
	// responses:
	//   200:
	//     $ref: "#/responses/artifactPullResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   401:
	//     $ref: "#/responses/artifactBadAuth"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/pull"), s.APIHandler(libpod.PullArtifact)).Methods(http.MethodPost)
	// swagger:operation DELETE /libpod/artifacts/remove libpod ArtifactDeleteAllLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Remove one or more artifacts
	// description: |
	//   Remove one or more OCI artifacts from local storage.
	//   Can be filtered by name/ID or all artifacts can be removed.
	// produces:
	//  - application/json
	// parameters:
	//  - name: artifacts
	//    in: query
	//    description: List of artifact names/IDs to remove
	//    type: array
	//    items:
	//        type: string
	//  - name: all
	//    in: query
	//    description: Remove all artifacts
	//    type: boolean
	//  - name: ignore
	//    in: query
	//    description: Ignore errors if artifact does not exist
	//    type: boolean
	// responses:
	//   200:
	//     $ref: "#/responses/artifactRemoveResponse"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/remove"), s.APIHandler(libpod.BatchRemoveArtifact)).Methods(http.MethodDelete)
	// swagger:operation DELETE /libpod/artifacts/{name} libpod ArtifactDeleteLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Remove an artifact
	// description: Remove a single artifact from local storage by name or ID.
	// produces:
	//  - application/json
	// parameters:
	//  - name: name
	//    in: path
	//    description: Name or ID of the artifact to remove
	//    required: true
	//    type: string
	// responses:
	//   200:
	//     $ref: "#/responses/artifactRemoveResponse"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/{name:.*}"), s.APIHandler(libpod.RemoveArtifact)).Methods(http.MethodDelete)
	// swagger:operation POST /libpod/artifacts/add libpod ArtifactAddLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Add a file as an artifact
	// description: |
	//   Add a file as a new OCI artifact, or append to an existing artifact if 'append' is true.
	// produces:
	//   - application/json
	// consumes:
	//   - application/octet-stream
	// parameters:
	//   - name: name
	//     in: query
	//     description: Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)
	//     required: true
	//     type: string
	//   - name: fileName
	//     in: query
	//     description: Path of the file to be added
	//     required: true
	//     type: string
	//   - name: fileMIMEType
	//     in: query
	//     description: Optionally set the type of file
	//     type: string
	//   - name: annotations
	//     in: query
	//     description: Array of annotation strings e.g "test=true"
	//     type: array
	//     items:
	//       type: string
	//   - name: artifactMIMEType
	//     in: query
	//     description: Use type to describe an artifact
	//     type: string
	//   - name: append
	//     in: query
	//     description: Append files to an existing artifact
	//     type: boolean
	//     default: false
	//   - name: replace
	//     in: query
	//     description: Replace an existing artifact with the same name
	//     type: boolean
	//     default: false
	//   - name: inputStream
	//     in: body
	//     description: Binary stream of the file to add to an artifact
	//     schema:
	//       type: string
	//       format: binary
	// responses:
	//   201:
	//     $ref: "#/responses/artifactAddResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/add"), s.APIHandler(libpod.AddArtifact)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/artifacts/local/add libpod ArtifactLocalLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Add a local file as an artifact
	// description: |
	//   Add a file from the local filesystem as a new OCI artifact, or append to an existing artifact if 'append' is true.
	// produces:
	//   - application/json
	// parameters:
	//   - name: name
	//     in: query
	//     description: Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)
	//     required: true
	//     type: string
	//   - name: path
	//     in: query
	//     description: Absolute path to the local file on the server filesystem to be added
	//     required: true
	//     type: string
	//   - name: fileName
	//     in: query
	//     description: Name/title of the file within the artifact
	//     required: true
	//     type: string
	//   - name: fileMIMEType
	//     in: query
	//     description: Optionally set the MIME type of the file
	//     type: string
	//   - name: annotations
	//     in: query
	//     description: Array of annotation strings e.g "test=true"
	//     type: array
	//     items:
	//       type: string
	//   - name: artifactMIMEType
	//     in: query
	//     description: Use type to describe an artifact
	//     type: string
	//   - name: append
	//     in: query
	//     description: Append files to an existing artifact
	//     type: boolean
	//     default: false
	//   - name: replace
	//     in: query
	//     description: Replace an existing artifact with the same name
	//     type: boolean
	//     default: false
	// responses:
	//   201:
	//     $ref: "#/responses/artifactAddResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/local/add"), s.APIHandler(libpod.AddLocalArtifact)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/artifacts/{name}/push libpod ArtifactPushLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Push an artifact
	// description: Push an OCI artifact from local storage to a remote image registry.
	// produces:
	//   - application/json
	// parameters:
	//   - name: name
	//     in: path
	//     description: Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)
	//     required: true
	//     type: string
	//   - name: retry
	//     in: query
	//     description: Number of times to retry in case of failure when performing pull
	//     type: integer
	//     default: 3
	//   - name: retryDelay
	//     in: query
	//     description: Delay between retries in case of pull failures (e.g., 10s)
	//     type: string
	//     default: 1s
	//   - name: tlsVerify
	//     in: query
	//     description: Require TLS verification
	//     type: boolean
	//     default: true
	//   - name: X-Registry-Auth
	//     in: header
	//     description: |
	//       base-64 encoded auth config.
	//       Must include the following four values: username, password, email and server address
	//       OR simply just an identity token.
	//     type: string
	// responses:
	//   200:
	//     $ref: "#/responses/artifactPushResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   401:
	//     $ref: "#/responses/artifactBadAuth"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/{name:.*}/push"), s.APIHandler(libpod.PushArtifact)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/artifacts/{name}/extract libpod ArtifactExtractLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Extract an artifacts contents
	// description: Extract the files of an OCI artifact to the local filesystem as a tar archive.
	// produces:
	//   - application/x-tar
	// parameters:
	//  - name: name
	//    in: path
	//    description: Name or digest of the artifact
	//    required: true
	//    type: string
	//  - name: title
	//    in: query
	//    description: Only extract the file with the given title
	//    type: string
	//  - name: digest
	//    in: query
	//    description: Only extract the file with the given digest
	//    type: string
	//  - name: excludeTitle
	//    in: query
	//    description: |
	//      When extracting a single file from an artifact, don't use the files title as the file name in the tar archive
	//    type: boolean
	// responses:
	//   200:
	//     description: Extract successful
	//     schema:
	//       type: file
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/{name:.*}/extract"), s.APIHandler(libpod.ExtractArtifact)).Methods(http.MethodGet)
	return nil
}
