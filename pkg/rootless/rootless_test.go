package rootless

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/moby/sys/user"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestGetNamespaceHandlesPath(t *testing.T) {
	stateDir := "libpod"
	result := GetNamespaceHandlesPath(stateDir)
	assert.Equal(t, filepath.Join(stateDir, "ns_handles"), result)
}

func TestGetPausePidPath(t *testing.T) {
	stateDir := "libpod"
	result := GetPausePidPath(stateDir)
	assert.Equal(t, filepath.Join(stateDir, "pause.pid"), result)
}

func TestMaybeSplitMappings(t *testing.T) {
	mappings := []spec.LinuxIDMapping{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        2,
		},
	}
	desiredMappings := []spec.LinuxIDMapping{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        1,
		},
		{
			ContainerID: 1,
			HostID:      1,
			Size:        1,
		},
	}
	availableMappings := []user.IDMap{
		{
			ID:       1,
			ParentID: 1000000,
			Count:    65536,
		},
		{
			ID:       0,
			ParentID: 1000,
			Count:    1,
		},
	}
	newMappings := MaybeSplitMappings(mappings, availableMappings)
	if !reflect.DeepEqual(newMappings, desiredMappings) {
		t.Fatal("wrong mappings generated")
	}

	mappings = []spec.LinuxIDMapping{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        2,
		},
	}
	desiredMappings = []spec.LinuxIDMapping{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        2,
		},
	}
	availableMappings = []user.IDMap{
		{
			ID:       0,
			ParentID: 1000000,
			Count:    65536,
		},
	}
	newMappings = MaybeSplitMappings(mappings, availableMappings)

	if !reflect.DeepEqual(newMappings, desiredMappings) {
		t.Fatal("wrong mappings generated")
	}

	mappings = []spec.LinuxIDMapping{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        1,
		},
	}
	desiredMappings = []spec.LinuxIDMapping{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        1,
		},
	}
	availableMappings = []user.IDMap{
		{
			ID:       10000,
			ParentID: 10000,
			Count:    65536,
		},
	}

	newMappings = MaybeSplitMappings(mappings, availableMappings)
	if !reflect.DeepEqual(newMappings, desiredMappings) {
		t.Fatal("wrong mappings generated")
	}

	mappings = []spec.LinuxIDMapping{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        4,
		},
	}
	desiredMappings = []spec.LinuxIDMapping{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        1,
		},
		{
			ContainerID: 1,
			HostID:      1,
			Size:        1,
		},
		{
			ContainerID: 2,
			HostID:      2,
			Size:        1,
		},
		{
			ContainerID: 3,
			HostID:      3,
			Size:        1,
		},
	}
	availableMappings = []user.IDMap{
		{
			ID:       0,
			ParentID: 0,
			Count:    1,
		},
		{
			ID:       1,
			ParentID: 1,
			Count:    1,
		},
		{
			ID:       2,
			ParentID: 2,
			Count:    1,
		},
		{
			ID:       3,
			ParentID: 3,
			Count:    1,
		},
	}

	newMappings = MaybeSplitMappings(mappings, availableMappings)
	if !reflect.DeepEqual(newMappings, desiredMappings) {
		t.Fatal("wrong mappings generated")
	}
}
