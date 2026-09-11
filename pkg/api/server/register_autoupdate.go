//go:build !remote

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/handlers/libpod"
)

func (s *APIServer) registerAutoUpdateHandlers(r *mux.Router) error {
	// swagger:operation POST /libpod/autoupdate libpod AutoupdateLibpod
	// ---
	// tags:
	//   - autoupdate
	// summary: Auto update
	// description: |
	//   Auto update containers according to their auto-update policy.
	//
	//   Auto-update policies are specified with the "io.containers.autoupdate" label.
	//   Containers are expected to run in systemd units created with "podman-generate-systemd --new",
	//   or similar units that create new containers in order to run the updated images.
	//   Please refer to the podman-auto-update(1) man page for details.
	// parameters:
	//   - in: query
	//     name: authfile
	//     type: string
	//     description: Authfile to use when contacting registries.
	//   - in: query
	//     name: dryRun
	//     type: boolean
	//     description: Only check for but do not perform any update. If an update is pending, it will be indicated in the Updated field.
	//   - in: query
	//     name: rollback
	//     type: boolean
	//     description: If restarting the service with the new image failed, restart it another time with the previous image.
	//   - in: query
	//     name: tlsVerify
	//     type: boolean
	//     default: true
	//     description: Require HTTPS and verify signatures when contacting registries.
	// produces:
	//   - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/autoupdateResponse"
	//   500:
	//     $ref: '#/responses/internalError'
	r.HandleFunc(VersionedPath("/libpod/autoupdate"), s.APIHandler(libpod.AutoUpdate)).Methods(http.MethodPost)
	return nil
}
