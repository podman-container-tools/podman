package quadlet

import (
	"encoding/json"
	"fmt"
	"strings"

	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	"go.podman.io/podman/v6/pkg/systemd/quadlet"
)

const (
	defaultPeriodSeconds   = 10
	defaultTimeoutSeconds  = 1
	defaultFailureThresh   = 3
	defaultSuccessThresh   = 1
	defaultInitialDelaySec = 0
)

// applyProbes maps liveness, readiness, and startup probes onto unit.
//
// Quadlet exposes a single HealthCmd slot (plus a separate HealthStartupCmd).
// There is no independent readiness health check key, so liveness and readiness
// probes compete for the same slot. The priority is:
//
//	liveness  > readiness  (liveness is strictly stronger: same check + restart)
//
// When both are present, the readiness probe is silently dropped — the
// liveness probe already subsumes it. When only a readiness probe is present,
// it is mapped to HealthCmd without HealthOnFailure=restart because a failing
// readiness check does not imply the container should be restarted.
func applyProbes(unit *parser.UnitFile, liveness, readiness, startup *v1.Probe, restartPolicy v1.RestartPolicy) {
	switch {
	case liveness != nil:
		applyLivenessProbe(unit, liveness, restartPolicy)
	case readiness != nil:
		applyReadinessProbe(unit, readiness)
	}
	if startup != nil {
		applyStartupProbe(unit, startup)
	}
}

func applyLivenessProbe(unit *parser.UnitFile, p *v1.Probe, restartPolicy v1.RestartPolicy) {
	cmd := probeCommand(p.Handler)
	if cmd == "" {
		return
	}
	period := probeInt(p.PeriodSeconds, defaultPeriodSeconds)
	timeout := probeInt(p.TimeoutSeconds, defaultTimeoutSeconds)
	failure := probeInt(p.FailureThreshold, defaultFailureThresh)
	initial := probeInt(p.InitialDelaySeconds, defaultInitialDelaySec)

	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthCmd, cmd)
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthInterval, fmt.Sprintf("%ds", period))
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthRetries, fmt.Sprintf("%d", failure))
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthTimeout, fmt.Sprintf("%ds", timeout))
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthStartPeriod, fmt.Sprintf("%ds", initial))

	if restartPolicy == v1.RestartPolicyAlways || restartPolicy == v1.RestartPolicyOnFailure {
		unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthOnFailure, "restart")
	}
}

func applyReadinessProbe(unit *parser.UnitFile, p *v1.Probe) {
	cmd := probeCommand(p.Handler)
	if cmd == "" {
		return
	}
	period := probeInt(p.PeriodSeconds, defaultPeriodSeconds)
	timeout := probeInt(p.TimeoutSeconds, defaultTimeoutSeconds)
	failure := probeInt(p.FailureThreshold, defaultFailureThresh)
	initial := probeInt(p.InitialDelaySeconds, defaultInitialDelaySec)

	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthCmd, cmd)
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthInterval, fmt.Sprintf("%ds", period))
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthRetries, fmt.Sprintf("%d", failure))
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthTimeout, fmt.Sprintf("%ds", timeout))
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthStartPeriod, fmt.Sprintf("%ds", initial))
}

func applyStartupProbe(unit *parser.UnitFile, p *v1.Probe) {
	cmd := probeCommand(p.Handler)
	if cmd == "" {
		return
	}
	period := probeInt(p.PeriodSeconds, defaultPeriodSeconds)
	timeout := probeInt(p.TimeoutSeconds, defaultTimeoutSeconds)
	failure := probeInt(p.FailureThreshold, defaultFailureThresh)
	success := probeInt(p.SuccessThreshold, defaultSuccessThresh)

	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthStartupCmd, cmd)
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthStartupInterval, fmt.Sprintf("%ds", period))
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthStartupRetries, fmt.Sprintf("%d", failure))
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthStartupSuccess, fmt.Sprintf("%d", success))
	unit.Set(quadlet.ContainerGroup, quadlet.KeyHealthStartupTimeout, fmt.Sprintf("%ds", timeout))
}

// probeCommand converts a probe's Handler into the string Quadlet expects for HealthCmd.
func probeCommand(h v1.Handler) string {
	switch {
	case h.Exec != nil:
		data, err := json.Marshal(h.Exec.Command)
		if err != nil {
			return ""
		}
		return string(data)

	case h.HTTPGet != nil:
		hg := h.HTTPGet
		url := fmt.Sprintf("http://localhost:%s%s", hg.Port.String(), hg.Path)
		var headers []string
		for _, hdr := range hg.HTTPHeaders {
			headers = append(headers, fmt.Sprintf("-H %q", hdr.Name+": "+hdr.Value))
		}
		cmd := "curl -f -s"
		if len(headers) > 0 {
			cmd += " " + strings.Join(headers, " ")
		}
		return cmd + " " + url

	case h.TCPSocket != nil:
		return fmt.Sprintf("sh -c \"echo > /dev/tcp/localhost/%s\"", h.TCPSocket.Port.String())
	}
	return ""
}

// probeInt returns val when non-zero, otherwise the supplied default.
func probeInt(val int32, def int32) int32 {
	if val == 0 {
		return def
	}
	return val
}
