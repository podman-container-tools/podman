package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecCreateConfigUnmarshalDetachKeys(t *testing.T) {
	t.Parallel()

	t.Run("omitted detach keys", func(t *testing.T) {
		t.Parallel()

		var cfg ExecCreateConfig
		err := json.Unmarshal([]byte(`{"Cmd":["echo"]}`), &cfg)
		require.NoError(t, err)
		assert.False(t, cfg.DetachKeysSet)
		assert.Empty(t, cfg.DetachKeys)
	})

	t.Run("empty detach keys disables detaching", func(t *testing.T) {
		t.Parallel()

		var cfg ExecCreateConfig
		err := json.Unmarshal([]byte(`{"Cmd":["echo"],"DetachKeys":""}`), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.DetachKeysSet)
		assert.Empty(t, cfg.DetachKeys)
	})

	t.Run("custom detach keys", func(t *testing.T) {
		t.Parallel()

		var cfg ExecCreateConfig
		err := json.Unmarshal([]byte(`{"Cmd":["echo"],"DetachKeys":"ctrl-a"}`), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.DetachKeysSet)
		assert.Equal(t, "ctrl-a", cfg.DetachKeys)
	})
}
