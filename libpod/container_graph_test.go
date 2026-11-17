//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/libpod/lock"
)

func TestBuildContainerGraphNoCtrsIsEmpty(t *testing.T) {
	graph, err := BuildContainerGraph([]*Container{})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(graph.nodes))
	assert.Equal(t, 0, len(graph.noDepNodes))
	assert.Equal(t, 0, len(graph.notDependedOnNodes))
}

func TestBuildContainerGraphOneCtr(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)

	graph, err := BuildContainerGraph([]*Container{ctr1})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(graph.nodes))
	assert.Equal(t, 1, len(graph.noDepNodes))
	assert.Equal(t, 1, len(graph.notDependedOnNodes))

	node, ok := graph.nodes[ctr1.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr1.ID(), node.id)

	assert.Equal(t, ctr1.ID(), graph.noDepNodes[0].id)
	assert.Equal(t, ctr1.ID(), graph.notDependedOnNodes[ctr1.ID()].id)
}

func TestBuildContainerGraphTwoCtrNoEdge(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)

	graph, err := BuildContainerGraph([]*Container{ctr1, ctr2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(graph.nodes))
	assert.Equal(t, 2, len(graph.noDepNodes))
	assert.Equal(t, 2, len(graph.notDependedOnNodes))

	node1, ok := graph.nodes[ctr1.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr1.ID(), node1.id)

	node2, ok := graph.nodes[ctr2.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr2.ID(), node2.id)
}

func TestBuildContainerGraphTwoCtrOneEdge(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr2.config.UserNsCtr = ctr1.config.ID

	graph, err := BuildContainerGraph([]*Container{ctr1, ctr2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(graph.nodes))
	assert.Equal(t, 1, len(graph.noDepNodes))
	assert.Equal(t, 1, len(graph.notDependedOnNodes))

	assert.Equal(t, ctr1.ID(), graph.noDepNodes[0].id)
	assert.Equal(t, ctr2.ID(), graph.notDependedOnNodes[ctr2.ID()].id)
}

func TestBuildContainerGraphTwoCtrCycle(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr2.config.UserNsCtr = ctr1.config.ID
	ctr1.config.NetNsCtr = ctr2.config.ID

	_, err = BuildContainerGraph([]*Container{ctr1, ctr2})
	assert.Error(t, err)
}

func TestBuildContainerGraphThreeCtrNoEdges(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr3, err := getTestCtrN("3", manager)
	assert.NoError(t, err)

	graph, err := BuildContainerGraph([]*Container{ctr1, ctr2, ctr3})
	assert.NoError(t, err)
	assert.Equal(t, 3, len(graph.nodes))
	assert.Equal(t, 3, len(graph.noDepNodes))
	assert.Equal(t, 3, len(graph.notDependedOnNodes))

	node1, ok := graph.nodes[ctr1.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr1.ID(), node1.id)

	node2, ok := graph.nodes[ctr2.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr2.ID(), node2.id)

	node3, ok := graph.nodes[ctr3.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr3.ID(), node3.id)
}

func TestBuildContainerGraphThreeContainersTwoInCycle(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr3, err := getTestCtrN("3", manager)
	assert.NoError(t, err)
	ctr1.config.UserNsCtr = ctr2.config.ID
	ctr2.config.IPCNsCtr = ctr1.config.ID

	_, err = BuildContainerGraph([]*Container{ctr1, ctr2, ctr3})
	assert.Error(t, err)
}

func TestBuildContainerGraphThreeContainersCycle(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr3, err := getTestCtrN("3", manager)
	assert.NoError(t, err)
	ctr1.config.UserNsCtr = ctr2.config.ID
	ctr2.config.IPCNsCtr = ctr3.config.ID
	ctr3.config.NetNsCtr = ctr1.config.ID

	_, err = BuildContainerGraph([]*Container{ctr1, ctr2, ctr3})
	assert.Error(t, err)
}

func TestBuildContainerGraphThreeContainersNoCycle(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr3, err := getTestCtrN("3", manager)
	assert.NoError(t, err)
	ctr1.config.UserNsCtr = ctr2.config.ID
	ctr1.config.NetNsCtr = ctr3.config.ID
	ctr2.config.IPCNsCtr = ctr3.config.ID

	graph, err := BuildContainerGraph([]*Container{ctr1, ctr2, ctr3})
	assert.NoError(t, err)
	assert.Equal(t, 3, len(graph.nodes))
	assert.Equal(t, 1, len(graph.noDepNodes))
	assert.Equal(t, 1, len(graph.notDependedOnNodes))

	assert.Equal(t, ctr3.ID(), graph.noDepNodes[0].id)
	assert.Equal(t, ctr1.ID(), graph.notDependedOnNodes[ctr1.ID()].id)
}

func TestBuildContainerGraphFourContainersNoEdges(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr3, err := getTestCtrN("3", manager)
	assert.NoError(t, err)
	ctr4, err := getTestCtrN("4", manager)
	assert.NoError(t, err)

	graph, err := BuildContainerGraph([]*Container{ctr1, ctr2, ctr3, ctr4})
	assert.NoError(t, err)
	assert.Equal(t, 4, len(graph.nodes))
	assert.Equal(t, 4, len(graph.noDepNodes))
	assert.Equal(t, 4, len(graph.notDependedOnNodes))

	node1, ok := graph.nodes[ctr1.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr1.ID(), node1.id)

	node2, ok := graph.nodes[ctr2.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr2.ID(), node2.id)

	node3, ok := graph.nodes[ctr3.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr3.ID(), node3.id)

	node4, ok := graph.nodes[ctr4.ID()]
	assert.True(t, ok)
	assert.Equal(t, ctr4.ID(), node4.id)
}

func TestBuildContainerGraphFourContainersTwoInCycle(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr3, err := getTestCtrN("3", manager)
	assert.NoError(t, err)
	ctr4, err := getTestCtrN("4", manager)
	assert.NoError(t, err)

	ctr1.config.IPCNsCtr = ctr2.config.ID
	ctr2.config.UserNsCtr = ctr1.config.ID

	_, err = BuildContainerGraph([]*Container{ctr1, ctr2, ctr3, ctr4})
	assert.Error(t, err)
}

func TestBuildContainerGraphFourContainersAllInCycle(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr3, err := getTestCtrN("3", manager)
	assert.NoError(t, err)
	ctr4, err := getTestCtrN("4", manager)
	assert.NoError(t, err)

	ctr1.config.IPCNsCtr = ctr2.config.ID
	ctr2.config.UserNsCtr = ctr3.config.ID
	ctr3.config.NetNsCtr = ctr4.config.ID
	ctr4.config.UTSNsCtr = ctr1.config.ID

	_, err = BuildContainerGraph([]*Container{ctr1, ctr2, ctr3, ctr4})
	assert.Error(t, err)
}

func TestBuildContainerGraphFourContainersNoneInCycle(t *testing.T) {
	manager, err := lock.NewInMemoryManager(16)
	if err != nil {
		t.Fatalf("Error setting up locks: %v", err)
	}

	ctr1, err := getTestCtr1(manager)
	assert.NoError(t, err)
	ctr2, err := getTestCtr2(manager)
	assert.NoError(t, err)
	ctr3, err := getTestCtrN("3", manager)
	assert.NoError(t, err)
	ctr4, err := getTestCtrN("4", manager)
	assert.NoError(t, err)

	ctr1.config.IPCNsCtr = ctr2.config.ID
	ctr1.config.NetNsCtr = ctr3.config.ID
	ctr2.config.UserNsCtr = ctr3.config.ID

	graph, err := BuildContainerGraph([]*Container{ctr1, ctr2, ctr3, ctr4})
	assert.NoError(t, err)
	assert.Equal(t, 4, len(graph.nodes))
	assert.Equal(t, 2, len(graph.noDepNodes))
	assert.Equal(t, 2, len(graph.notDependedOnNodes))
}
