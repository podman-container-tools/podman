//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/libpod/define"
)

func Test_createPodStatusResults(t *testing.T) {
	tests := []struct {
		name        string
		ctrStatuses map[string]define.ContainerStatus
		want        string
	}{
		{
			name:        "EmptyPod",
			ctrStatuses: map[string]define.ContainerStatus{},
			want:        define.PodStateCreated,
		},
		{
			name: "AllRunning",
			ctrStatuses: map[string]define.ContainerStatus{
				"ctr1": define.ContainerStateRunning,
				"ctr2": define.ContainerStateRunning,
				"ctr3": define.ContainerStateRunning,
			},
			want: define.PodStateRunning,
		},
		{
			name: "Degraded",
			ctrStatuses: map[string]define.ContainerStatus{
				"ctr1": define.ContainerStateRunning,
				"ctr2": define.ContainerStateStopped,
			},
			want: define.PodStateDegraded,
		},
		{
			name: "AllPaused",
			ctrStatuses: map[string]define.ContainerStatus{
				"ctr1": define.ContainerStatePaused,
				"ctr2": define.ContainerStatePaused,
			},
			want: define.PodStatePaused,
		},
		{
			name: "AllExited",
			ctrStatuses: map[string]define.ContainerStatus{
				"ctr1": define.ContainerStateExited,
				"ctr2": define.ContainerStateStopped,
			},
			want: define.PodStateExited,
		},
		{
			name: "PartialStopped",
			ctrStatuses: map[string]define.ContainerStatus{
				"ctr1": define.ContainerStateStopped,
				"ctr2": define.ContainerStateCreated,
			},
			want: define.PodStateStopped,
		},
		{
			name: "Errored",
			ctrStatuses: map[string]define.ContainerStatus{
				"ctr1": define.ContainerStateCreated,
				"ctr2": define.ContainerStateUnknown,
			},
			want: define.PodStateErrored,
		},
		{
			name: "AllCreated",
			ctrStatuses: map[string]define.ContainerStatus{
				"ctr1": define.ContainerStateCreated,
				"ctr2": define.ContainerStateConfigured,
			},
			want: define.PodStateCreated,
		},
		{
			name: "SingleRunning",
			ctrStatuses: map[string]define.ContainerStatus{
				"ctr1": define.ContainerStateRunning,
			},
			want: define.PodStateRunning,
		},
		{
			name: "ExitedCountsAsStopped",
			ctrStatuses: map[string]define.ContainerStatus{
				"ctr1": define.ContainerStateExited,
				"ctr2": define.ContainerStateExited,
			},
			want: define.PodStateExited,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createPodStatusResults(tt.ctrStatuses)
			assert.Equalf(t, tt.want, got, "createPodStatusResults(%v)", tt.ctrStatuses)
		})
	}
}
