package entities

import (
	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

// ServiceOptions provides the input for starting an API and sidecar pprof services
type (
	ServiceOptions          = types.ServiceOptions
	SystemPruneOptions      = types.SystemPruneOptions
	SystemPruneReport       = types.SystemPruneReport
	SystemMigrateOptions    = types.SystemMigrateOptions
	SystemCheckOptions      = types.SystemCheckOptions
	SystemCheckReport       = types.SystemCheckReport
	SystemDfOptions         = types.SystemDfOptions
	SystemDfReport          = types.SystemDfReport
	SystemDfImageReport     = types.SystemDfImageReport
	SystemDfContainerReport = types.SystemDfContainerReport
	SystemDfVolumeReport    = types.SystemDfVolumeReport
	SystemVersionReport     = types.SystemVersionReport
	SystemUnshareOptions    = types.SystemUnshareOptions
	ComponentVersion        = types.SystemComponentVersion
	ListRegistriesReport    = types.ListRegistriesReport
)

type (
	AuthConfig  = types.AuthConfig
	AuthReport  = types.AuthReport
	LocksReport = types.LocksReport
)
