//go:build !remote && (linux || freebsd)

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerSystemHandlers(r *mux.Router) error {
	// swagger:operation GET /system/df compat SystemDataUsage
	// ---
	// tags:
	//   - system (compat)
	// summary: Show disk usage
	// description: Return information about disk usage for containers, images, and volumes
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/systemDiskUsage'
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/system/df"), s.APIHandler(compat.GetDiskUsage)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/system/df", s.APIHandler(compat.GetDiskUsage)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/system/check libpod SystemCheckLibpod
	// ---
	// tags:
	//   - system
	// summary: Performs consistency checks on storage, optionally removing items which fail checks
	// parameters:
	//   - in: query
	//     name: quick
	//     type: boolean
	//     description: Skip time-consuming checks
	//   - in: query
	//     name: repair
	//     type: boolean
	//     description: Remove inconsistent images
	//   - in: query
	//     name: repair_lossy
	//     type: boolean
	//     description: Remove inconsistent containers and images
	//   - in: query
	//     name: unreferenced_layer_max_age
	//     type: string
	//     description: Maximum allowed age of unreferenced layers
	//     default: 24h0m0s
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/systemCheckResponse'
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/system/check"), s.APIHandler(libpod.SystemCheck)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/system/prune libpod SystemPruneLibpod
	// ---
	// tags:
	//   - system
	// summary: Prune unused data
	// parameters:
	//   - in: query
	//     name: all
	//     type: boolean
	//     description: Remove all unused data, not just dangling data
	//   - in: query
	//     name: volumes
	//     type: boolean
	//     description: Prune volumes
	//   - in: query
	//     name: external
	//     type: boolean
	//     description: Remove images used by external containers (e.g., build containers)
	//   - in: query
	//     name: build
	//     type: boolean
	//     description: Remove build cache
	//   - in: query
	//     name: filters
	//     type: string
	//     description: |
	//       JSON encoded value of filters (a map[string][]string) to match data against before pruning.
	//       Available filters:
	//         - `until=<timestamp>` Prune data created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine's time.
	//         - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune data with (or without, in case `label!=...` is used) the specified labels.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/systemPruneResponse'
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/system/prune"), s.APIHandler(libpod.SystemPrune)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/system/df libpod SystemDataUsageLibpod
	// ---
	// tags:
	//   - system
	// summary: Show disk usage
	// description: Return information about disk usage for containers, images, and volumes
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/systemDiskUsage'
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/system/df"), s.APIHandler(libpod.DiskUsage)).Methods(http.MethodGet)
	return nil
}
