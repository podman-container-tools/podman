package images

import (
	"net/url"

	"go.podman.io/podman/v6/pkg/bindings/internal/util"
)

// ToParams formats struct fields to be passed to API service
func (o *ScpOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}
