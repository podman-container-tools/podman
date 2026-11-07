//go:build !remote && (linux || freebsd)

package libpod

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/schema"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
)

func CreateConfig(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder = r.Context().Value(api.DecoderKey).(*schema.Decoder)
	)

	query := struct {
		Name       string            `schema:"name"`
		Driver     string            `schema:"driver"`
		DriverOpts map[string]string `schema:"driveropts"`
		Labels     map[string]string `schema:"labels"`
		Replace    bool              `schema:"replace"`
		Ignore     bool              `schema:"ignore"`
	}{}
	opts := entities.ConfigCreateOptions{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	opts.Driver = query.Driver
	opts.DriverOpts = query.DriverOpts
	opts.Labels = query.Labels
	opts.Replace = query.Replace
	opts.Ignore = query.Ignore

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.ConfigCreate(r.Context(), query.Name, r.Body, opts)
	if err != nil {
		if strings.Contains(err.Error(), "cannot use --ignore and --replace") {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		if strings.Contains(err.Error(), "config name in use") {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func ConfigExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	ic := abi.ContainerEngine{Libpod: runtime}

	report, err := ic.ConfigExists(r.Context(), name)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if !report.Value {
		utils.ConfigNotFound(w, name, entities.ErrNoSuchConfig)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}
