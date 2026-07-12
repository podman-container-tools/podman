//go:build !remote && (linux || freebsd)

package libpod

import (
	"fmt"
	"net/http"

	"github.com/gorilla/schema"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
)

func GenerateSystemd(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	cfg, err := config.Default()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("reading containers.conf: %w", err))
		return
	}

	query := struct {
		Name                   bool     `schema:"useName"`
		New                    bool     `schema:"new"`
		NoHeader               bool     `schema:"noHeader"`
		TemplateUnitFile       bool     `schema:"templateUnitFile"`
		RestartPolicy          *string  `schema:"restartPolicy"`
		RestartSec             uint     `schema:"restartSec"`
		StopTimeout            uint     `schema:"stopTimeout"`
		StartTimeout           uint     `schema:"startTimeout"`
		ContainerPrefix        *string  `schema:"containerPrefix"`
		PodPrefix              *string  `schema:"podPrefix"`
		Separator              *string  `schema:"separator"`
		Wants                  []string `schema:"wants"`
		After                  []string `schema:"after"`
		Requires               []string `schema:"requires"`
		AdditionalEnvVariables []string `schema:"additionalEnvVariables"`
	}{
		StartTimeout: 0,
		StopTimeout:  cfg.Engine.StopTimeout,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	ContainerPrefix := "container"
	if query.ContainerPrefix != nil {
		ContainerPrefix = *query.ContainerPrefix
	}

	PodPrefix := "pod"
	if query.PodPrefix != nil {
		PodPrefix = *query.PodPrefix
	}

	Separator := "-"
	if query.Separator != nil {
		Separator = *query.Separator
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.GenerateSystemdOptions{
		Name:                   query.Name,
		New:                    query.New,
		NoHeader:               query.NoHeader,
		TemplateUnitFile:       query.TemplateUnitFile,
		RestartPolicy:          query.RestartPolicy,
		StartTimeout:           &query.StartTimeout,
		StopTimeout:            &query.StopTimeout,
		ContainerPrefix:        ContainerPrefix,
		PodPrefix:              PodPrefix,
		Separator:              Separator,
		RestartSec:             &query.RestartSec,
		Wants:                  query.Wants,
		After:                  query.After,
		Requires:               query.Requires,
		AdditionalEnvVariables: query.AdditionalEnvVariables,
	}

	report, err := containerEngine.GenerateSystemd(r.Context(), utils.GetName(r), options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("generating systemd units: %w", err))
		return
	}

	utils.WriteResponse(w, http.StatusOK, report.Units)
}

func GenerateKube(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		PodmanOnly bool     `schema:"podmanOnly"`
		Names      []string `schema:"names"`
		Service    bool     `schema:"service"`
		Type       string   `schema:"type"`
		Replicas   int32    `schema:"replicas"`
		NoTrunc    bool     `schema:"noTrunc"`
	}{
		// Defaults would go here.
		Replicas: 1,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// Read the default kubeGenerateType from containers.conf it the user doesn't specify it
	generateType := query.Type
	if generateType == "" {
		config, err := runtime.GetConfigNoCopy()
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
		generateType = config.Engine.KubeGenerateType
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.GenerateKubeOptions{
		PodmanOnly:         query.PodmanOnly,
		Service:            query.Service,
		Type:               generateType,
		Replicas:           query.Replicas,
		UseLongAnnotations: query.NoTrunc,
	}
	report, err := containerEngine.GenerateKube(r.Context(), query.Names, options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("generating YAML: %w", err))
		return
	}

	// FIXME: Content-Type is being set as application/x-tar NOT text/vnd.yaml
	// https://mailarchive.ietf.org/arch/msg/media-types/e9ZNC0hDXKXeFlAVRWxLCCaG9GI/
	utils.WriteResponse(w, http.StatusOK, report.Reader)
}
