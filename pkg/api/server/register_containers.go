//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerContainersHandlers(r *mux.Router) error {
	// swagger:operation POST /containers/create compat ContainerCreate
	// ---
	//   tags:
	//    - containers (compat)
	//   summary: Create a container
	//   produces:
	//   - application/json
	//   parameters:
	//    - in: query
	//      name: name
	//      type: string
	//      description: container name
	//    - in: body
	//      name: body
	//      description: Container to create
	//      schema:
	//        $ref: "#/definitions/CreateContainerConfig"
	//      required: true
	//   responses:
	//     201:
	//       $ref: "#/responses/containerCreateResponse"
	//     400:
	//       $ref: "#/responses/badParamError"
	//     404:
	//       $ref: "#/responses/containerNotFound"
	//     409:
	//       $ref: "#/responses/conflictError"
	//     500:
	//       $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/create"), s.APIHandler(compat.CreateContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/create", s.APIHandler(compat.CreateContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/json compat ContainerList
	// ---
	// tags:
	//  - containers (compat)
	// summary: List containers
	// description: Returns a list of containers
	// parameters:
	//  - in: query
	//    name: all
	//    type: boolean
	//    default: false
	//    description: Return all containers. By default, only running containers are shown
	//  - in: query
	//    name: external
	//    type: boolean
	//    default: false
	//    description: Return containers in storage not controlled by Podman
	//  - in: query
	//    name: limit
	//    description: Return this number of most recently created containers, including non-running ones.
	//    type: integer
	//  - in: query
	//    name: size
	//    type: boolean
	//    default: false
	//    description: Return the size of container as fields SizeRw and SizeRootFs.
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//        A JSON encoded value of the filters (a `map[string][]string`) to process on the containers list. Available filters:
	//        - `ancestor`=(`<image-name>[:<tag>]`, `<image id>`, or `<image@digest>`)
	//        - `annotation`=(`key` or `"key=value"`) of a container annotation
	//        - `before`=(`<container id>` or `<container name>`)
	//        - `exited=<int>` containers with exit code of `<int>`
	//        - `expose`=(`<port>[/<proto>]` or `<startport-endport>/[<proto>]`)
	//        - `health`=(`starting`, `healthy`, `unhealthy` or `none`)
	//        - `id=<ID>` a container's ID
	//        - `is-task`=(`true` or `false`)
	//        - `label`=(`key` or `"key=value"`) of a container label
	//        - `name=<name>` a container's name
	//        - `network`=(`<network id>` or `<network name>`)
	//        - `publish`=(`<port>[/<proto>]` or `<startport-endport>/[<proto>]`)
	//        - `since`=(`<container id>` or `<container name>`)
	//        - `status`=(`created`, `restarting`, `running`, `removing`, `paused`, `exited` or `dead`)
	//        - `volume`=(`<volume name>` or `<mount point destination>`)
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containersList"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/json"), s.APIHandler(compat.ListContainers)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/json", s.APIHandler(compat.ListContainers)).Methods(http.MethodGet)
	// swagger:operation POST  /containers/prune compat ContainerPrune
	// ---
	// tags:
	//   - containers (compat)
	// summary: Delete stopped containers
	// description: Remove containers not in use
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description:  |
	//      Filters to process on the prune list, encoded as JSON (a `map[string][]string`).  Available filters:
	//       - `annotation` (`annotation=<key>`, `annotation=<key>=<value>`, `annotation!=<key>`, or `annotation!=<key>=<value>`) Prune containers with (or without, in case `annotation!=...` is used) the specified annotations.
	//       - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune containers with (or without, in case `label!=...` is used) the specified labels.
	//       - `until=<timestamp>` Prune containers created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containersPrune"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/prune"), s.APIHandler(compat.PruneContainers)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/prune", s.APIHandler(compat.PruneContainers)).Methods(http.MethodPost)
	// swagger:operation DELETE /containers/{name} compat ContainerDelete
	// ---
	// tags:
	//  - containers (compat)
	// summary: Remove a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: force
	//    type: boolean
	//    default: false
	//    description: If the container is running, kill it before removing it.
	//  - in: query
	//    name: v
	//    type: boolean
	//    default: false
	//    description: Remove the volumes associated with the container.
	//  - in: query
	//    name: link
	//    type: boolean
	//    description: not supported
	//  - in: query
	//    name: ignore
	//    type: boolean
	//    default: false
	//    description: Ignore if a specified container does not exist.
	//  - in: query
	//    name: depend
	//    type: boolean
	//    default: false
	//    description: Remove container dependencies.
	//  - in: query
	//    name: timeout
	//    type: integer
	//    description: Number of seconds to wait before forcibly stopping the container.
	//  - in: query
	//    name: volumes
	//    type: boolean
	//    default: false
	//    description: Remove anonymous volumes associated with the container.
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   409:
	//     $ref: "#/responses/conflictError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}"), s.APIHandler(compat.RemoveContainer)).Methods(http.MethodDelete)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}", s.APIHandler(compat.RemoveContainer)).Methods(http.MethodDelete)
	// swagger:operation GET /containers/{name}/json compat ContainerInspect
	// ---
	// tags:
	//  - containers (compat)
	// summary: Inspect container
	// description: Return low-level information about a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or id of the container
	//  - in: query
	//    name: size
	//    type: boolean
	//    default: false
	//    description: include the size of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containerInspectResponse"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/json"), s.APIHandler(compat.GetContainer)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/json", s.APIHandler(compat.GetContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{name}/kill compat ContainerKill
	// ---
	// tags:
	//   - containers (compat)
	// summary: Kill container
	// description: Signal to send to the container as an integer or string (e.g. SIGINT)
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: all
	//    type: boolean
	//    default: false
	//    description: Send kill signal to all containers
	//  - in: query
	//    name: signal
	//    type: string
	//    description: signal to be sent to container
	//    default: SIGKILL
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   409:
	//     $ref: "#/responses/conflictError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/kill"), s.APIHandler(compat.KillContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/kill", s.APIHandler(compat.KillContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/logs compat ContainerLogs
	// ---
	// tags:
	//   - containers (compat)
	// summary: Get container logs
	// description: Get stdout and stderr logs from a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: follow
	//    type: boolean
	//    description: Keep connection after returning logs.
	//  - in: query
	//    name: stdout
	//    type: boolean
	//    description: Return logs from stdout
	//  - in: query
	//    name: stderr
	//    type: boolean
	//    description: Return logs from stderr
	//  - in: query
	//    name: since
	//    type:  string
	//    description: Only return logs since this time, as a UNIX timestamp
	//  - in: query
	//    name: until
	//    type:  string
	//    description: Only return logs before this time, as a UNIX timestamp
	//  - in: query
	//    name: timestamps
	//    type: boolean
	//    default: false
	//    description: Add timestamps to every log line
	//  - in: query
	//    name: tail
	//    type: string
	//    description: Only return this number of log lines from the end of the logs
	//    default: all
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description:  logs returned as a stream in response body.
	//   404:
	//      $ref: "#/responses/containerNotFound"
	//   500:
	//      $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/logs"), s.APIHandler(compat.LogsFromContainer)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/logs", s.APIHandler(compat.LogsFromContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{name}/pause compat ContainerPause
	// ---
	// tags:
	//   - containers (compat)
	// summary: Pause container
	// description: Use the cgroups freezer to suspend all processes in a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/pause"), s.APIHandler(compat.PauseContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/pause", s.APIHandler(compat.PauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/restart compat ContainerRestart
	// ---
	// tags:
	//   - containers (compat)
	// summary: Restart container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: integer
	//    description: timeout before sending kill signal to container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/restart"), s.APIHandler(compat.RestartContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/restart", s.APIHandler(compat.RestartContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/start compat ContainerStart
	// ---
	// tags:
	//   - containers (compat)
	// summary: Start a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    type: string
	//    description: "Override the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _."
	//    default: ctrl-p,ctrl-q
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     $ref: "#/responses/containerAlreadyStartedError"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/start"), s.APIHandler(compat.StartContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/start", s.APIHandler(compat.StartContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/stats compat ContainerStats
	// ---
	// tags:
	//   - containers (compat)
	// summary: Get stats for a container
	// description: This returns a live stream of a container’s resource usage statistics.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: stream
	//    type: boolean
	//    default: true
	//    description: Stream the output
	//  - in: query
	//    name: one-shot
	//    type: boolean
	//    default: false
	//    description: Provide a one-shot response in which preCPU stats are blank, resulting in a single cycle return.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//       type: object
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/stats"), s.StreamBufferedAPIHandler(compat.StatsContainer)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/stats", s.StreamBufferedAPIHandler(compat.StatsContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{name}/stop compat ContainerStop
	// ---
	// tags:
	//   - containers (compat)
	// summary: Stop a container
	// description: Stop a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: integer
	//    description: number of seconds to wait before killing container
	//  - in: query
	//    name: timeout
	//    type: integer
	//    description: Number of seconds to wait before killing the container (libpod alias for `t`).
	//  - in: query
	//    name: ignore
	//    type: boolean
	//    default: false
	//    description: Do not return an error if the container is already stopped.
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     $ref: "#/responses/containerAlreadyStoppedError"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/stop"), s.APIHandler(compat.StopContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/stop", s.APIHandler(compat.StopContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/top compat ContainerTop
	// ---
	// tags:
	//   - containers (compat)
	// summary: List processes running inside a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: ps_args
	//    type: string
	//    default: -ef
	//    description: arguments to pass to ps such as aux.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containerTopResponse"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/top"), s.StreamBufferedAPIHandler(compat.TopContainer)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/top", s.StreamBufferedAPIHandler(compat.TopContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{name}/unpause compat ContainerUnpause
	// ---
	// tags:
	//   - containers (compat)
	// summary: Unpause container
	// description: Resume a paused container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/unpause"), s.APIHandler(compat.UnpauseContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/unpause", s.APIHandler(compat.UnpauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/wait compat ContainerWait
	// ---
	// tags:
	//   - containers (compat)
	// summary: Wait on a container
	// description: |
	//   Block until a container stops or given condition is met.
	//   This is a Docker-compatible endpoint.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: condition
	//    type: string
	//    default: not-running
	//    description: |
	//      Wait condition.  Valid values are:
	//        - not-running (default) - return when the container is not running
	//          (stopped, exited, or was never started).
	//        - next-exit - wait for the next time the container stops.
	//          If the container is running, block until it exits.
	//          If the container is already stopped, block until
	//          the next start-and-exit cycle.
	//        - removed - wait until the container is removed.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containerWaitResponse"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/wait"), s.APIHandler(compat.WaitContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/wait", s.APIHandler(compat.WaitContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/attach compat ContainerAttach
	// ---
	// tags:
	//   - containers (compat)
	// summary: Attach to a container
	// description: |
	//   Attach to a container to read its output or send it input. You can attach
	//   to the same container multiple times and you can reattach to containers
	//   that have been detached.
	//
	//   It uses the same stream format as docker, see the libpod attach endpoint for a description of the format.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    required: false
	//    type: string
	//    description: keys to use for detaching from the container
	//  - in: query
	//    name: logs
	//    required: false
	//    type: boolean
	//    description: Stream all logs from the container across the connection. Happens before streaming attach (if requested). At least one of logs or stream must be set
	//  - in: query
	//    name: stream
	//    required: false
	//    type: boolean
	//    default: true
	//    description: Attach to the container. If unset, and logs is set, only the container's logs will be sent. At least one of stream or logs must be set
	//  - in: query
	//    name: stdout
	//    required: false
	//    type: boolean
	//    description: Attach to container STDOUT
	//  - in: query
	//    name: stderr
	//    required: false
	//    type: boolean
	//    description: Attach to container STDERR
	//  - in: query
	//    name: stdin
	//    required: false
	//    type: boolean
	//    description: Attach to container STDIN
	// produces:
	// - application/json
	// responses:
	//   101:
	//     description: No error, connection has been hijacked for transporting streams.
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/attach"), s.APIHandler(compat.AttachContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/attach", s.APIHandler(compat.AttachContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/resize compat ContainerResize
	// ---
	// tags:
	//  - containers (compat)
	// summary: Resize a container's TTY
	// description: Resize the terminal attached to a container (for use with Attach).
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: h
	//    type: integer
	//    required: false
	//    description: Height to set for the terminal, in characters
	//  - in: query
	//    name: w
	//    type: integer
	//    required: false
	//    description: Width to set for the terminal, in characters
	//  - in: query
	//    name: running
	//    type: boolean
	//    required: false
	//    description: Ignore containers not running errors
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/ok"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/resize"), s.APIHandler(compat.ResizeTTY)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/resize", s.APIHandler(compat.ResizeTTY)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/export compat ContainerExport
	// ---
	// tags:
	//   - containers (compat)
	// summary: Export a container
	// description: Export the contents of a container as a tarball.
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
	//     description: tarball is returned in body
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/export"), s.APIHandler(compat.ExportContainer)).Methods(http.MethodGet)
	r.HandleFunc("/containers/{name}/export", s.APIHandler(compat.ExportContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{name}/rename compat ContainerRename
	// ---
	// tags:
	//   - containers (compat)
	// summary: Rename an existing container
	// description: Change the name of an existing container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: Full or partial ID or full name of the container to rename
	//  - in: query
	//    name: name
	//    type: string
	//    required: true
	//    description: New name for the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   409:
	//     $ref: "#/responses/conflictError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/rename"), s.APIHandler(compat.RenameContainer)).Methods(http.MethodPost)
	r.HandleFunc("/containers/{name}/rename", s.APIHandler(compat.RenameContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/update compat ContainerUpdate
	// ---
	// tags:
	//   - containers (compat)
	// summary: Update configuration of an existing container, allowing changes to resource limits
	// description: Change configuration settings for an existing container without requiring recreation.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: Full or partial ID or full name of the container to rename
	//  - in: body
	//    name: resources
	//    required: false
	//    description: attributes for updating the container
	//    schema:
	//      $ref: "#/definitions/containerUpdateRequest"
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/update"), s.APIHandler(compat.UpdateContainer)).Methods(http.MethodPost)
	r.HandleFunc("/containers/{name}/update", s.APIHandler(compat.UpdateContainer)).Methods(http.MethodPost)

	/*
		libpod endpoints
	*/

	// swagger:operation POST /libpod/containers/create libpod ContainerCreateLibpod
	// ---
	//   tags:
	//    - containers
	//   summary: Create a container
	//   produces:
	//   - application/json
	//   parameters:
	//    - in: body
	//      name: create
	//      description: attributes for creating a container
	//      schema:
	//        $ref: "#/definitions/SpecGenerator"
	//      required: true
	//   responses:
	//     201:
	//       $ref: "#/responses/containerCreateResponse"
	//     400:
	//       $ref: "#/responses/badParamError"
	//     404:
	//       $ref: "#/responses/containerNotFound"
	//     409:
	//       $ref: "#/responses/conflictError"
	//     500:
	//       $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/create"), s.APIHandler(libpod.CreateContainer)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/json libpod ContainerListLibpod
	// ---
	// tags:
	//  - containers
	// summary: List containers
	// description: Returns a list of containers
	// parameters:
	//  - in: query
	//    name: all
	//    type: boolean
	//    default: false
	//    description: Return all containers. By default, only running containers are shown
	//  - in: query
	//    name: limit
	//    description: Return this number of most recently created containers, including non-running ones.
	//    type: integer
	//  - in: query
	//    name: last
	//    description: Alias for `limit`. Return this number of most recently created containers.
	//    type: integer
	//  - in: query
	//    name: external
	//    type: boolean
	//    default: false
	//    description: Return containers created by external tools that use container storage.
	//  - in: query
	//    name: namespace
	//    type: boolean
	//    description: Include namespace information
	//    default: false
	//  - in: query
	//    name: pod
	//    type: boolean
	//    default: false
	//    description: Ignored. Previously included details on pod name and ID that are currently included by default.
	//  - in: query
	//    name: size
	//    type: boolean
	//    default: false
	//    description: Return the size of container as fields SizeRw and SizeRootFs.
	//  - in: query
	//    name: sync
	//    type: boolean
	//    default: false
	//    description: Sync container state with OCI runtime
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//        A JSON encoded value of the filters (a `map[string][]string`) to process on the containers list. Available filters:
	//        - `ancestor`=(`<image-name>[:<tag>]`, `<image id>`, or `<image@digest>`)
	//        - `annotation`=(`key` or `"key=value"`) of a container annotation
	//        - `before`=(`<container id>` or `<container name>`)
	//        - `exited=<int>` containers with exit code of `<int>`
	//        - `expose`=(`<port>[/<proto>]` or `<startport-endport>/[<proto>]`)
	//        - `health`=(`starting`, `healthy`, `unhealthy` or `none`)
	//        - `id=<ID>` a container's ID
	//        - `is-task`=(`true` or `false`)
	//        - `label`=(`key` or `"key=value"`) of a container label
	//        - `name=<name>` a container's name
	//        - `network`=(`<network id>` or `<network name>`)
	//        - `pod`=(`<pod id>` or `<pod name>`)
	//        - `publish`=(`<port>[/<proto>]` or `<startport-endport>/[<proto>]`)
	//        - `since`=(`<container id>` or `<container name>`)
	//        - `status`=(`created`, `restarting`, `running`, `removing`, `paused`, `exited` or `dead`)
	//        - `volume`=(`<volume name>` or `<mount point destination>`)
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containersListLibpod"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/json"), s.APIHandler(libpod.ListContainers)).Methods(http.MethodGet)
	// swagger:operation POST  /libpod/containers/prune libpod ContainerPruneLibpod
	// ---
	// tags:
	//   - containers
	// summary: Delete stopped containers
	// description: Remove containers not in use
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description:  |
	//      Filters to process on the prune list, encoded as JSON (a `map[string][]string`).  Available filters:
	//       - `annotation` (`annotation=<key>`, `annotation=<key>=<value>`, `annotation!=<key>`, or `annotation!=<key>=<value>`) Prune containers with (or without, in case `annotation!=...` is used) the specified annotations.
	//       - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune containers with (or without, in case `label!=...` is used) the specified labels.
	//       - `until=<timestamp>` Prune containers created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containersPruneLibpod"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/prune"), s.APIHandler(compat.PruneContainers)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/showmounted libpod ContainerShowMountedLibpod
	// ---
	// tags:
	//  - containers
	// summary: Show mounted containers
	// description: Lists all mounted containers mount points
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: mounted containers
	//     schema:
	//      type: object
	//      additionalProperties:
	//       type: string
	//   500:
	//      $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/showmounted"), s.APIHandler(libpod.ShowMountedContainers)).Methods(http.MethodGet)
	// swagger:operation DELETE /libpod/containers/{name} libpod ContainerDeleteLibpod
	// ---
	// tags:
	//  - containers
	// summary: Delete container
	// description: Delete container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: depend
	//    type: boolean
	//    description: additionally remove containers that depend on the container to be removed
	//  - in: query
	//    name: force
	//    type: boolean
	//    description: force stop container if running
	//  - in: query
	//    name: ignore
	//    type: boolean
	//    description: ignore errors when the container to be removed does not existxo
	//  - in: query
	//    name: timeout
	//    type: integer
	//    default: 10
	//    description: number of seconds to wait before killing container when force removing
	//  - in: query
	//    name: v
	//    type: boolean
	//    description: delete volumes
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containerRemoveLibpod"
	//   204:
	//     description: no error
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   409:
	//     $ref: "#/responses/conflictError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}"), s.APIHandler(compat.RemoveContainer)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/containers/{name}/json libpod ContainerInspectLibpod
	// ---
	// tags:
	//  - containers
	// summary: Inspect container
	// description: Return low-level information about a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: size
	//    type: boolean
	//    description: display filesystem usage
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containerInspectResponseLibpod"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/json"), s.APIHandler(libpod.GetContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/kill libpod ContainerKillLibpod
	// ---
	// tags:
	//  - containers
	// summary: Kill container
	// description: send a signal to a container, defaults to killing the container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: signal
	//    type: string
	//    default: SIGKILL
	//    description: signal to be sent to container, either by integer or SIG_ name
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   409:
	//     $ref: "#/responses/conflictError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/kill"), s.APIHandler(compat.KillContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/mount libpod ContainerMountLibpod
	// ---
	// tags:
	//  - containers
	// summary: Mount a container
	// description: Mount a container to the filesystem
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: external
	//    type: boolean
	//    default: false
	//    description: Include external containers that are not managed by Podman.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: mounted container
	//     schema:
	//      description: id
	//      type: string
	//      example: /var/lib/containers/storage/overlay/f3f693bd88872a1e3193f4ebb925f4c282e8e73aadb8ab3e7492754dda3a02a4/merged
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/mount"), s.APIHandler(libpod.MountContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/unmount libpod ContainerUnmountLibpod
	// ---
	// tags:
	//  - containers
	// summary: Unmount a container
	// description: Unmount a container from the filesystem
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// responses:
	//   204:
	//     description: ok
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/unmount"), s.APIHandler(libpod.UnmountContainer)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/{name}/logs libpod ContainerLogsLibpod
	// ---
	// tags:
	//   - containers
	// summary: Get container logs
	// description: |
	//   Get stdout and stderr logs from a container.
	//
	//   The stream format is the same as described in the attach endpoint.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: follow
	//    type: boolean
	//    description: Keep connection after returning logs.
	//  - in: query
	//    name: stdout
	//    type: boolean
	//    description: Return logs from stdout
	//  - in: query
	//    name: stderr
	//    type: boolean
	//    description: Return logs from stderr
	//  - in: query
	//    name: since
	//    type:  string
	//    description: Only return logs since this time, as a UNIX timestamp
	//  - in: query
	//    name: until
	//    type:  string
	//    description: Only return logs before this time, as a UNIX timestamp
	//  - in: query
	//    name: timestamps
	//    type: boolean
	//    default: false
	//    description: Add timestamps to every log line
	//  - in: query
	//    name: tail
	//    type: string
	//    description: Only return this number of log lines from the end of the logs
	//    default: all
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description:  logs returned as a stream in response body.
	//   404:
	//      $ref: "#/responses/containerNotFound"
	//   500:
	//      $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/logs"), s.APIHandler(compat.LogsFromContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/pause libpod ContainerPauseLibpod
	// ---
	// tags:
	//  - containers
	// summary: Pause a container
	// description: Use the cgroups freezer to suspend all processes in a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/pause"), s.APIHandler(compat.PauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/restart libpod ContainerRestartLibpod
	// ---
	// tags:
	//  - containers
	// summary: Restart a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: integer
	//    description: number of seconds to wait before killing container (Docker compatibility)
	//  - in: query
	//    name: timeout
	//    type: integer
	//    description: number of seconds to wait before killing container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/restart"), s.APIHandler(compat.RestartContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/start libpod ContainerStartLibpod
	// ---
	// tags:
	//  - containers
	// summary: Start a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    type: string
	//    description: "Override the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _."
	//    default: ctrl-p,ctrl-q
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     $ref: "#/responses/containerAlreadyStartedError"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/start"), s.APIHandler(compat.StartContainer)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/{name}/stats libpod ContainerStatsLibpod
	// ---
	// tags:
	//  - containers
	// summary: Get stats for a container
	// description: DEPRECATED. This endpoint will be removed with the next major release. Please use /libpod/containers/stats instead.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: stream
	//    type: boolean
	//    default: true
	//    description: Stream the output
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   409:
	//     $ref: "#/responses/conflictError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/stats"), s.APIHandler(compat.StatsContainer)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/containers/stats libpod ContainersStatsAllLibpod
	// ---
	// tags:
	//  - containers
	// summary: Get stats for one or more containers
	// description: Return a live stream of resource usage statistics of one or more container. If no container is specified, the statistics of all containers are returned.
	// parameters:
	//  - in: query
	//    name: containers
	//    description: names or IDs of containers
	//    type: array
	//    items:
	//       type: string
	//  - in: query
	//    name: stream
	//    type: boolean
	//    default: true
	//    description: Stream the output
	//  - in: query
	//    name: interval
	//    type: integer
	//    default: 5
	//    description: Time in seconds between stats reports
	//  - in: query
	//    name: all
	//    type: boolean
	//    default: false
	//    description: Provide statistics for all running containers
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containerStats"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/stats"), s.APIHandler(libpod.StatsContainer)).Methods(http.MethodGet)

	// swagger:operation GET /libpod/containers/{name}/top libpod ContainerTopLibpod
	// ---
	// tags:
	//  - containers
	// summary: List processes
	// description: List processes running inside a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: Name of container to query for processes (As of version 1.xx)
	//  - in: query
	//    name: stream
	//    type: boolean
	//    description: when true, repeatedly stream the latest output (As of version 4.0)
	//  - in: query
	//    name: delay
	//    type: integer
	//    description: if streaming, delay in seconds between updates. Must be >1. (As of version 4.0)
	//    default: 5
	//  - in: query
	//    name: ps_args
	//    type: array
	//    items:
	//       type: string
	//    description: |
	//      arguments to pass to ps such as aux.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/containerTopResponse"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/top"), s.APIHandler(compat.TopContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/unpause libpod ContainerUnpauseLibpod
	// ---
	// tags:
	//  - containers
	// summary: Unpause Container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/unpause"), s.APIHandler(compat.UnpauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/wait libpod ContainerWaitLibpod
	// ---
	// tags:
	//  - containers
	// summary: Wait on a container
	// description: Wait on a container to meet a given condition
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: condition
	//    type: array
	//    items:
	//      type: string
	//      enum:
	//       - configured
	//       - created
	//       - exited
	//       - healthy
	//       - initialized
	//       - paused
	//       - removing
	//       - running
	//       - stopped
	//       - stopping
	//       - unhealthy
	//    description: "Conditions to wait for. If no condition provided the 'exited' condition is assumed."
	//  - in: query
	//    name: interval
	//    type: string
	//    default: "250ms"
	//    description: Time Interval to wait before polling for completion.
	// produces:
	// - application/json
	// - text/plain
	// responses:
	//   200:
	//     description: Status code
	//     schema:
	//       type: integer
	//       format: int32
	//     examples:
	//       text/plain: 137
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/wait"), s.APIHandler(libpod.WaitContainer)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/{name}/exists libpod ContainerExistsLibpod
	// ---
	// tags:
	//  - containers
	// summary: Check if container exists
	// description: Quick way to determine if a container exists by name or ID
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: container exists
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/exists"), s.APIHandler(libpod.ContainerExists)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/stop libpod ContainerStopLibpod
	// ---
	// tags:
	//  - containers
	// summary: Stop a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: timeout
	//    type: integer
	//    default: 10
	//    description: number of seconds to wait before killing container
	//  - in: query
	//    name: ignore
	//    type: boolean
	//    default: false
	//    description: do not return error if container is already stopped
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     $ref: "#/responses/containerAlreadyStoppedError"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/stop"), s.APIHandler(compat.StopContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/attach libpod ContainerAttachLibpod
	// ---
	// tags:
	//   - containers
	// summary: Attach to a container
	// description: |
	//   Attach to a container to read its output or send it input. You can attach
	//   to the same container multiple times and you can reattach to containers
	//   that have been detached.
	//
	//   ### Hijacking
	//
	//   This endpoint hijacks the HTTP connection to transport `stdin`, `stdout`,
	//   and `stderr` on the same socket.
	//
	//   This is the response from the service for an attach request:
	//
	//   ```
	//   HTTP/1.1 200 OK
	//   Content-Type: application/vnd.docker.raw-stream
	//
	//   [STREAM]
	//   ```
	//
	//   After the headers and two new lines, the TCP connection can now be used
	//   for raw, bidirectional communication between the client and server.
	//
	//   To inform potential proxies about connection hijacking, the client
	//   can also optionally send connection upgrade headers.
	//
	//   For example, the client sends this request to upgrade the connection:
	//
	//   ```
	//   POST /v4.6.0/libpod/containers/16253994b7c4/attach?stream=1&stdout=1 HTTP/1.1
	//   Upgrade: tcp
	//   Connection: Upgrade
	//   ```
	//
	//   The service will respond with a `101 UPGRADED` response, and will
	//   similarly follow with the raw stream:
	//
	//   ```
	//   HTTP/1.1 101 UPGRADED
	//   Content-Type: application/vnd.docker.raw-stream
	//   Connection: Upgrade
	//   Upgrade: tcp
	//
	//   [STREAM]
	//   ```
	//
	//   ### Stream format
	//
	//   When the TTY setting is disabled for the container,
	//   the HTTP Content-Type header is set to application/vnd.docker.multiplexed-stream
	//   (starting with v4.7.0, previously application/vnd.docker.raw-stream was always used)
	//   and the stream over the hijacked connected is multiplexed to separate out
	//   `stdout` and `stderr`. The stream consists of a series of frames, each
	//   containing a header and a payload.
	//
	//   The header contains the information about the output stream type and the size of
	//   the payload.
	//   It is encoded on the first eight bytes like this:
	//
	//   ```go
	//   header := [8]byte{STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4}
	//   ```
	//
	//   `STREAM_TYPE` can be:
	//
	//   - 0: `stdin` (is written on `stdout`)
	//   - 1: `stdout`
	//   - 2: `stderr`
	//
	//   `SIZE1, SIZE2, SIZE3, SIZE4` are the four bytes of the `uint32` size
	//   encoded as big endian.
	//
	//   Following the header is the payload, which contains the specified number of
	//   bytes as written in the size.
	//
	//   The simplest way to implement this protocol is the following:
	//
	//   1. Read 8 bytes.
	//   2. Choose `stdout` or `stderr` depending on the first byte.
	//   3. Extract the frame size from the last four bytes.
	//   4. Read the extracted size and output it on the correct output.
	//   5. Goto 1.
	//
	//   ### Stream format when using a TTY
	//
	//   When the TTY setting is enabled for the container,
	//   the stream is not multiplexed. The data exchanged over the hijacked
	//   connection is simply the raw data from the process PTY and client's
	//   `stdin`.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    required: false
	//    type: string
	//    description: keys to use for detaching from the container
	//  - in: query
	//    name: logs
	//    required: false
	//    type: boolean
	//    description: Stream all logs from the container across the connection. Happens before streaming attach (if requested). At least one of logs or stream must be set
	//  - in: query
	//    name: stream
	//    required: false
	//    type: boolean
	//    default: true
	//    description: Attach to the container. If unset, and logs is set, only the container's logs will be sent. At least one of stream or logs must be set
	//  - in: query
	//    name: stdout
	//    required: false
	//    type: boolean
	//    description: Attach to container STDOUT
	//  - in: query
	//    name: stderr
	//    required: false
	//    type: boolean
	//    description: Attach to container STDERR
	//  - in: query
	//    name: stdin
	//    required: false
	//    type: boolean
	//    description: Attach to container STDIN
	// produces:
	// - application/json
	// responses:
	//   101:
	//     description: No error, connection has been hijacked for transporting streams.
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/attach"), s.APIHandler(compat.AttachContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/resize libpod ContainerResizeLibpod
	// ---
	// tags:
	//  - containers
	// summary: Resize a container's TTY
	// description: Resize the terminal attached to a container (for use with Attach).
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: h
	//    type: integer
	//    required: false
	//    description: Height to set for the terminal, in characters
	//  - in: query
	//    name: w
	//    type: integer
	//    required: false
	//    description: Width to set for the terminal, in characters
	//  - in: query
	//    name: running
	//    type: boolean
	//    required: false
	//    description: Ignore containers not running errors
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/ok"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   409:
	//     $ref: "#/responses/conflictError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/resize"), s.APIHandler(compat.ResizeTTY)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/{name}/export libpod ContainerExportLibpod
	// ---
	// tags:
	//   - containers
	// summary: Export a container
	// description: Export the contents of a container as a tarball.
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
	//     description: tarball is returned in body
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/export"), s.APIHandler(compat.ExportContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/checkpoint libpod ContainerCheckpointLibpod
	// ---
	// tags:
	//   - containers
	// summary: Checkpoint a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: keep
	//    type: boolean
	//    description: keep all temporary checkpoint files
	//  - in: query
	//    name: leaveRunning
	//    type: boolean
	//    description: leave the container running after writing checkpoint to disk
	//  - in: query
	//    name: tcpEstablished
	//    type: boolean
	//    description: checkpoint a container with established TCP connections
	//  - in: query
	//    name: export
	//    type: boolean
	//    description:  export the checkpoint image to a tar.gz
	//  - in: query
	//    name: ignoreRootFS
	//    type: boolean
	//    description: do not include root file-system changes when exporting. can only be used with export
	//  - in: query
	//    name: ignoreVolumes
	//    type: boolean
	//    description: do not include associated volumes. can only be used with export
	//  - in: query
	//    name: preCheckpoint
	//    type: boolean
	//    description: dump the container's memory information only, leaving the container running. only works on runc 1.0-rc or higher
	//  - in: query
	//    name: withPrevious
	//    type: boolean
	//    description: check out the container with previous criu image files in pre-dump. only works on runc 1.0-rc or higher
	//  - in: query
	//    name: fileLocks
	//    type: boolean
	//    description: checkpoint a container with filelocks
	//  - in: query
	//    name: printStats
	//    type: boolean
	//    description: add checkpoint statistics to the returned CheckpointReport
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: tarball is returned in body if exported
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/checkpoint"), s.APIHandler(libpod.Checkpoint)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/restore libpod ContainerRestoreLibpod
	// ---
	// tags:
	//   - containers
	// summary: Restore a container
	// description: Restore a container from a checkpoint.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or id of the container
	//  - in: query
	//    name: name
	//    type: string
	//    description: the name of the container when restored from a tar. can only be used with import
	//  - in: query
	//    name: keep
	//    type: boolean
	//    description: keep all temporary checkpoint files
	//  - in: query
	//    name: tcpEstablished
	//    type: boolean
	//    description: restore a container with established TCP connections
	//  - in: query
	//    name: tcpClose
	//    type: boolean
	//    description: restore a container but close the TCP connections
	//  - in: query
	//    name: import
	//    type: boolean
	//    description:  import the restore from a checkpoint tar.gz
	//  - in: query
	//    name: ignoreRootFS
	//    type: boolean
	//    description: do not include root file-system changes when exporting. can only be used with import
	//  - in: query
	//    name: ignoreVolumes
	//    type: boolean
	//    description: do not restore associated volumes. can only be used with import
	//  - in: query
	//    name: ignoreStaticIP
	//    type: boolean
	//    description: ignore IP address if set statically
	//  - in: query
	//    name: ignoreStaticMAC
	//    type: boolean
	//    description: ignore MAC address if set statically
	//  - in: query
	//    name: fileLocks
	//    type: boolean
	//    description: restore a container with file locks
	//  - in: query
	//    name: printStats
	//    type: boolean
	//    description: add restore statistics to the returned RestoreReport
	//  - in: query
	//    name: pod
	//    type: string
	//    description: pod to restore into
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: tarball is returned in body if exported
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/restore"), s.APIHandler(libpod.Restore)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/changes compat ContainerChanges
	// ---
	// tags:
	//   - containers (compat)
	// summary: Report on changes to container's filesystem; adds, deletes or modifications.
	// description: |
	//   Returns which files in a container's filesystem have been added, deleted, or modified. The Kind of modification can be one of:
	//
	//   0: Modified
	//   1: Added
	//   2: Deleted
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or id of the container
	//  - in: query
	//    name: parent
	//    type: string
	//    description: specify a second layer which is used to compare against it instead of the parent layer
	//  - in: query
	//    name: diffType
	//    type: string
	//    enum: [all, container, image]
	//    description: select what you want to match, default is all
	// responses:
	//   200:
	//     description: Array of Changes
	//     content:
	//       application/json:
	//       schema:
	//         $ref: "#/responses/Changes"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/containers/{name}/changes"), s.APIHandler(compat.Changes)).Methods(http.MethodGet)
	r.HandleFunc("/containers/{name}/changes", s.APIHandler(compat.Changes)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/containers/{name}/changes libpod ContainerChangesLibpod
	// ---
	// tags:
	//   - containers
	// summary: Report on changes to container's filesystem; adds, deletes or modifications.
	// description: |
	//   Returns which files in a container's filesystem have been added, deleted, or modified. The Kind of modification can be one of:
	//
	//   0: Modified
	//   1: Added
	//   2: Deleted
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or id of the container
	//  - in: query
	//    name: parent
	//    type: string
	//    description: specify a second layer which is used to compare against it instead of the parent layer
	//  - in: query
	//    name: diffType
	//    type: string
	//    enum: [all, container, image]
	//    description: select what you want to match, default is all
	// responses:
	//   200:
	//     description: Array of Changes
	//     content:
	//       application/json:
	//       schema:
	//         $ref: "#/responses/Changes"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/changes"), s.APIHandler(compat.Changes)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/init libpod ContainerInitLibpod
	// ---
	// tags:
	//  - containers
	// summary: Initialize a container
	// description: Performs all tasks necessary for initializing the container but does not start the container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     description: container already initialized
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/init"), s.APIHandler(libpod.InitContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/rename libpod ContainerRenameLibpod
	// ---
	// tags:
	//   - containers
	// summary: Rename an existing container
	// description: Change the name of an existing container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: Full or partial ID or full name of the container to rename
	//  - in: query
	//    name: name
	//    type: string
	//    required: true
	//    description: New name for the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   409:
	//     $ref: "#/responses/conflictError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/rename"), s.APIHandler(compat.RenameContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/update libpod ContainerUpdateLibpod
	// ---
	// tags:
	//   - containers
	// summary: Updates the configuration of an existing container, allowing changes to resource limits and healthchecks
	// description: Updates the configuration of an existing container, allowing changes to resource limits and healthchecks.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: Full or partial ID or full name of the container to update
	//  - in: query
	//    name: restartPolicy
	//    type: string
	//    required: false
	//    description: New restart policy for the container.
	//  - in: query
	//    name: restartRetries
	//    type: integer
	//    required: false
	//    description: New amount of retries for the container's restart policy. Only allowed if restartPolicy is set to on-failure
	//  - in: body
	//    name: config
	//    description: attributes for updating the container
	//    schema:
	//      $ref: "#/definitions/UpdateEntities"
	// produces:
	// - application/json
	// responses:
	//   201:
	//     $ref: "#/responses/containerUpdateResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/update"), s.APIHandler(libpod.UpdateContainer)).Methods(http.MethodPost)
	return nil
}
