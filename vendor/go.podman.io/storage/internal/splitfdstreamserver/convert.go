// Package splitfdstreamserver serves layers as splitdirfdstream over the
// org.composefs.Oci varlink interface.
package splitfdstreamserver

import (
	"fmt"
	"io"

	"github.com/vbatts/tar-split/tar/storage"
	"go.podman.io/storage/internal/driver"
)

// copyBufferSize is the buffer used when content has to be copied into the
// stream rather than referenced.
const copyBufferSize = 1 << 20

// convert writes the layer described by the tar-split metadata to w in the
// splitdirfdstream format, referencing the content of regular files through
// files wherever it can, and copying it from files where it cannot.  With
// onlyWorldReadable, only files anyone may read are referenced.
//
// What the consumer reassembles has to hash to the layer's diff ID, so the
// archive is not regenerated: tar-split records every byte that is not file
// content - entry headers, extended headers, padding and whatever follows
// the end-of-archive marker - and those are passed through as they are.
// The content of a referenced file is never read here.
func convert(w io.Writer, layer Layer, onlyWorldReadable bool) error {
	metadata := storage.NewJSONUnpacker(layer.TarSplit())
	files := layer.Files()
	writer := newWriter(w)

	// Consecutive segments, typically the padding after one entry and the
	// header of the next, go out as a single metadata chunk.
	var pending []byte
	flush := func() error {
		err := writer.writeMetadata(pending)
		pending = pending[:0]
		return err
	}

	var buf []byte
	for {
		entry, err := metadata.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar-split metadata: %w", err)
		}

		switch entry.Type {
		case storage.SegmentType:
			if len(pending)+len(entry.Payload) > maxInlineChunkSize {
				if err := flush(); err != nil {
					return fmt.Errorf("writing tar metadata: %w", err)
				}
			}
			pending = append(pending, entry.Payload...)
			continue
		case storage.FileType:
		default:
			return fmt.Errorf("unknown tar-split entry type %d", entry.Type)
		}

		// Only entries with content have anything in the archive;
		// directories, links and empty files are all in the segments.
		if entry.Size == 0 {
			continue
		}
		if err := flush(); err != nil {
			return fmt.Errorf("writing tar metadata: %w", err)
		}

		name := entry.GetName()
		index, filename, ok, err := files.Lookup(name, entry.Size, onlyWorldReadable)
		if err != nil {
			return fmt.Errorf("looking up %q: %w", name, err)
		}
		if ok {
			if err := writer.writeFileBackedData(entry.Size, index, filename); err != nil {
				return fmt.Errorf("writing FileBackedData for %q: %w", name, err)
			}
			continue
		}

		if buf == nil {
			buf = make([]byte, copyBufferSize)
		}
		if err := copyInline(writer, files, name, entry.Size, buf); err != nil {
			return err
		}
	}
	if err := flush(); err != nil {
		return fmt.Errorf("writing tar metadata: %w", err)
	}
	return nil
}

// copyInline writes the content of the entry called name, which is size
// bytes long, as an InlineData chunk.
func copyInline(writer *splitDirFDStreamWriter, files driver.LayerFiles, name string, size int64, buf []byte) error {
	content, err := files.Open(name)
	if err != nil {
		return fmt.Errorf("opening %q: %w", name, err)
	}
	defer content.Close()

	iw, err := writer.inlineDataWriter(size)
	if err != nil {
		return fmt.Errorf("writing inline prefix for %q: %w", name, err)
	}
	// The chunk header has declared size bytes, so a file that has changed
	// size since the layer was created is an error rather than a short or
	// long body.
	n, err := io.CopyBuffer(iw, io.LimitReader(content, size), buf)
	if err != nil {
		return fmt.Errorf("writing inline content of %q: %w", name, err)
	}
	if n != size {
		return fmt.Errorf("content of %q is %d bytes, expected %d", name, n, size)
	}
	return nil
}
