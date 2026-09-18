//go:build !remote && (linux || freebsd)

package libpod

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/libpod/define"
)

// WaitForConditionWithInterval used to panic when it was called without any
// condition.  Callers cannot recover from that, so it has to report the
// invalid argument as a regular error instead.
func TestWaitForConditionWithIntervalNoConditions(t *testing.T) {
	c := &Container{valid: true}

	code, err := c.WaitForConditionWithInterval(context.Background(), time.Second)
	assert.Equal(t, int32(-1), code)
	assert.ErrorIs(t, err, define.ErrInvalidArg)
	assert.ErrorContains(t, err, "at least one condition should be passed")
}
