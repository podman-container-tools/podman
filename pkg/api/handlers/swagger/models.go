//go:build !remote

//nolint:unused // these types are used to wire generated swagger to API code
package swagger

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// Details for creating a volume
// swagger:model
type volumeCreate struct {
	// Name of the volume driver to use.
	// Required: true
	Driver string `json:"Driver"`

	// A mapping of driver options and values. These options are
	// passed directly to the driver and are driver specific.
	//
	// Required: true
	DriverOpts map[string]string `json:"DriverOpts"`

	// User-defined key/value metadata.
	// Required: true
	Labels map[string]string `json:"Labels"`

	// The new volume's name. If not specified, Docker generates a name.
	//
	// Required: true
	Name string `json:"Name"`
}

// Network create
// swagger:model
type networkCreate network.CreateRequest

// Network connect
// swagger:model
type networkConnectRequest network.ConnectRequest

// Network disconnect
// swagger:model
type networkDisconnectRequest network.DisconnectRequest

// Network connect
// swagger:model
type networkConnectRequestLibpod entities.NetworkConnectOptions

// Network update
// swagger:model
type networkUpdateRequestLibpod entities.NetworkUpdateOptions

// Container update
// swagger:model
type containerUpdateRequest struct {
	container.UpdateConfig
}
