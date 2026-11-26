package events

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.podman.io/storage/pkg/lockfile"
)

// The stream tests use /proc/self/fdinfo, Linux only

// Requires the next events from ch to have the given names.  It
// writes a "sync" event and reads up to it. Rotation events are ignored.
func requireNames(t *testing.T, eventer *EventLogFile, ch <-chan ReadResult, want ...string) {
	t.Helper()
	writeEvent(t, eventer, "sync")
	var names []string
	timeout := time.After(5 * time.Second)
	for {
		select {
		case r, ok := <-ch:
			require.True(t, ok, "event channel closed")
			require.NoError(t, r.Error)
			if r.Event.Type == System {
				continue
			}
			if r.Event.Name == "sync" {
				require.Equal(t, want, names)
				return
			}
			names = append(names, r.Event.Name)
		case <-timeout:
			t.Fatalf("got %v, want %v", names, want)
		}
	}
}

func eventLine(t *testing.T, name string, ts time.Time) string {
	t.Helper()
	s, err := (&Event{Type: Container, Status: Start, Name: name, Time: ts}).ToJSONString()
	require.NoError(t, err)
	return s
}

func writeEvent(t *testing.T, eventer *EventLogFile, name string) {
	t.Helper()
	require.NoError(t, eventer.writeString(eventLine(t, name, time.Now())))
}

func rotateMarker(t *testing.T, path, which string) string {
	t.Helper()
	e := NewEvent(Rotate)
	e.Type = System
	e.Name = path
	e.Attributes = map[string]string{rotateEventAttribute: which}
	s, err := e.ToJSONString()
	require.NoError(t, err)
	return s
}

// Replaces the log file the way truncate() does: a new, smaller file
// with the last keep lines of the log and the extra lines between rotation
// markers, renamed over the old one.  The tail notices a smaller file even if it
// started watching after the rename.
func replaceLog(t *testing.T, path string, keep int, extra ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	old := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	lines := append([]string{rotateMarker(t, path, rotateEventBegin)}, old[len(old)-keep:]...)
	lines = append(lines, extra...)
	lines = append(lines, rotateMarker(t, path, rotateEventEnd))
	content := strings.Join(lines, "\n") + "\n"
	// Leave room for later writes.
	require.Less(t, 2*len(content), len(data), "the copy must be much smaller than the log")
	tmp, err := os.CreateTemp(filepath.Dir(path), "")
	require.NoError(t, err)
	_, err = tmp.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tmp.Close())
	require.NoError(t, os.Rename(tmp.Name(), path))
}

func newTestEventer(t *testing.T, maxSize uint64) (*EventLogFile, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "events.log")
	eventer, err := newLogFileEventer(EventerOptions{LogFilePath: path, LogFileMaxSize: maxSize})
	require.NoError(t, err)
	return eventer, path
}

// Returns the offset of the open file descriptor of path, or -1 if
// there is none.
func tailOffset(path string) int64 {
	fds, _ := os.ReadDir("/proc/self/fd")
	for _, fd := range fds {
		if target, _ := os.Readlink("/proc/self/fd/" + fd.Name()); target != path {
			continue
		}
		info, err := os.ReadFile("/proc/self/fdinfo/" + fd.Name())
		if err != nil {
			continue
		}
		var pos int64
		if _, err := fmt.Sscanf(string(info), "pos: %d", &pos); err == nil {
			return pos
		}
	}
	return -1
}

// Waits until the tail of path has read to the end or closed the file
func waitTail(t *testing.T, path string, closed bool) {
	t.Helper()
	path, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		off := tailOffset(path)
		if closed {
			return off == -1
		}
		fi, err := os.Stat(path)
		return err == nil && off == fi.Size()
	}, 5*time.Second, 10*time.Millisecond)
}

// startReader starts streaming and waits until the tail is at the end of the
// file.  The file must not be empty, or the end cannot be told from the start.
func startReader(t *testing.T, ctx context.Context, eventer *EventLogFile, path string, ch chan ReadResult) {
	t.Helper()
	fi, err := os.Stat(path)
	require.NoError(t, err)
	require.NotZero(t, fi.Size())
	require.NoError(t, eventer.Read(ctx, ReadOptions{EventChannel: ch, Stream: true}))
	waitTail(t, path, false)
}

// An event written while Read waits for the events lock is returned.
// The last-line anchor relies on the tail starting before the lock.
func TestReadStreamStartWhileLocked(t *testing.T) {
	t.Parallel()
	eventer, path := newTestEventer(t, 0)
	writeEvent(t, eventer, "old")
	lock, err := lockfile.GetLockFile(path + ".lock")
	require.NoError(t, err)
	lock.Lock()
	// Unlock also on failure.
	unlock := sync.OnceFunc(lock.Unlock)
	defer unlock()
	ch := make(chan ReadResult, 10)
	errCh := make(chan error, 1)
	go func() {
		errCh <- eventer.Read(t.Context(), ReadOptions{EventChannel: ch, Stream: true})
	}()
	waitTail(t, path, false)
	writeEvent(t, eventer, "new")
	unlock()
	require.NoError(t, <-errCh)
	requireNames(t, eventer, ch, "new")
}

// A rotation copies new events into the new file.  They are returned once,
// and old events are not.  An event can be older than the reader, because
// writers set the time before they take the lock.
func TestReadStreamRotationIdleReader(t *testing.T) {
	t.Parallel()
	eventer, path := newTestEventer(t, 0)
	for i := range 30 {
		writeEvent(t, eventer, fmt.Sprintf("old-%d", i))
	}
	ts := time.Now()
	ch := make(chan ReadResult, 100)
	startReader(t, t.Context(), eventer, path, ch)

	replaceLog(t, path, 3, eventLine(t, "new", ts))
	writeEvent(t, eventer, "after")
	requireNames(t, eventer, ch, "new", "after")
}

// The same for a reader that has already returned events.
func TestReadStreamRotationActiveReader(t *testing.T) {
	t.Parallel()
	eventer, path := newTestEventer(t, 0)
	for i := range 30 {
		writeEvent(t, eventer, fmt.Sprintf("old-%d", i))
	}
	ts := time.Now()
	ch := make(chan ReadResult, 100)
	startReader(t, t.Context(), eventer, path, ch)
	writeEvent(t, eventer, "read")
	requireNames(t, eventer, ch, "read")

	replaceLog(t, path, 3, eventLine(t, "new", ts))
	requireNames(t, eventer, ch, "new")
}

// If the consumer is gone, the reader should stop.
func TestReadStreamCancel(t *testing.T) {
	t.Parallel()
	eventer, path := newTestEventer(t, 0)
	writeEvent(t, eventer, "old")
	ch := make(chan ReadResult)
	ctx, cancel := context.WithCancel(t.Context())
	startReader(t, ctx, eventer, path, ch)
	writeEvent(t, eventer, "unread")
	waitTail(t, path, false)
	// The tail reads "next" only after handing "unread" to the reader, which
	// then blocks on returning it.
	writeEvent(t, eventer, "next")
	waitTail(t, path, false)
	cancel()
	waitTail(t, path, true)
	select {
	case r, ok := <-ch:
		require.False(t, ok, "reader still running after cancel, returned %v", r.Event)
	case <-time.After(5 * time.Second):
		t.Fatal("event channel not closed after cancel")
	}
}

// A reader that keeps up does not lose events across rotations.
// Duplicates are still possible as before.
func TestReadStreamMultipleRotations(t *testing.T) {
	t.Parallel()
	// Large enough that a rotated file stays smaller than the reader's offset,
	// so the tail notices it even if its watcher started late.
	const maxSize = 3000
	eventer, path := newTestEventer(t, maxSize)
	writeEvent(t, eventer, "old")
	ch := make(chan ReadResult, 100)
	startReader(t, t.Context(), eventer, path, ch)

	// Write small chunks and wait for each to be read, so that rotations
	// happen while some events are not read yet.
	var written []string
	seen := map[string]bool{}
	rotations := 0
	for chunk := range 14 {
		for i := range 5 {
			name := fmt.Sprintf("ctr-%02d-%d", chunk, i)
			ev := Event{Type: Container, Status: Create, Name: name, Time: time.Now()}
			evJSON, err := ev.ToJSONString()
			require.NoError(t, err)
			rotate, err := logNeedsRotation(path, evJSON, maxSize)
			require.NoError(t, err)
			if rotate {
				rotations++
			}
			require.NoError(t, eventer.Write(ev))
			written = append(written, name)
		}
		timeout := time.After(5 * time.Second)
		for len(seen) < len(written) {
			select {
			case r := <-ch:
				require.NoError(t, r.Error)
				if r.Event.Type != System {
					seen[r.Event.Name] = true
				}
			case <-timeout:
				var missing []string
				for _, n := range written {
					if !seen[n] {
						missing = append(missing, n)
					}
				}
				t.Fatalf("chunk %d: events not delivered: %v", chunk, missing)
			}
		}
	}
	require.GreaterOrEqual(t, rotations, 3, "the log file must have been rotated")
}
