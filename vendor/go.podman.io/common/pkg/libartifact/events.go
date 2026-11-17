//go:build !remote

package libartifact

import (
	"time"

	"github.com/sirupsen/logrus"
)

// EventType indicates the type of an event.
type EventType int

const (
	// EventTypeUnknown is an uninitialized EventType.
	EventTypeUnknown EventType = iota
	// EventTypeArtifactPull represents an artifact pull.
	EventTypeArtifactPull
	// EventTypeArtifactPush represents an artifact push.
	EventTypeArtifactPush
	// EventTypeArtifactRemove represents an artifact removal.
	EventTypeArtifactRemove
	// EventTypeArtifactAdd represents an artifact being added.
	EventTypeArtifactAdd
)

// Event represents an event such as an artifact pull or push.
type Event struct {
	// ID of the object (e.g., artifact digest).
	ID string
	// Name of the object (e.g., artifact name "quay.io/foobar/artifact:special")
	Name string
	// Time of the event.
	Time time.Time
	// Type of the event.
	Type EventType
	// Error in case of failure.
	Error error
}

// writeEvent writes the specified event to the store's event channel. The
// event is discarded if no event channel has been registered (yet).
func (as *ArtifactStore) writeEvent(event *Event) {
	select {
	case as.eventChannel <- event:
		// Done
	case <-time.After(2 * time.Second):
		// The store's event channel has a buffer of size 100 which
		// should be enough even under high load.  However, we
		// shouldn't block too long in case the buffer runs full (could
		// be an honest user error or bug).
		logrus.Warnf("Discarding libartifact event which was not read within 2 seconds: %v", event)
	}
}
