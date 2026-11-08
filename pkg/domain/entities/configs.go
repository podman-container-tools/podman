package entities

import (
	"errors"

	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

// MaxConfigSize is the exclusive maximum size of a config payload in bytes.
const MaxConfigSize = 512000

var (
	ErrNoSuchConfig       = define.ErrNoSuchConfig
	ErrConfigDataTooLarge = errors.New("config data must be smaller than 512000 bytes")
)

type ConfigCreateReport = types.ConfigCreateReport

type ConfigCreateOptions struct {
	Driver     string
	DriverOpts map[string]string
	Labels     map[string]string
	Replace    bool
	Ignore     bool
}

type ConfigInspectOptions struct {
	ShowConfig bool
}

type ConfigListRequest struct {
	Filters map[string][]string
}

type ConfigRmOptions struct {
	All    bool
	Ignore bool
}

type ConfigRmReport = types.ConfigRmReport

type ConfigInfoReport = types.ConfigInfoReport

type ConfigListReport = types.ConfigListReport

type ConfigInfoReportCompat = types.ConfigInfoReportCompat

type ConfigVersion = types.ConfigVersion

type ConfigSpec = types.ConfigSpec

type ConfigDriverSpec = types.ConfigDriverSpec
