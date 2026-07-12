package copy

import (
	"context"
	"io"
	"math"
	"time"

	"go.podman.io/image/v5/internal/private"
	"go.podman.io/image/v5/types"
)

// blobChunkAccessorProxy wraps a BlobChunkAccessor to update a *progressBar
// and progressReporter with the number of received bytes.
type blobChunkAccessorProxy struct {
	wrapped  private.BlobChunkAccessor // The underlying BlobChunkAccessor
	bar      *progressBar              // A progress bar updated with the number of bytes read so far
	reporter progressReporter          // A progress reporter updated with the number of bytes read so far
}

// GetBlobAt returns a sequential channel of readers that contain data for the requested
// blob chunks, and a channel that might get a single error value.
// The specified chunks must be not overlapping and sorted by their offset.
// The readers must be fully consumed, in the order they are returned, before blocking
// to read the next chunk.
// If the Length for the last chunk is set to math.MaxUint64, then it
// fully fetches the remaining data from the offset to the end of the blob.
func (s *blobChunkAccessorProxy) GetBlobAt(ctx context.Context, info types.BlobInfo, chunks []private.ImageSourceChunk) (chan io.ReadCloser, chan error, error) {
	start := time.Now()
	rc, errs, err := s.wrapped.GetBlobAt(ctx, info, chunks)
	if err == nil {
		total := int64(0)
		for _, c := range chunks {
			// do not update the progress bar if there is a chunk with unknown length.
			if c.Length == math.MaxUint64 {
				return rc, errs, err
			}
			total += int64(c.Length)
		}
		s.reporter.reportRead(uint64(total))
		s.bar.EwmaIncrInt64(total, time.Since(start))
	}
	return rc, errs, err
}
