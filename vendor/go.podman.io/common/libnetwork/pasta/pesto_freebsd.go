package pasta

import (
	"errors"

	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
)

var errPestoNotSupported = errors.New("pesto is not supported on FreeBSD")

func PestoAddPorts(_ *config.Config, _ string, _ []types.PortMapping, _, _ string) error {
	return errPestoNotSupported
}

func PestoDeletePorts(_ *config.Config, _ string, _ []types.PortMapping, _, _ string) error {
	return errPestoNotSupported
}
