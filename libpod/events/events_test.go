package events

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToHumanReadableFormatting(t *testing.T) {
	t.Parallel()

	testTime := time.Date(2026, time.September, 30, 10, 15, 30, 123456789, time.UTC)
	timeStr := testTime.String()
	fullID := "1234567890abcdef1234567890"
	truncID := "1234567890ab"

	tests := []struct {
		name                       string
		event                      Event
		wantWithoutTimeUntruncated string
		wantWithoutTimeTruncated   string
	}{
		{
			name: "container basic",
			event: Event{
				Time:   testTime,
				Type:   Container,
				Status: Create,
				ID:     fullID,
				Image:  "docker.io/library/alpine:latest",
				Name:   "my-container",
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("container create %s (image=docker.io/library/alpine:latest, name=my-container)", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("container create %s (image=docker.io/library/alpine:latest, name=my-container)", truncID),
		},
		{
			name: "container with pod and attribute",
			event: Event{
				Time:   testTime,
				Type:   Container,
				Status: Start,
				ID:     fullID,
				Image:  "docker.io/library/nginx:latest",
				Name:   "my-nginx",
				Details: Details{
					PodID:      "pod1234567890abcdef",
					Attributes: map[string]string{"tier": "frontend"},
				},
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("container start %s (image=docker.io/library/nginx:latest, name=my-nginx, pod_id=pod1234567890abcdef, tier=frontend)", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("container start %s (image=docker.io/library/nginx:latest, name=my-nginx, pod_id=pod1234567890abcdef, tier=frontend)", truncID),
		},
		{
			name: "container with health status",
			event: Event{
				Time:                testTime,
				Type:                Container,
				Status:              HealthStatus,
				ID:                  fullID,
				Image:               "docker.io/library/alpine:latest",
				Name:                "health-ctr",
				HealthStatus:        "healthy",
				HealthFailingStreak: 3,
				HealthLog:           "exit 0",
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("container health_status %s (image=docker.io/library/alpine:latest, name=health-ctr, health_status=healthy, health_failing_streak=3, health_log=exit 0)", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("container health_status %s (image=docker.io/library/alpine:latest, name=health-ctr, health_status=healthy, health_failing_streak=3, health_log=exit 0)", truncID),
		},
		{
			name: "pod basic",
			event: Event{
				Time:   testTime,
				Type:   Pod,
				Status: Create,
				ID:     fullID,
				Name:   "my-pod",
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("pod create %s (image=, name=my-pod)", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("pod create %s (image=, name=my-pod)", truncID),
		},
		{
			name: "network create with driver",
			event: Event{
				Time:    testTime,
				Type:    Network,
				Status:  Create,
				ID:      fullID,
				Network: "podman-net",
				Details: Details{
					Attributes: map[string]string{"driver": "bridge"},
				},
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("network create %s (name=podman-net, type=bridge)", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("network create %s (name=podman-net, type=bridge)", fullID),
		},
		{
			name: "network remove with driver",
			event: Event{
				Time:    testTime,
				Type:    Network,
				Status:  Remove,
				ID:      fullID,
				Network: "podman-net",
				Details: Details{
					Attributes: map[string]string{"driver": "bridge"},
				},
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("network remove %s (name=podman-net, type=bridge)", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("network remove %s (name=podman-net, type=bridge)", fullID),
		},
		{
			name: "network connect",
			event: Event{
				Time:    testTime,
				Type:    Network,
				Status:  NetworkConnect,
				ID:      fullID,
				Network: "podman-net",
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("network connect %s (container=%s, name=podman-net)", fullID, fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("network connect %s (container=%s, name=podman-net)", truncID, truncID),
		},
		{
			name: "network disconnect",
			event: Event{
				Time:    testTime,
				Type:    Network,
				Status:  NetworkDisconnect,
				ID:      fullID,
				Network: "podman-net",
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("network disconnect %s (container=%s, name=podman-net)", fullID, fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("network disconnect %s (container=%s, name=podman-net)", truncID, truncID),
		},
		{
			name: "image pull",
			event: Event{
				Time:   testTime,
				Type:   Image,
				Status: Pull,
				ID:     fullID,
				Name:   "quay.io/libpod/alpine:latest",
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("image pull %s quay.io/libpod/alpine:latest", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("image pull %s quay.io/libpod/alpine:latest", truncID),
		},
		{
			name: "image pull error",
			event: Event{
				Time:   testTime,
				Type:   Image,
				Status: PullError,
				ID:     fullID,
				Name:   "quay.io/libpod/alpine:latest",
				Error:  "manifest unknown",
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("image pull-error %s quay.io/libpod/alpine:latest manifest unknown", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("image pull-error %s quay.io/libpod/alpine:latest manifest unknown", truncID),
		},
		{
			name: "artifact create",
			event: Event{
				Time:   testTime,
				Type:   Artifact,
				Status: Create,
				ID:     fullID,
				Name:   "my-artifact",
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("artifact create %s my-artifact", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("artifact create %s my-artifact", truncID),
		},
		{
			name: "system with name",
			event: Event{
				Time:   testTime,
				Type:   System,
				Status: Refresh,
				Name:   "system-test",
			},
			wantWithoutTimeUntruncated: "system refresh system-test",
			wantWithoutTimeTruncated:   "system refresh system-test",
		},
		{
			name: "system without name",
			event: Event{
				Time:   testTime,
				Type:   System,
				Status: Refresh,
				Name:   "",
			},
			wantWithoutTimeUntruncated: "system refresh",
			wantWithoutTimeTruncated:   "system refresh",
		},
		{
			name: "machine start",
			event: Event{
				Time:   testTime,
				Type:   Machine,
				Status: Start,
				Name:   "podman-machine-default",
			},
			wantWithoutTimeUntruncated: "machine start podman-machine-default",
			wantWithoutTimeTruncated:   "machine start podman-machine-default",
		},
		{
			name: "volume create",
			event: Event{
				Time:   testTime,
				Type:   Volume,
				Status: Create,
				Name:   "my-vol",
			},
			wantWithoutTimeUntruncated: "volume create my-vol",
			wantWithoutTimeTruncated:   "volume create my-vol",
		},
		{
			name: "secret create",
			event: Event{
				Time:   testTime,
				Type:   Secret,
				Status: Create,
				ID:     fullID,
			},
			wantWithoutTimeUntruncated: fmt.Sprintf("secret create %s", fullID),
			wantWithoutTimeTruncated:   fmt.Sprintf("secret create %s", truncID),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// 1. Without timestamp (untruncated)
			gotWithoutTime := tc.event.ToHumanReadableWithoutTime(false)
			assert.Equal(t, tc.wantWithoutTimeUntruncated, gotWithoutTime)
			expectedPrefix := fmt.Sprintf("%s %s", tc.event.Type, tc.event.Status)
			assert.True(t, strings.HasPrefix(gotWithoutTime, expectedPrefix),
				"output should start directly with '%s', got: %s", expectedPrefix, gotWithoutTime)
			assert.Equal(t, strings.TrimLeft(gotWithoutTime, " \t\r\n"), gotWithoutTime,
				"output without timestamp must not contain leading whitespace")
			assert.Equal(t, strings.TrimRight(gotWithoutTime, " \t\r\n"), gotWithoutTime,
				"output without timestamp must not contain trailing whitespace")

			// 2. Without timestamp (truncated)
			gotWithoutTimeTrunc := tc.event.ToHumanReadableWithoutTime(true)
			assert.Equal(t, tc.wantWithoutTimeTruncated, gotWithoutTimeTrunc)
			assert.True(t, strings.HasPrefix(gotWithoutTimeTrunc, expectedPrefix),
				"truncated output should start directly with '%s', got: %s", expectedPrefix, gotWithoutTimeTrunc)
			assert.Equal(t, strings.TrimLeft(gotWithoutTimeTrunc, " \t\r\n"), gotWithoutTimeTrunc,
				"truncated output without timestamp must not contain leading whitespace")
			assert.Equal(t, strings.TrimRight(gotWithoutTimeTrunc, " \t\r\n"), gotWithoutTimeTrunc,
				"truncated output without timestamp must not contain trailing whitespace")

			// 3. With timestamp (untruncated) - preserves backward compatibility
			gotWithTime := tc.event.ToHumanReadable(false)
			wantWithTime := fmt.Sprintf("%s %s", timeStr, tc.wantWithoutTimeUntruncated)
			assert.Equal(t, wantWithTime, gotWithTime)
			assert.True(t, strings.HasPrefix(gotWithTime, timeStr),
				"output should start with timestamp prefix, got: %s", gotWithTime)

			// 4. With timestamp (truncated)
			gotWithTimeTrunc := tc.event.ToHumanReadable(true)
			wantWithTimeTrunc := fmt.Sprintf("%s %s", timeStr, tc.wantWithoutTimeTruncated)
			assert.Equal(t, wantWithTimeTrunc, gotWithTimeTrunc)
		})
	}
}

func TestToHumanReadableNilEvent(t *testing.T) {
	t.Parallel()

	var nilEvent *Event
	assert.Empty(t, nilEvent.ToHumanReadable(false))
	assert.Empty(t, nilEvent.ToHumanReadable(true))
	assert.Empty(t, nilEvent.ToHumanReadableWithoutTime(false))
	assert.Empty(t, nilEvent.ToHumanReadableWithoutTime(true))
}
