//go:build !remote && (linux || freebsd) && !systemd

package libpod

import (
	"bufio"
	jsonlib "encoding/json"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/sirupsen/logrus"
)

const (
	// Healthcheck message type from conmon (using negative to avoid PID conflicts)
	HealthCheckMsgStatusUpdate = -100

	// Healthcheck status values sent by conmon (added to base message type -100)
	HealthCheckStatusNone      = 0
	HealthCheckStatusStarting  = 1
	HealthCheckStatusHealthy   = 2
	HealthCheckStatusUnhealthy = 3
)

// createOCIContainer generates this container's main conmon instance with healthcheck support
func (r *ConmonOCIRuntime) createOCIContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (int64, error) {
	// Call the base implementation from common file
	result, err := r.createOCIContainerBase(ctr, restoreOptions)
	if err != nil {
		return result, err
	}

	// Add healthcheck-specific logic for non-systemd builds
	logrus.Debugf("HEALTHCHECK: Container %s created with healthcheck support", ctr.ID())

	return result, nil
}

// readConmonPipeData reads container creation response and starts healthcheck monitoring
func readConmonPipeData(runtimeName string, pipe *os.File, ociLog string, ctr ...*Container) (int, error) {
	// Call the base implementation from common file
	data, err := readConmonPipeDataBase(runtimeName, pipe, ociLog, ctr...)
	if err != nil {
		return data, err
	}

	// Add healthcheck monitoring for non-systemd builds
	if len(ctr) > 0 && ctr[0] != nil && data > 0 {
		logrus.Debugf("HEALTHCHECK: Starting pipe monitoring for container %s (PID: %d)", ctr[0].ID(), data)
		startContinuousPipeMonitoring(ctr[0], pipe, data)
	}

	return data, nil
}

// addHealthCheckArgs adds healthcheck-related arguments to conmon for non-systemd builds
func (r *ConmonOCIRuntime) addHealthCheckArgs(ctr *Container, args []string) []string {
	// Add healthcheck configuration as CLI arguments if container has healthcheck config
	if ctr.HasHealthCheck() {
		healthConfig := ctr.HealthCheckConfig()
		if healthConfig != nil {
			logrus.Debugf("HEALTHCHECK: Adding healthcheck CLI args for container %s", ctr.ID())

			// Build healthcheck command and arguments from test array
			healthCmd, healthArgs := r.buildHealthcheckCmdAndArgs(healthConfig.Test)
			if healthCmd != "" {
				args = append(args, "--healthcheck-cmd", healthCmd)

				// Add all healthcheck arguments
				for _, arg := range healthArgs {
					args = append(args, "--healthcheck-arg", arg)
				}

				// Add optional healthcheck parameters with validation and defaults
				interval := r.validateAndGetInterval(healthConfig.Interval)
				timeout := r.validateAndGetTimeout(healthConfig.Timeout)
				retries := r.validateAndGetRetries(healthConfig.Retries)
				startPeriod := r.validateAndGetStartPeriod(healthConfig.StartPeriod)

				args = append(args, "--healthcheck-interval", strconv.Itoa(interval))
				args = append(args, "--healthcheck-timeout", strconv.Itoa(timeout))
				args = append(args, "--healthcheck-retries", strconv.Itoa(retries))
				args = append(args, "--healthcheck-start-period", strconv.Itoa(startPeriod))

				logrus.Debugf("HEALTHCHECK: Added healthcheck args for container %s: cmd=%s, args=%v, interval=%ds, timeout=%ds, retries=%d, start-period=%ds",
					ctr.ID(), healthCmd, healthArgs, interval, timeout, retries, startPeriod)
			} else {
				logrus.Warnf("HEALTHCHECK: Container %s has healthcheck config but no valid command", ctr.ID())
			}
		}
	} else {
		logrus.Debugf("HEALTHCHECK: Container %s does not have healthcheck config, skipping healthcheck args", ctr.ID())
	}
	return args
}

// startContinuousPipeMonitoring starts continuous pipe monitoring for non-systemd builds
func startContinuousPipeMonitoring(ctr *Container, pipe *os.File, pid int) {
	logrus.Debugf("Starting continuous pipe monitoring for container %s (PID: %d)", ctr.ID())
	go readConmonHealthCheckPipeData(ctr, pipe)
}

// readConmonHealthCheckPipeData continuously reads healthcheck status updates from conmon
func readConmonHealthCheckPipeData(ctr *Container, pipe *os.File) {
	logrus.Debugf("HEALTHCHECK: Starting continuous healthcheck monitoring for container %s", ctr.ID())

	rdr := bufio.NewReader(pipe)
	for {
		// Read one line from the pipe
		b, err := rdr.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				logrus.Debugf("HEALTHCHECK: Pipe closed for container %s, stopping monitoring", ctr.ID())
				return
			}
			logrus.Errorf("HEALTHCHECK: Error reading from pipe for container %s: %v", ctr.ID(), err)
			return
		}

		// Log the raw JSON string received from conmon
		logrus.Debugf("HEALTHCHECK: Raw JSON received from conmon for container %s: %q", ctr.ID(), string(b))
		logrus.Debugf("HEALTHCHECK: JSON length: %d bytes", len(b))

		// Parse the JSON
		var si syncInfo
		if err := jsonlib.Unmarshal(b, &si); err != nil {
			logrus.Errorf("HEALTHCHECK: Failed to parse JSON from conmon for container %s: %v", ctr.ID(), err)
			continue
		}

		logrus.Debugf("HEALTHCHECK: Parsed sync info for container %s: Data=%d, Message=%q", ctr.ID(), si.Data, si.Message)

		// Handle healthcheck status updates based on your new encoding scheme
		// Base message type is -100, status values are added to it:
		// -100 + 0 (none) = -100
		// -100 + 1 (starting) = -99
		// -100 + 2 (healthy) = -98
		// -100 + 3 (unhealthy) = -97
		if si.Data >= HealthCheckMsgStatusUpdate && si.Data <= HealthCheckMsgStatusUpdate+HealthCheckStatusUnhealthy {
			statusValue := si.Data - HealthCheckMsgStatusUpdate // Convert back to status value
			var status string

			switch statusValue {
			case HealthCheckStatusNone:
				status = define.HealthCheckReset // "reset" or "none"
			case HealthCheckStatusStarting:
				status = define.HealthCheckStarting // "starting"
			case HealthCheckStatusHealthy:
				status = define.HealthCheckHealthy // "healthy"
			case HealthCheckStatusUnhealthy:
				status = define.HealthCheckUnhealthy // "unhealthy"
			default:
				logrus.Errorf("HEALTHCHECK: Unknown status value %d for container %s", statusValue, ctr.ID())
				continue
			}

			logrus.Infof("HEALTHCHECK: Received healthcheck status update for container %s: %s (message type: %d, status value: %d)",
				ctr.ID(), status, si.Data, statusValue)

			// Update the container's healthcheck status
			if err := ctr.updateHealthStatus(status); err != nil {
				logrus.Errorf("HEALTHCHECK: Failed to update healthcheck status for container %s: %v", ctr.ID(), err)
			} else {
				logrus.Infof("HEALTHCHECK: Successfully updated healthcheck status for container %s to %s", ctr.ID(), status)
			}
		} else if si.Data < 0 {
			// Other negative message types - might be healthcheck related but not recognized
			logrus.Debugf("HEALTHCHECK: Received unrecognized negative message type %d for container %s - might be healthcheck related", si.Data, ctr.ID())
		} else if si.Data > 0 {
			// Positive message types - not healthcheck related
			logrus.Debugf("HEALTHCHECK: Received positive message type %d for container %s - not healthcheck related", si.Data, ctr.ID())
		}
	}
}

// buildHealthcheckCmdAndArgs converts Podman's healthcheck test array to command and arguments
func (r *ConmonOCIRuntime) buildHealthcheckCmdAndArgs(test []string) (string, []string) {
	if len(test) == 0 {
		return "", nil
	}

	// Handle special cases
	switch test[0] {
	case "", "NONE":
		return "", nil
	case "CMD":
		// CMD format: ["CMD", "curl", "-f", "http://localhost:8080/health"]
		// -> cmd="curl", args=["-f", "http://localhost:8080/health"]
		if len(test) > 1 {
			return test[1], test[2:]
		}
		return "", nil
	case "CMD-SHELL":
		// CMD-SHELL format: ["CMD-SHELL", "curl -f http://localhost:8080/health"]
		// -> cmd="/bin/sh", args=["-c", "curl -f http://localhost:8080/health"]
		if len(test) > 1 {
			return "/bin/sh", []string{"-c", test[1]}
		}
		return "", nil
	default:
		// Direct command format: ["curl", "-f", "http://localhost:8080/health"]
		// -> cmd="curl", args=["-f", "http://localhost:8080/health"]
		return test[0], test[1:]
	}
}

// validateAndGetInterval validates and returns the healthcheck interval in seconds
func (r *ConmonOCIRuntime) validateAndGetInterval(interval time.Duration) int {
	// Default interval is 30 seconds
	if interval <= 0 {
		return 30
	}
	// Ensure minimum interval of 1 second
	if interval < time.Second {
		logrus.Warnf("HEALTHCHECK: Interval %v is less than 1 second, using 1 second", interval)
		return 1
	}
	return int(interval.Seconds())
}

// validateAndGetTimeout validates and returns the healthcheck timeout in seconds
func (r *ConmonOCIRuntime) validateAndGetTimeout(timeout time.Duration) int {
	// Default timeout is 30 seconds
	if timeout <= 0 {
		return 30
	}
	// Ensure minimum timeout of 1 second
	if timeout < time.Second {
		logrus.Warnf("HEALTHCHECK: Timeout %v is less than 1 second, using 1 second", timeout)
		return 1
	}
	return int(timeout.Seconds())
}

// validateAndGetRetries validates and returns the healthcheck retries count
func (r *ConmonOCIRuntime) validateAndGetRetries(retries int) int {
	// Default retries is 3
	if retries <= 0 {
		return 3
	}
	// Ensure reasonable maximum retries (conmon should handle this too)
	if retries > 10 {
		logrus.Warnf("HEALTHCHECK: Retries %d is very high, using 10", retries)
		return 10
	}
	return retries
}

// validateAndGetStartPeriod validates and returns the healthcheck start period in seconds
func (r *ConmonOCIRuntime) validateAndGetStartPeriod(startPeriod time.Duration) int {
	// Default start period is 0 seconds
	if startPeriod < 0 {
		logrus.Warnf("HEALTHCHECK: Start period %v is negative, using 0", startPeriod)
		return 0
	}
	return int(startPeriod.Seconds())
}
