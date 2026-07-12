//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerKubeHandlers(r *mux.Router) error {
	// swagger:operation POST /libpod/play/kube libpod PlayKubeLibpod
	// ---
	// tags:
	//  - containers
	//  - pods
	// summary: Play a Kubernetes YAML file.
	// description: |
	//   Create and run pods based on a Kubernetes YAML file.
	//
	//   ### Content-Type
	//
	//   Then endpoint support two Content-Type
	//    - `plain/text` for yaml format
	//    - `application/x-tar` for sending context(s) required for building images
	//
	//   #### Tar format
	//
	//   The tar format must contain a `play.yaml` file at the root that will be used.
	//   If the file format requires context to build an image, it uses the image name and
	//   check for corresponding folder.
	//
	//   For example, the client sends a tar file with the following structure:
	//
	//   ```
	//   └── content.tar
	//    ├── play.yaml
	//    └── foobar/
	//        └── Containerfile
	//   ```
	//
	//   The `play.yaml` is the following, the `foobar` image means we are looking for a context with this name.
	//   ```
	//   apiVersion: v1
	//   kind: Pod
	//   metadata:
	//   name: demo-build-remote
	//   spec:
	//   containers:
	//    - name: container
	//      image: foobar
	//   ```
	//
	// parameters:
	//  - in: header
	//    name: Content-Type
	//    type: string
	//    default: plain/text
	//    enum: ["plain/text", "application/x-tar"]
	//  - in: query
	//    name: annotations
	//    type: string
	//    description: JSON encoded value of annotations (a map[string]string).
	//  - in: query
	//    name: logDriver
	//    type: string
	//    description: Logging driver for the containers in the pod.
	//  - in: query
	//    name: logOptions
	//    type: array
	//    description: logging driver options
	//    items:
	//         type: string
	//  - in: query
	//    name: network
	//    type: array
	//    description: USe the network mode or specify an array of networks.
	//    items:
	//      type: string
	//  - in: query
	//    name: noHosts
	//    type: boolean
	//    default: false
	//    description: do not setup /etc/hosts file in container
	//  - in: query
	//    name: noTrunc
	//    type: boolean
	//    default: false
	//    description: use annotations that are not truncated to the Kubernetes maximum length of 63 characters
	//  - in: query
	//    name: publishPorts
	//    type: array
	//    description: publish a container's port, or a range of ports, to the host
	//    items:
	//         type: string
	//  - in: query
	//    name: publishAllPorts
	//    type: boolean
	//    description: Whether to publish all ports defined in the K8S YAML file (containerPort, hostPort), if false only hostPort will be published
	//  - in: query
	//    name: replace
	//    type: boolean
	//    default: false
	//    description: replace existing pods and containers
	//  - in: query
	//    name: serviceContainer
	//    type: boolean
	//    default: false
	//    description: Starts a service container before all pods.
	//  - in: query
	//    name: start
	//    type: boolean
	//    default: true
	//    description: Start the pod after creating it.
	//  - in: query
	//    name: staticIPs
	//    type: array
	//    description: Static IPs used for the pods.
	//    items:
	//      type: string
	//  - in: query
	//    name: staticMACs
	//    type: array
	//    description: Static MACs used for the pods.
	//    items:
	//      type: string
	//  - in: query
	//    name: tlsVerify
	//    type: boolean
	//    default: true
	//    description: Require HTTPS and verify signatures when contacting registries.
	//  - in: query
	//    name: userns
	//    type: string
	//    description: Set the user namespace mode for the pods.
	//  - in: query
	//    name: wait
	//    type: boolean
	//    default: false
	//    description: Clean up all objects created when a SIGTERM is received or pods exit.
	//  - in: query
	//    name: build
	//    type: boolean
	//    description: Build the images with corresponding context.
	//  - in: body
	//    name: request
	//    description: Kubernetes YAML file.
	//    schema:
	//      type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/playKubeResponseLibpod"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/play/kube"), s.APIHandler(libpod.PlayKube)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/kube/play"), s.APIHandler(libpod.KubePlay)).Methods(http.MethodPost)
	// swagger:operation DELETE /libpod/play/kube libpod PlayKubeDownLibpod
	// ---
	// tags:
	//  - containers
	//  - pods
	// summary: Remove resources created from kube play
	// description: Tears down pods, secrets, and volumes defined in a YAML file
	// parameters:
	//  - in: query
	//    name: force
	//    type: boolean
	//    default: false
	//    description: Remove volumes.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/playKubeResponseLibpod"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/play/kube"), s.APIHandler(libpod.PlayKubeDown)).Methods(http.MethodDelete)
	r.HandleFunc(VersionedPath("/libpod/kube/play"), s.APIHandler(libpod.KubePlayDown)).Methods(http.MethodDelete)
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
	//  - in: query
	//    name: type
	//    type: string
	//    default: pod
	//    description: Generate YAML for the given Kubernetes kind.
	//  - in: query
	//    name: replicas
	//    type: integer
	//    format: int32
	//    default: 0
	//    description: Set the replica number for Deployment kind.
	//  - in: query
	//    name: noTrunc
	//    type: boolean
	//    default: false
	//    description: don't truncate annotations to the Kubernetes maximum length of 63 characters
	//  - in: query
	//    name: podmanOnly
	//    type: boolean
	//    default: false
	//    description: add podman-only reserved annotations in generated YAML file (cannot be used by Kubernetes)
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
	r.HandleFunc(VersionedPath("/libpod/kube/generate"), s.APIHandler(libpod.KubeGenerate)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/kube/apply libpod KubeApplyLibpod
	// ---
	// tags:
	//  - containers
	//  - pods
	// summary: Apply a podman workload or Kubernetes YAML file.
	// description: Deploy a podman container, pod, volume, or Kubernetes yaml to a Kubernetes cluster.
	// parameters:
	//  - in: query
	//    name: caCertFile
	//    type: string
	//    description: Path to the CA cert file for the Kubernetes cluster.
	//  - in: query
	//    name: kubeConfig
	//    type: string
	//    description: Path to the kubeconfig file for the Kubernetes cluster.
	//  - in: query
	//    name: namespace
	//    type: string
	//    description: The namespace to deploy the workload to on the Kubernetes cluster.
	//  - in: query
	//    name: service
	//    type: boolean
	//    description: Create a service object for the container being deployed.
	//  - in: query
	//    name: file
	//    type: string
	//    description: Path to the Kubernetes yaml file to deploy.
	//  - in: body
	//    name: request
	//    description: Kubernetes YAML file.
	//    schema:
	//      type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: Kubernetes YAML file successfully deployed to cluster
	//     schema:
	//      type: string
	//      format: binary
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/kube/apply"), s.APIHandler(libpod.KubeApply)).Methods(http.MethodPost)
	return nil
}
