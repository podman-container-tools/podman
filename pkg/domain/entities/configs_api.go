package entities

import (
	"go.podman.io/podman/v6/pkg/errorhandling"
)

// swagger:model ConfigCreate
type ConfigCreateRequest struct {
	// User-defined name of the config.
	Name string
	// Base64-url-safe-encoded (RFC 4648) data to store as config.
	Data string
	// Driver represents a driver (default "file")
	Driver ConfigDriverSpec
	// Labels are labels on the config
	Labels map[string]string
}

// Config create response
// swagger:response ConfigCreateResponse
type SwagConfigCreateResponse struct {
	// in:body
	Body struct {
		ConfigCreateReport
	}
}

// Config list response
// swagger:response ConfigListResponse
type SwagConfigListResponse struct {
	// in:body
	Body []*ConfigInfoReport
}

// Config list response
// swagger:response ConfigListCompatResponse
type SwagConfigListCompatResponse struct {
	// in:body
	Body []*ConfigInfoReportCompat
}

// Config inspect response
// swagger:response ConfigInspectResponse
type SwagConfigInspectResponse struct {
	// in:body
	Body ConfigInfoReport
}

// Config inspect compat
// swagger:response ConfigInspectCompatResponse
type SwagConfigInspectCompatResponse struct {
	// in:body
	Body ConfigInfoReportCompat
}

// No such config
// swagger:response NoSuchConfig
type SwagErrNoSuchConfig struct {
	// in:body
	Body struct {
		errorhandling.ErrorModel
	}
}

// Config in use
// swagger:response ConfigInUse
type SwagErrConfigInUse struct {
	// in:body
	Body struct {
		errorhandling.ErrorModel
	}
}
