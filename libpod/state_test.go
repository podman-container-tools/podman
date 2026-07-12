//go:build !remote && (linux || freebsd)

package libpod

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/lock"
	"go.podman.io/storage"
)

type emptyStateFunc func(t *testing.T) (State, lock.Manager)

var testedStates = map[string]emptyStateFunc{
	"sqlite": getEmptySqliteState,
}

func getEmptySqliteState(t *testing.T) (State, lock.Manager) {
	t.Helper()
	tmpDir := t.TempDir()
	lockManager, err := lock.NewInMemoryManager(16)
	require.NoError(t, err)

	runtime := new(Runtime)
	runtime.config = new(config.Config)
	runtime.storageConfig = storage.StoreOptions{}
	runtime.storageSet = storageSet{}
	runtime.lockManager = lockManager

	runtime.storageConfig.GraphRoot = tmpDir
	runtime.storageSet.StaticDirSet = true

	state, err := NewSqliteState(runtime)
	require.NoError(t, err)
	t.Cleanup(func() {
		state.Close()
	})

	return state, lockManager
}

func runForAllStates(t *testing.T, testFunc func(*testing.T, State, lock.Manager)) {
	for stateName, stateFunc := range testedStates {
		t.Run(stateName, func(t *testing.T) {
			state, manager := stateFunc(t)
			testFunc(t, state, manager)
		})
	}
}

func TestAddAndGetContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		retrievedCtr, err := state.Container(testCtr.ID())
		assert.NoError(t, err)

		testContainersEqual(t, retrievedCtr, testCtr, true)
	})
}

func TestAddAndGetContainerFromMultiple(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		retrievedCtr, err := state.Container(testCtr1.ID())
		assert.NoError(t, err)

		testContainersEqual(t, retrievedCtr, testCtr1, true)
	})
}

func TestGetContainerPodSameIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		_, err = state.Container(testPod.ID())
		assert.Error(t, err)
	})
}

func TestAddInvalidContainerFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.AddContainer(&Container{config: &ContainerConfig{ID: "1234"}})
		assert.Error(t, err)
	})
}

func TestAddDuplicateCtrIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestContainer(testCtr1.ID(), "test2", manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.Error(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))
	})
}

func TestAddDuplicateCtrNameFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestContainer(strings.Repeat("2", 32), testCtr1.Name(), manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.Error(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))
	})
}

func TestAddCtrPodDupIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)
		testCtr, err := getTestContainer(testPod.ID(), "testCtr", manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestAddCtrDepInPodFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.Error(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		require.Len(t, ctrs, 1)

		testContainersEqual(t, ctrs[0], testCtr1, true)
	})
}

func TestGetNonexistentContainerFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.Container("does not exist")
		assert.Error(t, err)
	})
}

func TestGetContainerWithEmptyIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.Container("")
		assert.Error(t, err)
	})
}

func TestLookupContainerWithEmptyIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.LookupContainer("")
		assert.Error(t, err)

		_, err = state.LookupContainerID("")
		assert.Error(t, err)
	})
}

func TestLookupNonexistentContainerFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.LookupContainer("does not exist")
		assert.Error(t, err)

		_, err = state.LookupContainerID("does not exist")
		assert.Error(t, err)
	})
}

func TestLookupContainerByFullID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		retrievedCtr, err := state.LookupContainer(testCtr.ID())
		assert.NoError(t, err)
		testContainersEqual(t, retrievedCtr, testCtr, true)

		retrievedID, err := state.LookupContainerID(testCtr.ID())
		assert.NoError(t, err)
		assert.Equal(t, retrievedID, testCtr.ID())
	})
}

func TestLookupContainerByUniquePartialID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		retrievedCtr, err := state.LookupContainer(testCtr.ID()[0:8])
		assert.NoError(t, err)
		testContainersEqual(t, retrievedCtr, testCtr, true)

		retrievedID, err := state.LookupContainerID(testCtr.ID()[0:8])
		assert.NoError(t, err)
		assert.Equal(t, retrievedID, testCtr.ID())
	})
}

func TestLookupContainerByNonUniquePartialIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestContainer(strings.Repeat("0", 32), "test1", manager)
		assert.NoError(t, err)
		testCtr2, err := getTestContainer(strings.Repeat("0", 31)+"1", "test2", manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		_, err = state.LookupContainer(testCtr1.ID()[0:8])
		assert.Error(t, err)

		_, err = state.LookupContainerID(testCtr1.ID()[0:8])
		assert.Error(t, err)
	})
}

func TestLookupContainerByName(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		retrievedCtr, err := state.LookupContainer(testCtr.Name())
		assert.NoError(t, err)
		testContainersEqual(t, retrievedCtr, testCtr, true)

		retrievedID, err := state.LookupContainerID(testCtr.Name())
		assert.NoError(t, err)
		assert.Equal(t, retrievedID, testCtr.ID())
	})
}

func TestLookupCtrByPodNameFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		_, err = state.LookupContainer(testPod.Name())
		assert.Error(t, err)

		_, err = state.LookupContainerID(testPod.Name())
		assert.Error(t, err)
	})
}

func TestLookupCtrByPodIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		_, err = state.LookupContainer(testPod.ID())
		assert.Error(t, err)

		_, err = state.LookupContainerID(testPod.ID())
		assert.Error(t, err)
	})
}

func TestHasContainerEmptyIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.HasContainer("")
		assert.Error(t, err)
	})
}

func TestHasContainerNoSuchContainerReturnsFalse(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		exists, err := state.HasContainer("does not exist")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestHasContainerFindsContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		exists, err := state.HasContainer(testCtr.ID())
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestHasContainerPodIDIsFalse(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		exists, err := state.HasContainer(testPod.ID())
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestSaveAndUpdateContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		retrievedCtr, err := state.Container(testCtr.ID())
		require.NoError(t, err)

		retrievedCtr.state.State = define.ContainerStateStopped
		retrievedCtr.state.ExitCode = 127
		retrievedCtr.state.FinishedTime = time.Now()

		err = state.SaveContainer(retrievedCtr)
		assert.NoError(t, err)

		err = state.UpdateContainer(testCtr)
		assert.NoError(t, err)

		testContainersEqual(t, testCtr, retrievedCtr, false)
	})
}

func TestUpdateContainerNotInDatabaseReturnsError(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.UpdateContainer(testCtr)
		assert.Error(t, err)
		assert.False(t, testCtr.valid)
	})
}

func TestUpdateInvalidContainerReturnsError(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.UpdateContainer(&Container{config: &ContainerConfig{ID: "1234"}})
		assert.Error(t, err)
	})
}

func TestSaveInvalidContainerReturnsError(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.SaveContainer(&Container{config: &ContainerConfig{ID: "1234"}})
		assert.Error(t, err)
	})
}

func TestSaveContainerNotInStateReturnsError(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.SaveContainer(testCtr)
		assert.Error(t, err)
		assert.False(t, testCtr.valid)
	})
}

func TestRemoveContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))

		err = state.RemoveContainer(testCtr)
		assert.NoError(t, err)

		ctrs2, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs2))
	})
}

func TestRemoveNonexistentContainerFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr)
		assert.Error(t, err)
	})
}

func TestGetAllContainersOnNewStateIsEmpty(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestGetAllContainersWithOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		require.Len(t, ctrs, 1)

		testContainersEqual(t, ctrs[0], testCtr, true)
	})
}

func TestGetAllContainersTwoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs))
	})
}

func TestContainerInUseInvalidContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.ContainerInUse(&Container{})
		assert.Error(t, err)
	})
}

func TestContainerInUseCtrNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)
		_, err = state.ContainerInUse(testCtr)
		assert.Error(t, err)
	})
}

func TestContainerInUseOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.config.ID

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ids, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ids))
		assert.Equal(t, testCtr2.config.ID, ids[0])
	})
}

func TestContainerInUseTwoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr3, err := getTestCtrN("3", manager)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.config.ID
		testCtr3.config.IPCNsCtr = testCtr1.config.ID

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr3)
		assert.NoError(t, err)

		ids, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ids))
	})
}

func TestContainerInUseOneContainerMultipleDependencies(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.config.ID
		testCtr2.config.IPCNsCtr = testCtr1.config.ID

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ids, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ids))
		assert.Equal(t, testCtr2.config.ID, ids[0])
	})
}

func TestContainerInUseGenericDependency(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2.config.Dependencies = []string{testCtr1.config.ID}

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ids, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ids))
		assert.Equal(t, testCtr2.config.ID, ids[0])
	})
}

func TestContainerInUseMultipleGenericDependencies(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr3, err := getTestCtrN("3", manager)
		assert.NoError(t, err)

		testCtr3.config.Dependencies = []string{testCtr1.config.ID, testCtr2.config.ID}

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr3)
		assert.NoError(t, err)

		ids1, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ids1))
		assert.Equal(t, testCtr3.config.ID, ids1[0])

		ids2, err := state.ContainerInUse(testCtr2)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ids2))
		assert.Equal(t, testCtr3.config.ID, ids2[0])
	})
}

func TestContainerInUseGenericAndNamespaceDependencies(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2.config.Dependencies = []string{testCtr1.config.ID}
		testCtr2.config.IPCNsCtr = testCtr1.config.ID

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ids, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ids))
		assert.Equal(t, testCtr2.config.ID, ids[0])
	})
}

func TestCannotRemoveContainerWithDependency(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.config.ID

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr1)
		assert.Error(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs))
	})
}

func TestCannotRemoveContainerWithGenericDependency(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2.config.Dependencies = []string{testCtr1.config.ID}

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr1)
		assert.Error(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs))
	})
}

func TestCanRemoveContainerAfterDependencyRemoved(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.ID()

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr1)
		assert.NoError(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestCanRemoveContainerAfterDependencyRemovedDuplicate(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr1, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr1)
		assert.NoError(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestCannotUsePodAsDependency(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		testPod, err := getTestPod2(manager)
		assert.NoError(t, err)

		testCtr.config.UserNsCtr = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestCannotUseBadIDAsDependency(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		testCtr.config.UserNsCtr = strings.Repeat("5", 32)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestCannotUseBadIDAsGenericDependency(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		testCtr.config.Dependencies = []string{strings.Repeat("5", 32)}

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestRewriteContainerConfigDoesNotExist(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.RewriteContainerConfig(&Container{}, &ContainerConfig{})
		assert.Error(t, err)
	})
}

func TestRewriteContainerConfigNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)
		err = state.RewriteContainerConfig(testCtr, &ContainerConfig{})
		assert.Error(t, err)
	})
}

func TestRewriteContainerConfigRewritesConfig(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		testCtr.config.LogPath = "/another/path/"

		err = state.RewriteContainerConfig(testCtr, testCtr.config)
		assert.NoError(t, err)

		testCtrFromState, err := state.Container(testCtr.ID())
		assert.NoError(t, err)

		testContainersEqual(t, testCtrFromState, testCtr, true)
	})
}

func TestRewritePodConfigDoesNotExist(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.RewritePodConfig(&Pod{}, &PodConfig{})
		assert.Error(t, err)
	})
}

func TestRewritePodConfigNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)
		err = state.RewritePodConfig(testPod, &PodConfig{})
		assert.Error(t, err)
	})
}

func TestRewritePodConfigRewritesConfig(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		testPod.config.CgroupParent = "/another_cgroup_parent"

		err = state.RewritePodConfig(testPod, testPod.config)
		assert.NoError(t, err)

		testPodFromState, err := state.Pod(testPod.ID())
		assert.NoError(t, err)

		testPodsEqual(t, testPodFromState, testPod, true)
	})
}

func TestGetPodDoesNotExist(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.Pod("doesnotexist")
		assert.Error(t, err)
	})
}

func TestGetPodEmptyID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.Pod("")
		assert.Error(t, err)
	})
}

func TestGetPodOnePod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		statePod, err := state.Pod(testPod.ID())
		assert.NoError(t, err)

		testPodsEqual(t, statePod, testPod, true)
	})
}

func TestGetOnePodFromTwo(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod1, err := getTestPod1(manager)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		statePod, err := state.Pod(testPod1.ID())
		assert.NoError(t, err)

		testPodsEqual(t, statePod, testPod1, true)
	})
}

func TestGetNotExistPodWithPods(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod1, err := getTestPod1(manager)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		_, err = state.Pod("nonexistent")
		assert.Error(t, err)
	})
}

func TestGetPodByCtrID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		_, err = state.Pod(testCtr.ID())
		assert.Error(t, err)
	})
}

func TestLookupPodEmptyID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.LookupPod("")
		assert.Error(t, err)
	})
}

func TestLookupNotExistPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.LookupPod("doesnotexist")
		assert.Error(t, err)
	})
}

func TestLookupPodFullID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		statePod, err := state.LookupPod(testPod.ID())
		assert.NoError(t, err)

		testPodsEqual(t, statePod, testPod, true)
	})
}

func TestLookupPodUniquePartialID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		statePod, err := state.LookupPod(testPod.ID()[0:8])
		assert.NoError(t, err)

		testPodsEqual(t, statePod, testPod, true)
	})
}

func TestLookupPodNonUniquePartialID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod1, err := getTestPod(strings.Repeat("1", 32), "test1", manager)
		assert.NoError(t, err)

		testPod2, err := getTestPod(strings.Repeat("1", 31)+"2", "test2", manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		_, err = state.LookupPod(testPod1.ID()[0:8])
		assert.Error(t, err)
	})
}

func TestLookupPodByName(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		statePod, err := state.LookupPod(testPod.Name())
		assert.NoError(t, err)

		testPodsEqual(t, statePod, statePod, true)
	})
}

func TestLookupPodByCtrID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		_, err = state.LookupPod(testCtr.ID())
		assert.Error(t, err)
	})
}

func TestLookupPodByCtrName(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		_, err = state.LookupPod(testCtr.Name())
		assert.Error(t, err)
	})
}

func TestHasPodEmptyIDErrors(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.HasPod("")
		assert.Error(t, err)
	})
}

func TestHasPodNoSuchPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		exist, err := state.HasPod("nonexistent")
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestHasPodWrongIDFalse(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		exist, err := state.HasPod(strings.Repeat("a", 32))
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestHasPodRightIDTrue(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		exist, err := state.HasPod(testPod.ID())
		assert.NoError(t, err)
		assert.True(t, exist)
	})
}

func TestHasPodCtrIDFalse(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		exist, err := state.HasPod(testCtr.ID())
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestAddPodInvalidPodErrors(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.AddPod(&Pod{config: &PodConfig{}})
		assert.Error(t, err)
	})
}

func TestAddPodValidPodSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))

		testPodsEqual(t, allPods[0], testPod, true)
	})
}

func TestAddPodDuplicateIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod1, err := getTestPod1(manager)
		assert.NoError(t, err)

		testPod2, err := getTestPod(testPod1.ID(), "testpod2", manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.Error(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))
	})
}

func TestAddPodDuplicateNameFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod1, err := getTestPod1(manager)
		assert.NoError(t, err)

		testPod2, err := getTestPod(strings.Repeat("2", 32), testPod1.Name(), manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.Error(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))
	})
}

func TestAddPodNonDuplicateSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod1, err := getTestPod1(manager)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allPods))
	})
}

func TestAddPodCtrIDConflictFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		testPod, err := getTestPod(testCtr.ID(), "testpod1", manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.Error(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods))
	})
}

func TestRemovePodInvalidPodErrors(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.RemovePod(&Pod{config: &PodConfig{}})
		assert.Error(t, err)
	})
}

func TestRemovePodNotInStateFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.RemovePod(testPod)
		assert.Error(t, err)
		assert.False(t, testPod.valid)
	})
}

func TestRemovePodSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.RemovePod(testPod)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods))
	})
}

func TestRemovePodFromPods(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod1, err := getTestPod1(manager)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		err = state.RemovePod(testPod1)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))

		testPodsEqual(t, allPods[0], testPod2, true)
	})
}

func TestRemovePodNotEmptyFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		err = state.RemovePod(testPod)
		assert.Error(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))
	})
}

func TestRemovePodAfterEmptySucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr)
		assert.NoError(t, err)

		err = state.RemovePod(testPod)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods))
	})
}

func TestAllPodsEmptyOnEmptyState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods))
	})
}

func TestAllPodsFindsPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))

		testPodsEqual(t, allPods[0], testPod, true)
	})
}

func TestAllPodsMultiplePods(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod1, err := getTestPod1(manager)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(manager)
		assert.NoError(t, err)

		testPod3, err := getTestPodN("3", manager)
		assert.NoError(t, err)

		allPods1, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods1))

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		allPods2, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods2))

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		allPods3, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allPods3))

		err = state.AddPod(testPod3)
		assert.NoError(t, err)

		allPods4, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 3, len(allPods4))
	})
}

func TestPodHasContainerNoSuchPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.PodHasContainer(&Pod{config: &PodConfig{}}, strings.Repeat("0", 32))
		assert.Error(t, err)
	})
}

func TestPodHasContainerEmptyCtrID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		_, err = state.PodHasContainer(testPod, "")
		assert.Error(t, err)
	})
}

func TestPodHasContainerNoSuchCtr(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		exist, err := state.PodHasContainer(testPod, strings.Repeat("2", 32))
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestPodHasContainerCtrNotInPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		exist, err := state.PodHasContainer(testPod, testCtr.ID())
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestPodHasContainerSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		exist, err := state.PodHasContainer(testPod, testCtr.ID())
		assert.NoError(t, err)
		assert.True(t, exist)
	})
}

func TestPodContainersByIDInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.PodContainersByID(&Pod{config: &PodConfig{}})
		assert.Error(t, err)
	})
}

func TestPodContainerdByIDPodNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		_, err = state.PodContainersByID(testPod)
		assert.Error(t, err)
	})
}

func TestPodContainersByIDEmptyPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestPodContainersByIDOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))
		assert.Equal(t, testCtr.ID(), ctrs[0])
	})
}

func TestPodContainersByIDMultipleContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		testCtr3, err := getTestCtrN("4", manager)
		assert.NoError(t, err)
		testCtr3.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		ctrs0, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs0))

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		ctrs1, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs1))

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ctrs2, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs2))

		err = state.AddContainer(testCtr3)
		assert.NoError(t, err)

		ctrs3, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(ctrs3))
	})
}

func TestPodContainersInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.PodContainers(&Pod{config: &PodConfig{}})
		assert.Error(t, err)
	})
}

func TestPodContainersPodNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		_, err = state.PodContainers(testPod)
		assert.Error(t, err)
	})
}

func TestPodContainersEmptyPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestPodContainersOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		require.Len(t, ctrs, 1)

		testContainersEqual(t, ctrs[0], testCtr, true)
	})
}

func TestPodContainersMultipleContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		testCtr3, err := getTestCtrN("4", manager)
		assert.NoError(t, err)
		testCtr3.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		ctrs0, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs0))

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		ctrs1, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs1))

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ctrs2, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs2))

		err = state.AddContainer(testCtr3)
		assert.NoError(t, err)

		ctrs3, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(ctrs3))
	})
}

func TestRemovePodContainersInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.RemovePodContainers(&Pod{config: &PodConfig{}})
		assert.Error(t, err)
	})
}

func TestRemovePodContainersPodNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.Error(t, err)
		assert.False(t, testPod.valid)
	})
}

func TestRemovePodContainersNoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestRemovePodContainersOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestRemovePodContainersPreservesCtrOutsidePod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allCtrs))
	})
}

func TestRemovePodContainersTwoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestRemovePodContainerDependencyInPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestAddContainerToPodInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = "1"

		err = state.AddContainer(testCtr)
		assert.Error(t, err)
	})
}

func TestAddContainerToPodInvalidCtr(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(&Container{config: &ContainerConfig{ID: "1234"}})
		assert.Error(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestAddContainerToPodPodNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddContainer(testCtr)
		assert.Error(t, err)
	})
}

func TestAddContainerToPodSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		require.Len(t, ctrs, 1)

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		require.Len(t, allCtrs, 1)

		testContainersEqual(t, ctrs[0], testCtr, true)
		testContainersEqual(t, ctrs[0], allCtrs[0], false)
	})
}

func TestAddContainerToPodTwoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs))

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allCtrs))
	})
}

func TestAddContainerToPodWithAddContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		require.Len(t, ctrs, 1)

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allCtrs))

		testContainersEqual(t, ctrs[0], testCtr1, true)
	})
}

func TestAddContainerToPodCtrIDConflict(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2, err := getTestContainer(testCtr1.ID(), "testCtr3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allCtrs))
	})
}

func TestAddContainerToPodCtrNameConflict(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2, err := getTestContainer(strings.Repeat("4", 32), testCtr1.Name(), manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allCtrs))
	})
}

func TestAddContainerToPodPodIDConflict(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestContainer(testPod.ID(), "testCtr", manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allCtrs))
	})
}

func TestAddContainerToPodAddsDependencies(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		deps, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(deps))
		assert.Equal(t, testCtr2.ID(), deps[0])
	})
}

func TestAddContainerToPodPodDependencyFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()
		testCtr.config.IPCNsCtr = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestAddContainerToPodBadDependencyFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()
		testCtr.config.IPCNsCtr = strings.Repeat("8", 32)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestAddContainerToPodDependencyOutsidePodFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allCtrs))

		deps, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(deps))
	})
}

func TestRemoveContainerFromPodBadPodFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = "1"

		err = state.RemoveContainer(testCtr)
		assert.Error(t, err)
	})
}

func TestRemoveContainerFromPodPodNotInStateFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.RemoveContainer(testCtr)
		assert.Error(t, err)
	})
}

func TestRemoveContainerFromPodCtrNotInStateFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr)
		assert.Error(t, err)

		assert.False(t, testCtr.valid)
	})
}

func TestRemoveContainerFromPodSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allCtrs))
	})
}

func TestRemoveContainerFromPodWithDependencyFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr1)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs))

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allCtrs))
	})
}

func TestRemoveContainerFromPodWithDependencySucceedsAfterDepRemoved(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(manager)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", manager)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr1)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers(false)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allCtrs))
	})
}

func TestUpdatePodInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.UpdatePod(&Pod{config: &PodConfig{}})
		assert.Error(t, err)
	})
}

func TestUpdatePodPodNotInStateFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.UpdatePod(testPod)
		assert.Error(t, err)
	})
}

func TestSavePodInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		err := state.SavePod(&Pod{config: &PodConfig{}})
		assert.Error(t, err)
	})
}

func TestSavePodPodNotInStateFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.SavePod(testPod)
		assert.Error(t, err)
	})
}

func TestSaveAndUpdatePod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testPod, err := getTestPod1(manager)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		statePod, err := state.Pod(testPod.ID())
		assert.NoError(t, err)

		testPodsEqual(t, statePod, testPod, true)

		testPod.state.CgroupPath = "/new/path/for/test"

		err = state.SavePod(testPod)
		assert.NoError(t, err)

		err = state.UpdatePod(statePod)
		assert.NoError(t, err)

		testPodsEqual(t, statePod, testPod, false)
	})
}

func TestGetContainerConfigSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, manager lock.Manager) {
		testCtr, err := getTestCtr1(manager)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		ctrCfg, err := state.GetContainerConfig(testCtr.ID())
		assert.NoError(t, err)
		assert.Equal(t, ctrCfg, testCtr.Config())
	})
}

func TestGetContainerConfigEmptyIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.GetContainerConfig("")
		assert.Error(t, err)
	})
}

func TestGetContainerConfigNonExistentIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		_, err := state.GetContainerConfig("does not exist")
		assert.Error(t, err)
	})
}

func TestRemoveVolumeNotInDB(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, _ lock.Manager) {
		v := &Volume{
			config: &VolumeConfig{
				Name: "Test",
			},
			valid: true,
		}
		err := state.RemoveVolume(v)
		require.Error(t, err)
		require.ErrorIs(t, err, define.ErrNoSuchVolume)
	})
}
