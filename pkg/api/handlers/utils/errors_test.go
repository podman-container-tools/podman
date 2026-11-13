//go:build !remote

package utils

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	secretsdefine "go.podman.io/common/pkg/secrets/define"
)

func TestGetCreateSecretStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"plain error", errors.New("oops"), http.StatusInternalServerError},
		{"invalid argument", secretsdefine.ErrInvalidArg, http.StatusBadRequest},
		{"wrapped invalid argument", fmt.Errorf("bad name: %w", secretsdefine.ErrInvalidArg), http.StatusBadRequest},
		{"secret name in use", fmt.Errorf("x: %w", secretsdefine.ErrSecretExists), http.StatusConflict},
		{"conflict error wins over invalid argument", fmt.Errorf("%w: %w", secretsdefine.ErrSecretExists, secretsdefine.ErrInvalidArg), http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetCreateSecretStatus(tt.err))
		})
	}
}
