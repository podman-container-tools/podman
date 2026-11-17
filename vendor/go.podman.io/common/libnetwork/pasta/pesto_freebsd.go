package pasta

import (
	"errors"

	"go.podman.io/common/libnetwork/types"
)

var errPestoNotSupported = errors.New("pesto is not supported on FreeBSD")

func (p *PestoClient) AddPorts(_ []types.PortMapping, _, _ string) error {
	return errPestoNotSupported
}

func (p *PestoClient) DeletePorts(_ []types.PortMapping, _, _ string) error {
	return errPestoNotSupported
}
