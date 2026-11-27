package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"

	drivers "go.podman.io/storage/drivers"
	internalDriver "go.podman.io/storage/internal/driver"
	"go.podman.io/storage/internal/splitfdstreamserver"
)

// SplitFDStreamStore gives access to the splitfdstream capabilities of a
// store.  Use SplitFDStreamStoreFor to obtain one.
//
// This API is experimental and can be changed without bumping the major version number.
type SplitFDStreamStore struct {
	store *store
}

// SplitFDStreamStoreFor returns the splitfdstream capabilities of s.
//
// This API is experimental and can be changed without bumping the major version number.
func SplitFDStreamStoreFor(s Store) (*SplitFDStreamStore, error) {
	store, ok := s.(*store)
	if !ok {
		return nil, fmt.Errorf("store implementation %T does not support splitdirfdstream operations: %w", s, drivers.ErrNotSupported)
	}
	return &SplitFDStreamStore{store: store}, nil
}

// Socket returns a socket on which the store serves org.composefs.Oci for
// the image with the given ID until the client closes it.  The caller is
// responsible for closing the returned file once it has handed it on.
//
// Layers are addressed by their position in the image, following the
// store's own record of which layers make up the image: the diff IDs in
// the image's config only identify those layers if they were enforced when
// the image was pulled.
//
// The client is responsible for verifying that each layer it reassembles
// is what it expects, normally by hashing it and comparing the result to the
// diff ID in the image's config (which only identifies the layer if it was
// enforced when the image was pulled), and for closing the lifetime
// descriptor of a layer as soon as it is done with it, without waiting for
// another layer in the meantime: until then, the layer is locked against
// writers.
//
// This API is experimental and can be changed without bumping the major version number.
func (s *SplitFDStreamStore) Socket(imageID string) (*os.File, error) {
	if err := s.store.startUsingGraphDriver(); err != nil {
		return nil, err
	}
	_, ok := s.store.graphDriver.(internalDriver.SplitDirFDStreamDriver)
	driverName := s.store.graphDriver.String()
	s.store.stopUsingGraphDriver()
	if !ok {
		return nil, fmt.Errorf("driver %s does not support splitdirfdstream operations: %w", driverName, drivers.ErrNotSupported)
	}

	layerIDs, err := s.imageLayerIDs(imageID)
	if err != nil {
		return nil, err
	}
	backend := &splitFDStreamBackend{store: s.store, layerIDs: layerIDs}
	s.store.splitFDStreamLock.Lock()
	defer s.store.splitFDStreamLock.Unlock()
	if s.store.splitFDStreamServer == nil {
		s.store.splitFDStreamServer = splitfdstreamserver.NewServer()
	}
	// Streams are written to unnamed files, so any directory on the store's
	// file system will do.
	return s.store.splitFDStreamServer.Connection(backend, s.store.graphRoot)
}

// imageLayerIDs returns the IDs of the layers of the image with the given
// ID, base layer first.
func (s *SplitFDStreamStore) imageLayerIDs(imageID string) ([]string, error) {
	img, err := s.store.Image(imageID)
	if err != nil {
		return nil, err
	}
	var layerIDs []string
	seen := map[string]struct{}{}
	for layerID := img.TopLayer; layerID != ""; {
		// The chain is data we did not write in this process; a loop in
		// it must not become one here.
		if _, ok := seen[layerID]; ok {
			return nil, fmt.Errorf("layer %q of image %q is its own ancestor", layerID, img.ID)
		}
		seen[layerID] = struct{}{}
		layer, err := s.store.Layer(layerID)
		if err != nil {
			return nil, fmt.Errorf("reading layer %q of image %q: %w", layerID, img.ID, err)
		}
		layerIDs = append(layerIDs, layer.ID)
		layerID = layer.Parent
	}
	slices.Reverse(layerIDs)
	return layerIDs, nil
}

// splitFDStreamBackend implements splitfdstreamserver.ImageBackend for one
// image.
type splitFDStreamBackend struct {
	store *store
	// layerIDs are the layers of the image, base layer first.
	layerIDs []string
}

// LayerIDByIndex implements splitfdstreamserver.ImageBackend.
func (b *splitFDStreamBackend) LayerIDByIndex(index int) (string, error) {
	if index < 0 || index >= len(b.layerIDs) {
		return "", fmt.Errorf("image has %d layers, no layer %d: %w", len(b.layerIDs), index, ErrLayerUnknown)
	}
	return b.layerIDs[index], nil
}

// OpenLayer implements splitfdstreamserver.ImageBackend.
func (b *splitFDStreamBackend) OpenLayer(layerID string) (splitfdstreamserver.Layer, error) {
	layerStores, err := b.store.allLayerStores()
	if err != nil {
		return nil, err
	}
	for _, store := range layerStores {
		if err := store.startReading(); err != nil {
			return nil, err
		}
		if !store.Exists(layerID) {
			store.stopReading()
			continue
		}
		// Everything is read with the layer store locked throughout, so
		// that a layer deleted concurrently is either fully available or
		// not found at all.
		layer, err := openSplitFDStreamLayer(store, layerID)
		if err != nil {
			store.stopReading()
			return nil, err
		}
		// The lock is released by closing the layer.
		return layer, nil
	}
	return nil, fmt.Errorf("locating layer %q: %w", layerID, ErrLayerUnknown)
}

// splitFDStreamLayer implements splitfdstreamserver.Layer.
type splitFDStreamLayer struct {
	// closeOnce guards the layer store lock: releasing it twice panics.
	closeOnce sync.Once
	store     roLayerStore
	files     internalDriver.LayerFiles
	tarSplit  io.ReadCloser
}

// openSplitFDStreamLayer opens layerID in store, which the caller has
// locked for reading.  On success, the result takes over that lock.
func openSplitFDStreamLayer(store roLayerStore, layerID string) (*splitFDStreamLayer, error) {
	tarSplit, err := store.tarSplitWithoutMounting(layerID)
	if err != nil {
		return nil, err
	}
	files, err := store.layerFilesWithoutMounting(layerID)
	if err != nil {
		return nil, errors.Join(err, tarSplit.Close())
	}
	return &splitFDStreamLayer{store: store, files: files, tarSplit: tarSplit}, nil
}

// Files implements splitfdstreamserver.Layer.
func (l *splitFDStreamLayer) Files() internalDriver.LayerFiles {
	return l.files
}

// TarSplit implements splitfdstreamserver.Layer.
func (l *splitFDStreamLayer) TarSplit() io.Reader {
	return l.tarSplit
}

// Close implements splitfdstreamserver.Layer.
func (l *splitFDStreamLayer) Close() error {
	var err error
	l.closeOnce.Do(func() {
		err = errors.Join(l.files.Close(), l.tarSplit.Close())
		l.store.stopReading()
	})
	return err
}
