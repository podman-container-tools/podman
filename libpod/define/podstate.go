package define

const (
	// PodStateCreated indicates the pod is created but has not been started
	PodStateCreated = "created"
	// PodStateErrored indicates the pod is in an errored state where
	// information about it can no longer be retrieved
	PodStateErrored = "error"
	// PodStateExited indicates the pod ran but has been stopped
	PodStateExited = "exited"
	// PodStatePaused indicates the pod has been paused
	PodStatePaused = "paused"
	// PodStateRunning indicates that all of the containers in the pod are
	// running.
	PodStateRunning = "running"
	// PodStateDegraded indicates that at least one, but not all, of the
	// containers in the pod are running.
	PodStateDegraded = "degraded"
	// PodStateStopped indicates all of the containers belonging to the pod
	// are stopped.
	PodStateStopped = "stopped"
)
