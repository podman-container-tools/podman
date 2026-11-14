//go:build !remote && (linux || freebsd)

package libpod

import (
	stdjson "encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamedVolumeCreateOptionsAreNotPersisted(t *testing.T) {
	t.Parallel()

	ctr := &Container{config: &ContainerConfig{}}
	volumes := []*ContainerNamedVolume{{
		Name: "data",
		Dest: "/data",
	}}
	driverOptions := map[string]string{"fstype": "nfs"}
	createOptions := map[string]NamedVolumeCreateOptions{
		"/data": {
			Driver:        "test-driver",
			DriverOptions: driverOptions,
		},
	}

	require.NoError(t, WithNamedVolumes(volumes)(ctr))
	require.NoError(t, WithNamedVolumeCreateOptions(createOptions)(ctr))

	require.Len(t, ctr.config.NamedVolumes, 1)
	assert.Equal(t, "data", ctr.config.NamedVolumes[0].Name)
	assert.Equal(t, "/data", ctr.config.NamedVolumes[0].Dest)
	assert.Equal(t, "test-driver", ctr.namedVolumeCreateOptions["/data"].Driver)
	assert.Equal(t, map[string]string{"fstype": "nfs"}, ctr.namedVolumeCreateOptions["/data"].DriverOptions)

	driverOptions["fstype"] = "changed"
	assert.Equal(t, "nfs", ctr.namedVolumeCreateOptions["/data"].DriverOptions["fstype"])

	configJSON, err := stdjson.Marshal(ctr.config)
	require.NoError(t, err)
	assert.NotContains(t, string(configJSON), "test-driver")
	assert.NotContains(t, string(configJSON), "driverOptions")
}
