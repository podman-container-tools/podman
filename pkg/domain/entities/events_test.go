package entities

import (
	"testing"
	"time"

	dockerEvents "github.com/moby/moby/api/types/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	libpodEvents "go.podman.io/podman/v6/libpod/events"
)

func newTestEvent(typ, action string, attrs map[string]string) Event {
	if attrs == nil {
		attrs = map[string]string{}
	}
	return Event{
		Message: dockerEvents.Message{
			Type:     dockerEvents.Type(typ),
			Action:   dockerEvents.Action(action),
			Actor:    dockerEvents.Actor{ID: "abc123", Attributes: attrs},
			TimeNano: 1700000000000000000,
		},
	}
}

func TestConvertToLibpodEvent(t *testing.T) {
	e := newTestEvent("container", "start", map[string]string{
		"image":             "alpine",
		"name":              "c1",
		"network":           "podman1",
		"podId":             "pod123",
		"containerExitCode": "137",
		"custom":            "value",
	})
	event, err := ConvertToLibpodEvent(e)
	require.NoError(t, err)
	require.NotNil(t, event)
	assert.Equal(t, libpodEvents.Container, event.Type)
	assert.Equal(t, libpodEvents.Start, event.Status)
	assert.Equal(t, "abc123", event.ID)
	assert.Equal(t, "alpine", event.Image)
	assert.Equal(t, "c1", event.Name)
	assert.Equal(t, "podman1", event.Network)
	assert.Equal(t, time.Unix(0, 1700000000000000000), event.Time)
	require.NotNil(t, event.ContainerExitCode)
	assert.Equal(t, 137, *event.ContainerExitCode)
	assert.Equal(t, "pod123", event.Details.PodID)
	assert.Equal(t, "value", event.Details.Attributes["custom"])
}

// Previously a server result that failed to parse was silently turned into a
// nil event, which surfaced to the user as the literal JSON "null". It must now
// return an error instead. See https://github.com/containers/podman/issues/28325
func TestConvertToLibpodEventReturnsError(t *testing.T) {
	tests := []struct {
		name   string
		typ    string
		action string
		attrs  map[string]string
	}{
		{name: "unknown status", typ: "container", action: "bogus-status"},
		{name: "empty action", typ: "container", action: ""},
		{name: "empty type", typ: "", action: "start"},
		{name: "invalid containerExitCode", typ: "container", action: "start", attrs: map[string]string{"containerExitCode": "not-a-number"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ConvertToLibpodEvent(newTestEvent(tt.typ, tt.action, tt.attrs))
			assert.Error(t, err)
			assert.Nil(t, event)
		})
	}
}
