package driver

import (
	"io"
	"os"

	graphdriver "go.podman.io/storage/drivers"
)

// SplitDirFDStreamDriver is implemented by storage drivers that can give a
// consumer direct access to the files of a layer.
type SplitDirFDStreamDriver interface {
	graphdriver.Driver

	// LayerFiles opens access to the files of a layer.  The caller is
	// responsible for closing the result.
	//
	// bigData reads the layer big data item stored under key, failing with
	// an error matching os.ErrNotExist if there is none.  A driver that
	// stores files under names other than their own finds those names in
	// the layer's table of contents, which pkg/chunked keeps as layer big
	// data rather than in the driver.  The caller holds the lock of the
	// layer store, so bigData can only be called before LayerFiles returns.
	LayerFiles(id string, bigData func(key string) (io.ReadCloser, error)) (LayerFiles, error)
}

// LayerFiles delegates access to the files making up one layer, so that
// their content can be referenced in a stream instead of copied into it.
type LayerFiles interface {
	// DirFDs returns the directories handed to the consumer alongside the
	// stream.  FileBackedData chunks reference them by index.
	DirFDs() []*os.File

	// Lookup reports where the consumer can find the content of the tar
	// entry called name, which is size bytes long.  It returns the index
	// of the directory in DirFDs() and the name to open relative to it.
	//
	// ok is false when the entry has no usable file behind it - it does
	// not exist under that name, it is not a regular file, it no longer
	// matches size, or onlyWorldReadable is set and it is not readable by
	// everyone - in which case the content has to be sent inline.
	Lookup(name string, size int64, onlyWorldReadable bool) (index int, filename string, ok bool, err error)

	// Open returns the content of the tar entry called name, for content
	// that cannot be referenced and has to be copied into the stream.  It
	// is read with the privileges of the caller, which is why it works for
	// files the consumer could not open itself.
	Open(name string) (io.ReadCloser, error)

	// Close releases the directory file descriptors.
	Close() error
}
