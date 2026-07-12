//go:build !remote && (linux || freebsd)

package libpod

import (
	"fmt"
	"net/http"

	"github.com/gorilla/schema"
	"go.podman.io/common/pkg/secrets"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
)

func CreateSecret(w http.ResponseWriter, r *http.Request) {
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
	}{
		// override any golang type defaults
	}
	opts := entities.SecretCreateOptions{}
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
	report, err := ic.SecretCreate(r.Context(), query.Name, r.Body, opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func SecretExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	ic := abi.ContainerEngine{Libpod: runtime}

	report, err := ic.SecretExists(r.Context(), name)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if !report.Value {
		utils.SecretNotFound(w, name, secrets.ErrNoSuchSecret)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}
