package entities

import (
	"errors"

	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

var ErrNoSuchConfig = errors.New("no such config")

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
