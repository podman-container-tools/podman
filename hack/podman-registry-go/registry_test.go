package registry

import (
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartAndStopMultipleRegistries(t *testing.T) {
	binary = "../podman-registry"

	registries := []*Registry{}

	registryOptions := &Options{
		PodmanPath: "../../bin/podman",
	}

	// Start registries.
	var errors *multierror.Error
	for range 3 {
		reg, err := StartWithOptions(registryOptions)
		if err != nil {
			errors = multierror.Append(errors, err)
			continue
		}
		assert.True(t, len(reg.Image) > 0)
		assert.True(t, len(reg.User) > 0)
		assert.True(t, len(reg.Password) > 0)
		assert.True(t, len(reg.Port) > 0)
		registries = append(registries, reg)
	}

	// Stop registries.
	for _, reg := range registries {
		// Make sure we can stop it properly.
		errors = multierror.Append(errors, reg.Stop())
		// Stopping an already stopped registry is fine as well.
		errors = multierror.Append(errors, reg.Stop())
	}

	require.NoError(t, errors.ErrorOrNil())
}
