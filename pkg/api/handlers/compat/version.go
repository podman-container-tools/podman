//go:build !remote && (linux || freebsd)

package compat

import (
	"fmt"
	"net/http"
	goRuntime "runtime"
	"time"

	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/types"
	"go.podman.io/podman/v6/version"
)

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	running, err := define.GetVersion()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	info, err := runtime.Info()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to obtain system memory info: %w", err))
		return
	}

	components := []types.ComponentVersion{{
		Name:    "Podman Engine",
		Version: running.Version,
		Details: map[string]string{
			"APIVersion":    version.APIVersion[version.Libpod][version.CurrentAPI].String(),
			"Arch":          goRuntime.GOARCH,
			"BuildTime":     time.Unix(running.Built, 0).Format(time.RFC3339),
			"Experimental":  "false",
			"GitCommit":     running.GitCommit,
			"GoVersion":     running.GoVersion,
			"KernelVersion": info.Host.Kernel,
			"MinAPIVersion": version.APIVersion[version.Libpod][version.MinimalAPI].String(),
			"Os":            goRuntime.GOOS,
		},
	}, {
		Name:    "Conmon",
		Version: info.Host.Conmon.Version,
		Details: map[string]string{
			"Package": info.Host.Conmon.Package,
		},
	}, {
		Name:    fmt.Sprintf("OCI Runtime (%s)", info.Host.OCIRuntime.Name),
		Version: info.Host.OCIRuntime.Version,
		Details: map[string]string{
			"Package": info.Host.OCIRuntime.Package,
		},
	}}

	apiVersion := version.APIVersion[version.Compat][version.CurrentAPI]
	minVersion := version.APIVersion[version.Compat][version.MinimalAPI]

	utils.WriteResponse(w, http.StatusOK, entities.ComponentVersion{
		Version: types.Version{
			Platform: struct {
				Name string
			}{
				Name: fmt.Sprintf("%s/%s/%s-%s", goRuntime.GOOS, goRuntime.GOARCH, info.Host.Distribution.Distribution, info.Host.Distribution.Version),
			},
			APIVersion:    fmt.Sprintf("%d.%d", apiVersion.Major, apiVersion.Minor),
			Arch:          components[0].Details["Arch"],
			BuildTime:     components[0].Details["BuildTime"],
			Components:    components,
			Experimental:  false,
			GitCommit:     components[0].Details["GitCommit"],
			GoVersion:     components[0].Details["GoVersion"],
			KernelVersion: components[0].Details["KernelVersion"],
			MinAPIVersion: fmt.Sprintf("%d.%d", minVersion.Major, minVersion.Minor),
			Os:            components[0].Details["Os"],
			Version:       components[0].Version,
		},
	})
}
