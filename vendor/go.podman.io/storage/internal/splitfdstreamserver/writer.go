package splitfdstreamserver

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	chunkMetadata   byte = 0x00
	chunkInlineData byte = 0x01
	// The consumer opens the file a FileBackedData chunk names with
	// openat2(RESOLVE_BENEATH), from the directory at the given index.
	chunkFileBackedData byte = 0x02

	// maxInlineChunkSize is the maximum size of a single metadata or
	// inline data chunk body.  It has to match MAX_INLINE_CHUNK_SIZE on
	// the consumer side, which buffers a whole chunk body in memory, and
	// bounds how much a peer can make it allocate.
	//
	// One chunk is one file: the format gives the consumer no way to tell
	// a continuation chunk from the body of the next file.  Inlining is
	// the fallback for entries with no usable file behind them, so a file
	// this large reaching it would be pathological.
	maxInlineChunkSize = 256 << 20
	// maxFilenameLen is the maximum filename length in a FileBackedData chunk.
	maxFilenameLen = 4096
)

// splitDirFDStreamWriter writes data in the composefs-rs splitdirfdstream
// format.
//
// The format carries a tar archive split in two.  Everything that makes
// up the structure of the archive - entry headers, the zero padding that
// follows each entry's content, and the end-of-archive marker - is sent
// as Metadata chunks.  The content of regular files is sent either as an
// InlineData chunk or, when the consumer can open the file for itself,
// as a FileBackedData chunk naming the file relative to one of the
// directory file descriptors passed alongside the stream.
//
// Concatenating, in stream order, the Metadata bodies, the InlineData
// bodies and the content of the files named by FileBackedData chunks
// reproduces the original tar archive byte for byte.
//
// Each chunk is prefixed by a type byte:
//   - 0x00 Metadata:       type(1) + length(u32 LE) + data
//   - 0x01 InlineData:     type(1) + length(u32 LE) + data
//   - 0x02 FileBackedData: type(1) + content_length(u64 LE) + dirfd_index(u32 LE) + name_len(u32 LE) + filename
type splitDirFDStreamWriter struct {
	writer io.Writer
}

func newWriter(w io.Writer) *splitDirFDStreamWriter {
	return &splitDirFDStreamWriter{writer: w}
}

// writeMetadata writes a metadata chunk, i.e. a part of the tar archive
// that is not file content: an entry header, inter-entry padding or the
// end-of-archive marker.
func (w *splitDirFDStreamWriter) writeMetadata(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if len(data) > maxInlineChunkSize {
		return fmt.Errorf("metadata chunk too large: %d > %d", len(data), maxInlineChunkSize)
	}
	buf := make([]byte, 0, 5+len(data))
	buf = append(buf, chunkMetadata)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(data)))
	buf = append(buf, data...)
	_, err := w.writer.Write(buf)
	return err
}

// inlineDataWriter writes an InlineData chunk header declaring size bytes
// of file content, and returns an io.Writer to write that content to.
//
// The caller must write exactly size bytes; the returned writer rejects
// any write past that point, because a short or long body would make the
// consumer read the following chunk headers as file content.
func (w *splitDirFDStreamWriter) inlineDataWriter(size int64) (io.Writer, error) {
	if size < 0 {
		return nil, fmt.Errorf("negative inline data chunk size %d", size)
	}
	if size > maxInlineChunkSize {
		return nil, fmt.Errorf("inline data chunk too large: %d > %d", size, maxInlineChunkSize)
	}
	hdr := make([]byte, 0, 5)
	hdr = append(hdr, chunkInlineData)
	hdr = binary.LittleEndian.AppendUint32(hdr, uint32(size))
	if _, err := w.writer.Write(hdr); err != nil {
		return nil, err
	}
	return &inlineBodyWriter{writer: w.writer, remaining: size}, nil
}

// writeFileBackedData writes a file-backed data chunk referencing a file
// in one of the directory FDs passed alongside the stream.
func (w *splitDirFDStreamWriter) writeFileBackedData(contentLength int64, dirfdIndex int, filename string) error {
	if len(filename) > maxFilenameLen {
		return fmt.Errorf("filename too long: %d > %d", len(filename), maxFilenameLen)
	}
	buf := make([]byte, 0, 1+8+4+4+len(filename))
	buf = append(buf, chunkFileBackedData)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(contentLength))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(dirfdIndex))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(filename)))
	buf = append(buf, filename...)
	_, err := w.writer.Write(buf)
	return err
}

// inlineBodyWriter enforces the body size declared by an InlineData
// chunk header.
type inlineBodyWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *inlineBodyWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("inline data chunk body overflow: %d bytes past the declared size", int64(len(p))-w.remaining)
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}
