//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/lock"
)

func TestBatchDoesNotModifyOriginalState(t *testing.T) {
	manager, err := lock.NewInMemoryManager(1)
	require.NoError(t, err)
	ctrLock, err := manager.AllocateLock()
	require.NoError(t, err)

	ctr := &Container{
		config: &ContainerConfig{ID: "test"},
		state: &ContainerState{
			State:      define.ContainerStateRunning,
			PID:        1234,
			BindMounts: map[string]string{"/etc/hosts": "/run/hosts"},
			Service:    Service{Pods: []string{"pod1"}},
		},
		lock:  ctrLock,
		valid: true,
	}

	err = ctr.Batch(func(batchCtr *Container) error {
		batchCtr.state.State = define.ContainerStateStopped
		batchCtr.state.PID = 0
		batchCtr.state.BindMounts["/etc/resolv.conf"] = "/run/resolv.conf"
		delete(batchCtr.state.BindMounts, "/etc/hosts")
		batchCtr.state.Service.Pods = append(batchCtr.state.Service.Pods, "pod2")
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, define.ContainerStateRunning, ctr.state.State)
	assert.Equal(t, 1234, ctr.state.PID)
	assert.Equal(t, map[string]string{"/etc/hosts": "/run/hosts"}, ctr.state.BindMounts)
	assert.Equal(t, []string{"pod1"}, ctr.state.Service.Pods)
}

func TestContainerStateCopyKeepsNilMaps(t *testing.T) {
	state := &ContainerState{State: define.ContainerStateConfigured}

	newState := state.copy()

	assert.Nil(t, newState.ExecSessions)
	assert.Nil(t, newState.NetworkStatus)
	assert.Nil(t, newState.BindMounts)
	assert.Nil(t, newState.ExtensionStageHooks)
	assert.Nil(t, newState.NetInterfaceDescriptions)
	assert.Nil(t, newState.Service.Pods)
	assert.Equal(t, state, newState)
}
