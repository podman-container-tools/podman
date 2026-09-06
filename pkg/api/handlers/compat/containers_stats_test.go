//go:build !remote && (linux || freebsd)

package compat

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/libpod/define"
)

func TestStatsErrorStatus(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want int
	}{
		{name: "not found", err: define.ErrNoSuchCtr, want: http.StatusNotFound},
		{name: "removed", err: define.ErrCtrRemoved, want: http.StatusNotFound},
		{name: "stopped", err: define.ErrCtrStopped, want: http.StatusConflict},
		{name: "invalid state", err: define.ErrCtrStateInvalid, want: http.StatusConflict},
		{name: "no cgroups", err: define.ErrNoCgroups, want: http.StatusConflict},
		{name: "wrapped stopped", err: fmt.Errorf("collecting stats: %w", define.ErrCtrStopped), want: http.StatusConflict},
		{name: "internal", err: errors.New("permission denied"), want: http.StatusInternalServerError},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := statsErrorStatus(testCase.err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
