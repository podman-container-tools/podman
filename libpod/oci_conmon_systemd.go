//go:build !remote && (linux || freebsd) && systemd

package libpod

import "os"

// createOCIContainer generates this container's main conmon instance (systemd version)
func (r *ConmonOCIRuntime) createOCIContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (int64, error) {
	// Call the base implementation from common file
	return r.createOCIContainerBase(ctr, restoreOptions)
}

// readConmonPipeData reads container creation response (systemd version)
func readConmonPipeData(runtimeName string, pipe *os.File, ociLog string, ctr ...*Container) (int, error) {
	// Call the base implementation from common file
	return readConmonPipeDataBase(runtimeName, pipe, ociLog, ctr...)
}

// addHealthCheckArgs is a no-op for systemd builds
// Systemd manages healthchecks via systemd timers, not conmon CLI arguments
func (r *ConmonOCIRuntime) addHealthCheckArgs(ctr *Container, args []string) []string {
	// No-op: systemd handles healthchecks via timers
	return args
}

// startContinuousPipeMonitoring is a no-op for systemd builds
// Systemd manages healthchecks via systemd timers, not conmon pipe messages
func startContinuousPipeMonitoring(ctr *Container, pipe *os.File, pid int) {
	// No-op: systemd handles healthchecks via timers
}

// readConmonHealthCheckPipeData is a no-op for systemd builds
// Systemd manages healthchecks via systemd timers, not conmon pipe messages
func readConmonHealthCheckPipeData(ctr *Container, pipe *os.File) {
	// No-op: systemd handles healthchecks via timers
}
