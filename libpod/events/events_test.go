package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringToStatusRoundTrip(t *testing.T) {
	// Every Status constant must survive a round-trip through StringToStatus.
	allStatuses := []Status{
		Attach,
		AutoUpdate,
		Build,
		Checkpoint,
		Cleanup,
		Commit,
		Copy,
		Create,
		Exec,
		ExecDied,
		Exited,
		Export,
		HealthStatus,
		History,
		Import,
		Init,
		Kill,
		LoadFromArchive,
		Mount,
		NetworkConnect,
		NetworkDisconnect,
		Pause,
		Prune,
		Pull,
		PullError,
		Push,
		Refresh,
		Remove,
		Rename,
		Renumber,
		Restart,
		Restore,
		Rotate,
		Save,
		Start,
		Stop,
		Sync,
		Tag,
		Unmount,
		Unpause,
		Untag,
		Update,
	}

	for _, s := range allStatuses {
		t.Run(string(s), func(t *testing.T) {
			got, err := StringToStatus(s.String())
			require.NoError(t, err, "StringToStatus(%q) should not error", s.String())
			assert.Equal(t, s, got)
		})
	}
}

func TestStringToStatusUnknown(t *testing.T) {
	_, err := StringToStatus("bogus")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown event status")
}
