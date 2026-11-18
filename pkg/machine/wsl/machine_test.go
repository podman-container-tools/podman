//go:build windows

package wsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSystemRunningOutput(t *testing.T) {
	tests := []struct {
		name string
		out  []byte
		want bool
	}{
		{"running", []byte("running"), true},
		{"degraded", []byte("degraded"), true},
		{"initializing", []byte("initializing"), false},
		{"starting", []byte("starting"), false},
		{"maintenance", []byte("maintenance"), false},
		{"stopping", []byte("stopping"), false},
		{"offline", []byte("offline"), false},
		{"unknown", []byte("unknown"), false},
		{"empty string", []byte(""), false},
		{"warning before state", []byte("Failed to translate 'Z:\\x'\nrunning"), true},
		{"running and CRLF", []byte("running\r\n"), true},
		{"degraded and LF", []byte("degraded\n"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseSystemRunningOutput(tt.out))
		})
	}
}
