package copy

import (
	"io"
	"time"

	"go.podman.io/image/v5/types"
)

// progressReporter is an interface for reporting progress about a single blob.
type progressReporter interface {
	// reportRead reports progress with the number of `bytesRead`.
	reportRead(bytesRead uint64)
	// reportSuccess reports successful completion.
	reportSuccess()
	// reset resets the reporter's progress.
	reset()
}

// noopProgressReporter is a no-op implementation of progressReporter.
type noopProgressReporter struct{}

func (r *noopProgressReporter) reportRead(uint64) {}
func (r *noopProgressReporter) reportSuccess()    {}
func (r *noopProgressReporter) reset()            {}

// channelProgressReporter reports progress about a single blob to a
// types.ProgressProperties channel and supports re-starting from zero
// without reporting the progress through the channel unless
// it's higher than the offset reached before the restart to
// avoid confusing behavior in consumers of the events
// (skipping back).
type channelProgressReporter struct {
	channel           chan<- types.ProgressProperties // The reporter channel to which the progress will be sent
	interval          time.Duration                   // The update interval to indicate how often the progress should update
	artifact          types.BlobInfo                  // The blob metadata which is currently being progressed
	lastUpdate        time.Time                       // The last time a progress channel event was sent
	offset            uint64                          // The currently downloaded size in bytes
	maxReportedOffset uint64                          // The high-water mark for offset already sent to the channel
}

// newProgressReporter returns a channelProgressReporter if a progress channel
// and interval are configured, otherwise a noopProgressReporter.
func newProgressReporter(
	channel chan<- types.ProgressProperties,
	interval time.Duration,
	artifact types.BlobInfo,
) progressReporter {
	if channel == nil || interval <= 0 {
		return &noopProgressReporter{}
	}
	return newChannelProgressReporter(channel, interval, artifact)
}

// newChannelProgressReporter creates a new progress reporter
// and immediately reports a new artifact event.
func newChannelProgressReporter(
	channel chan<- types.ProgressProperties,
	interval time.Duration,
	artifact types.BlobInfo,
) progressReporter {
	channel <- types.ProgressProperties{
		Event:    types.ProgressEventNewArtifact,
		Artifact: artifact,
	}
	return &channelProgressReporter{
		channel:           channel,
		interval:          interval,
		artifact:          artifact,
		lastUpdate:        time.Now(),
		offset:            0,
		maxReportedOffset: 0,
	}
}

// reset resets the reporter's progress.
//
// It's meant to be used on error when
// the processing has to be re-started
// (e.g. ErrFallbackToOrdinaryLayerDownload).
func (r *channelProgressReporter) reset() {
	r.offset = 0
}

// reportRead reports progress with the number of `bytesRead`
// while keeping track of a current high-water mark in case
// of reset(). It never skips back below the already reported
// offset and does not report the progress unless
// the configured `interval` elapses.
func (r *channelProgressReporter) reportRead(bytesRead uint64) {
	r.offset += bytesRead
	if r.offset > r.maxReportedOffset && time.Since(r.lastUpdate) > r.interval {
		r.channel <- types.ProgressProperties{
			Event:        types.ProgressEventRead,
			Artifact:     r.artifact,
			Offset:       r.offset,
			OffsetUpdate: r.offset - r.maxReportedOffset,
		}
		r.maxReportedOffset = r.offset
		r.lastUpdate = time.Now()
	}
}

// reportSuccess reports successful completion.
func (r *channelProgressReporter) reportSuccess() {
	offset := max(r.offset, r.maxReportedOffset)
	r.channel <- types.ProgressProperties{
		Event:        types.ProgressEventDone,
		Artifact:     r.artifact,
		Offset:       offset,
		OffsetUpdate: offset - r.maxReportedOffset,
	}
}

// progressReader extends a wrapped io.Reader
// with additional reporting of its progress.
type progressReader struct {
	source io.Reader
	progressReporter
}

// newProgressReader creates a new progress reader that wraps source
// and reports progress through the given reporter.
func newProgressReader(
	source io.Reader,
	reporter progressReporter,
) *progressReader {
	return &progressReader{
		source:           source,
		progressReporter: reporter,
	}
}

// Read continuously reads bytes into the progress reader and reports the
// status via the internal channel.
func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.reportRead(uint64(n))
	return n, err
}
