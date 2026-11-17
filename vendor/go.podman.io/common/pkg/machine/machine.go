package machine

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"
)

type Marker struct {
	Enabled bool
	Type    string
}

const (
	// New marker file as of podman 6.0 since /etc/containers get overmounted.
	markerFile = "/etc/podman-machine"
	// Marker file prior to podman 6.0.
	markerFileOld = "/etc/containers/podman-machine"
	Wsl           = "wsl"
	Qemu          = "qemu"
	AppleHV       = "applehv"
	HyperV        = "hyperv"
)

var readMarkerOnce = sync.OnceValue(func() *Marker {
	return loadMachineMarker(markerFile, markerFileOld)
})

func loadMachineMarker(file, fallbackFile string) *Marker {
	if content, err := os.ReadFile(file); err == nil {
		return &Marker{Enabled: true, Type: strings.TrimSpace(string(content))}
	} else if errors.Is(err, fs.ErrNotExist) {
		if content, err := os.ReadFile(fallbackFile); err == nil {
			return &Marker{Enabled: true, Type: strings.TrimSpace(string(content))}
		}
	}
	return &Marker{}
}

func (m *Marker) IsPodmanMachine() bool {
	return m.Enabled
}

func IsPodmanMachine() bool {
	return GetMachineMarker().IsPodmanMachine()
}

func (m *Marker) HostType() string {
	return m.Type
}

func HostType() string {
	return GetMachineMarker().HostType()
}

func (m *Marker) IsGvProxyBased() bool {
	return m.IsPodmanMachine() && m.HostType() != Wsl
}

func IsGvProxyBased() bool {
	return GetMachineMarker().IsGvProxyBased()
}

func GetMachineMarker() *Marker {
	return readMarkerOnce()
}
