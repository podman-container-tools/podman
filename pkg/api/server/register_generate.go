//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerGenerateHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/generate/{name}/systemd libpod GenerateSystemdLibpod
	// ---
	// tags:
	//  - containers
	//  - pods
	// summary: Generate Systemd Units
	// description: Generate Systemd Units based on a pod or container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: Name or ID of the container or pod.
	//  - in: query
	//    name: useName
	//    type: boolean
	//    default: false
	//    description: Use container/pod names instead of IDs.
	//  - in: query
	//    name: new
	//    type: boolean
	//    default: false
	//    description: Create a new container instead of starting an existing one.
	//  - in: query
	//    name: noHeader
	//    type: boolean
	//    default: false
	//    description: Do not generate the header including the Podman version and the timestamp.
	//  - in: query
	//    name: startTimeout
	//    type: integer
	//    default: 0
	//    description: Start timeout in seconds.
	//  - in: query
	//    name: stopTimeout
	//    type: integer
	//    default: 10
	//    description: Stop timeout in seconds.
	//  - in: query
	//    name: restartPolicy
	//    default: on-failure
	//    type: string
	//    enum: ["no", on-success, on-failure, on-abnormal, on-watchdog, on-abort, always]
	//    description: Systemd restart-policy.
	//  - in: query
	//    name: containerPrefix
	//    type: string
	//    default: container
	//    description: Systemd unit name prefix for containers.
	//  - in: query
	//    name: podPrefix
	//    type: string
	//    default: pod
	//    description: Systemd unit name prefix for pods.
	//  - in: query
	//    name: separator
	//    type: string
	//    default: "-"
	//    description: Systemd unit name separator between name/id and prefix.
	//  - in: query
	//    name: restartSec
	//    type: integer
	//    default: 0
	//    description: Configures the time to sleep before restarting a service.
	//  - in: query
	//    name: wants
	//    type: array
	//    items:
	//        type: string
	//    default: []
	//    description: Systemd Wants list for the container or pods.
	//  - in: query
	//    name: after
	//    type: array
	//    items:
	//        type: string
	//    default: []
	//    description: Systemd After list for the container or pods.
	//  - in: query
	//    name: requires
	//    type: array
	//    items:
	//        type: string
	//    default: []
	//    description: Systemd Requires list for the container or pods.
	//  - in: query
	//    name: additionalEnvVariables
	//    type: array
	//    items:
	//        type: string
	//    default: []
	//    description: Set environment variables to the systemd unit files.
	//  - in: query
	//    name: templateUnitFile
	//    type: boolean
	//    default: false
	//    description: Add template specifier for the systemd unit file names.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//       type: object
	//       additionalProperties:
	//         type: string
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/generate/{name:.*}/systemd"), s.APIHandler(libpod.GenerateSystemd)).Methods(http.MethodGet)

	// swagger:operation GET /libpod/generate/kube libpod GenerateKubeLibpod
	// ---
	// tags:
	//  - containers
	//  - pods
	// summary: Generate a Kubernetes YAML file.
	// description: Generate Kubernetes YAML based on a pod or container.
	// parameters:
	//  - in: query
	//    name: names
	//    type: array
	//    items:
	//       type: string
	//    required: true
	//    description: Name or ID of the container or pod.
	//  - in: query
	//    name: service
	//    type: boolean
	//    default: false
	//    description: Generate YAML for a Kubernetes service object.
	// produces:
	// - text/vnd.yaml
	// - application/json
	// responses:
	//   200:
	//     description: Kubernetes YAML file describing pod
	//     schema:
	//      type: string
	//      format: binary
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/generate/kube"), s.APIHandler(libpod.GenerateKube)).Methods(http.MethodGet)
	return nil
}
