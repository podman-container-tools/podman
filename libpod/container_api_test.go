//go:build !remote && (linux || freebsd)

package libpod

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/libpod/events"
)

func TestWaitForConditionResultPreservesReceivedExitCode(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	resultChan := make(chan waitResult)
	go func() {
		resultChan <- waitResult{code: 7}
		cancel()
	}()

	code, err := waitForConditionResult(ctx, resultChan)
	require.NoError(t, err)
	require.Equal(t, int32(7), code)
}

func TestWaitForNextExitEventStreamClosed(t *testing.T) {
	readErr := errors.New("bad event")
	tests := []struct {
		name    string
		results []events.ReadResult
		wantErr string
	}{
		{"without events", nil, "event stream closed before the container exited"},
		{"after read error", []events.ReadResult{{Error: readErr}}, readErr.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := make(chan events.ReadResult, len(tt.results))
			for _, r := range tt.results {
				ch <- r
			}
			close(ch)

			code, err := waitForNextExitEvent(ch)
			assert.EqualError(t, err, tt.wantErr)
			assert.Equal(t, int32(-1), code)
		})
	}
}
