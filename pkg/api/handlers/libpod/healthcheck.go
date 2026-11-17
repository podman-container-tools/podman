//go:build !remote && (linux || freebsd)

package libpod

import (
	"net/http"

	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
)

func RunHealthCheck(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	status, err := runtime.HealthCheck(r.Context(), name)
	if err != nil {
		if status == define.HealthCheckContainerNotFound {
			utils.ContainerNotFound(w, name, err)
			return
		}
		if status == define.HealthCheckNotDefined {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		if status == define.HealthCheckContainerStopped {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	report := define.HealthCheckResults{
		Status: status.String(),
	}
	utils.WriteResponse(w, http.StatusOK, report)
}
