//go:build !remote && (linux || freebsd)

package libpod

import "go.podman.io/podman/v6/libpod/define"

// GetPodStatus determines the status of the pod based on the
// statuses of the containers in the pod.
// Returns a string representation of the pod status
func (p *Pod) GetPodStatus() (string, error) {
	ctrStatuses, err := p.Status()
	if err != nil {
		return define.PodStateErrored, err
	}
	return createPodStatusResults(ctrStatuses), nil
}

func createPodStatusResults(ctrStatuses map[string]define.ContainerStatus) string {
	ctrNum := len(ctrStatuses)
	if ctrNum == 0 {
		return define.PodStateCreated
	}

	statusPodStateStopped := 0
	statusPodStateRunning := 0
	statusPodStatePaused := 0
	statusPodStateCreated := 0
	statusPodStateErrored := 0

	for _, ctrStatus := range ctrStatuses {
		switch ctrStatus {
		case define.ContainerStateExited:
			fallthrough
		case define.ContainerStateStopped:
			statusPodStateStopped++
		case define.ContainerStateRunning:
			statusPodStateRunning++
		case define.ContainerStatePaused:
			statusPodStatePaused++
		case define.ContainerStateCreated, define.ContainerStateConfigured:
			statusPodStateCreated++
		default:
			statusPodStateErrored++
		}
	}

	switch {
	case statusPodStateRunning == ctrNum:
		return define.PodStateRunning
	case statusPodStateRunning > 0:
		return define.PodStateDegraded
	case statusPodStatePaused == ctrNum:
		return define.PodStatePaused
	case statusPodStateStopped == ctrNum:
		return define.PodStateExited
	case statusPodStateStopped > 0:
		return define.PodStateStopped
	case statusPodStateErrored > 0:
		return define.PodStateErrored
	default:
		return define.PodStateCreated
	}
}
