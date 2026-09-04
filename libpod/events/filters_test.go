//go:build linux || freebsd

package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateEventFilterSecret(t *testing.T) {
	t.Parallel()

	filterFunc, err := generateEventFilter("secret", "my-secret")
	require.NoError(t, err)
	require.NotNil(t, filterFunc)

	// Matching secret event by name
	secretEvent := &Event{
		Type: Secret,
		Name: "my-secret",
		ID:   "1234567890abcdef",
	}
	assert.True(t, filterFunc(secretEvent))

	// Matching secret event by ID prefix
	filterFuncByID, err := generateEventFilter("secret", "12345")
	require.NoError(t, err)
	assert.True(t, filterFuncByID(secretEvent))

	// Non-matching secret event
	otherSecretEvent := &Event{
		Type: Secret,
		Name: "other-secret",
		ID:   "9876543210fedcba",
	}
	assert.False(t, filterFunc(otherSecretEvent))

	// Non-secret event type with same name
	containerEvent := &Event{
		Type: Container,
		Name: "my-secret",
		ID:   "1234567890abcdef",
	}
	assert.False(t, filterFunc(containerEvent))
}
