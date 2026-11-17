//go:build linux || freebsd

package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGenerateEventFilter_Network is a regression test for the missing NETWORK
// filter support.  Before this fix, calling generateEventFilter("network", ...)
// returned an error ("NETWORK is an invalid filter") instead of a working
// filter function.
func TestGenerateEventFilter_Network(t *testing.T) {
	tests := []struct {
		name        string
		filterValue string
		event       Event
		wantMatch   bool
	}{
		{
			name:        "match by network name",
			filterValue: "mynet",
			event:       Event{Type: Network, Network: "mynet"},
			wantMatch:   true,
		},
		{
			name:        "no match when network name differs",
			filterValue: "mynet",
			event:       Event{Type: Network, Network: "othernet"},
			wantMatch:   false,
		},
		{
			// For create/remove events e.ID is the network ID, so an
			// ID-prefix filter should match.
			name:        "match by network ID prefix on create event",
			filterValue: "abc123",
			event:       Event{Type: Network, Status: Create, ID: "abc123def456", Network: "mynet"},
			wantMatch:   true,
		},
		{
			// For connect/disconnect events e.ID is the container ID, not
			// the network ID.  A filter value that is a prefix of the
			// container ID must NOT produce a false positive.
			name:        "no false-positive on connect event with colliding container ID",
			filterValue: "abc123",
			event:       Event{Type: Network, Status: NetworkConnect, ID: "abc123def456", Network: "mynet"},
			wantMatch:   false,
		},
		{
			// Same guarantee for disconnect events.
			name:        "no false-positive on disconnect event with colliding container ID",
			filterValue: "abc123",
			event:       Event{Type: Network, Status: NetworkDisconnect, ID: "abc123def456", Network: "mynet"},
			wantMatch:   false,
		},
		{
			name:        "no match when event type is not Network",
			filterValue: "mynet",
			event:       Event{Type: Container, Network: "mynet"},
			wantMatch:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter, err := generateEventFilter("network", tc.filterValue)
			require.NoError(t, err, "generateEventFilter must not return an error for network filter")
			require.NotNil(t, filter)
			require.Equal(t, tc.wantMatch, filter(&tc.event))
		})
	}
}

// TestGenerateEventFilter_InvalidFilter verifies that an unrecognised filter
// key produces the expected error.
func TestGenerateEventFilter_InvalidFilter(t *testing.T) {
	_, err := generateEventFilter("doesnotexist", "value")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid filter")
}

// TestGenerateEventFilter_KnownFilters is a smoke-test that all documented
// filter keys are accepted without error.
func TestGenerateEventFilter_KnownFilters(t *testing.T) {
	knownFilters := []struct {
		key   string
		value string
	}{
		{"container", "mycontainer"},
		{"event", "start"},
		{"status", "stop"},
		{"image", "alpine"},
		{"artifact", "myartifact"},
		{"pod", "mypod"},
		{"volume", "myvol"},
		{"network", "mynet"},
		{"type", "container"},
		{"label", "mykey=myvalue"},
	}

	for _, kf := range knownFilters {
		t.Run(kf.key, func(t *testing.T) {
			filter, err := generateEventFilter(kf.key, kf.value)
			require.NoError(t, err)
			require.NotNil(t, filter)
		})
	}
}
