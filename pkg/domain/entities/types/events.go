package types

import (
	dockerEvents "github.com/moby/moby/api/types/events"
)

// Event combines various event-related data such as time, event type, status
// and more.
type Event struct {
	// TODO: it would be nice to have full control over the types at some
	// point and fork such Docker types.
	dockerEvents.Message
	HealthStatus string `json:",omitempty"`
	// Deprecated: use Action instead.
	// Information from JSONMessage.
	// With data only in container events.
	Status string `json:"status,omitempty"`
	// Deprecated: use Actor.ID instead.
	ID string `json:"id,omitempty"`
	// Deprecated: use Actor.Attributes["image"] instead.
	From string `json:"from,omitempty"`
}
