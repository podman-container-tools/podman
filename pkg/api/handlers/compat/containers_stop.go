//go:build !remote && (linux || freebsd)

package compat

import (
	"errors"
	"fmt"
	"net/http"

	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/util"
)

func StopContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	// /{version}/containers/(name)/stop
	query := struct {
		Ignore        bool `schema:"ignore"`
		DockerTimeout int  `schema:"t"`
		LibpodTimeout uint `schema:"timeout"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	name := utils.GetName(r)
	options := entities.StopOptions{
		Ignore: query.Ignore,
	}
	if utils.IsLibpodRequest(r) {
		if _, found := r.URL.Query()["timeout"]; found {
			options.Timeout = &query.LibpodTimeout
		}
	} else {
		if _, found := r.URL.Query()["t"]; found {
			// -1 is allowed in Docker API, meaning wait infinite long, translate -1 to math.MaxInt value seconds to wait.
			timeout := util.ConvertTimeout(query.DockerTimeout)
			options.Timeout = &timeout
		}
	}
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	state, err := con.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		utils.WriteResponse(w, http.StatusNotModified, nil)
		return
	}
	report, err := containerEngine.ContainerStop(r.Context(), []string{name}, options)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, name, err)
			return
		}

		utils.InternalServerError(w, err)
		return
	}

	if len(report) > 0 && report[0].Err != nil {
		utils.InternalServerError(w, report[0].Err)
		return
	}

	// Success
	utils.WriteResponse(w, http.StatusNoContent, nil)
}
