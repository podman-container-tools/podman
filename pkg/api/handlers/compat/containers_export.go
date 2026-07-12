//go:build !remote && (linux || freebsd)

package compat

import (
	"fmt"
	"net/http"

	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
)

func ExportContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	// set the correct header
	w.Header().Set("Content-Type", "application/x-tar")
	// NOTE: As described in w.Write() it automatically sets the http code to
	// 200 on first write if no other code was set.

	if err := con.Export(w); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to export container: %w", err))
		return
	}
}
