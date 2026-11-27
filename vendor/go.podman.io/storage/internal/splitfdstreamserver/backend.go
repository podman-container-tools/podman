package splitfdstreamserver

import (
	"io"

	"go.podman.io/storage/internal/driver"
)

// ImageBackend serves the layers of one image in a store.
type ImageBackend interface {
	// LayerIDByIndex returns the ID of the index-th layer of the image, counting
	// from 0 for the base layer.  It fails if the image has no such layer.
	LayerIDByIndex(index int) (string, error)

	// OpenLayer opens a layer for streaming.  The layer store holding the
	// layer stays locked for reading until the result is closed, so the
	// layer, and the names of its files, stay valid until then.
	//
	// Holding that lock holds up writers to the layer store, in this
	// process and in others, and a writer that is waiting holds up later
	// readers in turn.  So whoever holds a Layer must always be able to
	// make progress towards closing it, without waiting for anything that
	// may itself be waiting for the lock: in particular, it must not wait
	// for another OpenLayer.  That can deadlock even with no other writer,
	// because reading a layer store can need to lock it for writing to
	// repair it.  Many holders, each making progress on its own, are fine.
	//
	// layerID was resolved earlier, under a lock that has since been
	// released, so the layer may be gone by now.
	OpenLayer(layerID string) (Layer, error)
}

// Layer is a layer opened for streaming; see ImageBackend.OpenLayer.
// Everything it returns is read with the layer store locked throughout, so
// it is all consistent.
type Layer interface {
	// Files gives access to the files of the layer, so that their content
	// can be referenced instead of copied.
	Files() driver.LayerFiles

	// TarSplit returns the tar-split metadata recorded for the layer,
	// uncompressed.  Together with the content of the files it names, which
	// Files provides, it reproduces the blob the layer was created from, so
	// that what the consumer reassembles hashes to the expected diff ID.  It
	// can only be read once.
	TarSplit() io.Reader

	// Close releases the layer.
	Close() error
}
