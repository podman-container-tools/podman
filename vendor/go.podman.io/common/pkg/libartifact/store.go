//go:build !remote

package libartifact

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	libartTypes "go.podman.io/common/pkg/libartifact/types"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/pkg/blobinfocache/none"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage/pkg/fileutils"
	"go.podman.io/storage/pkg/lockfile"
)

const ManifestSchemaVersion = 2

type ArtifactStore struct {
	SystemContext *types.SystemContext
	storePath     string
	lock          *lockfile.LockFile
	eventChannel  chan *Event
}

// NewArtifactStore is a constructor for artifact stores.  Most artifact dealings depend on this. Store path is
// the filesystem location.
func NewArtifactStore(storePath string, sc *types.SystemContext) (*ArtifactStore, error) {
	if storePath == "" {
		return nil, errors.New("store path cannot be empty")
	}
	if !filepath.IsAbs(storePath) {
		return nil, fmt.Errorf("store path %q must be absolute", storePath)
	}

	logrus.Debugf("Using artifact store path: %s", storePath)

	artifactStore := &ArtifactStore{
		storePath:     storePath,
		SystemContext: sc,
	}

	// if the storage dir does not exist, we need to create it.
	baseDir := filepath.Dir(artifactStore.indexPath())
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, err
	}
	// Open the lockfile, creating if necessary
	lock, err := lockfile.GetLockFile(filepath.Join(storePath, "index.lock"))
	if err != nil {
		return nil, err
	}
	artifactStore.lock = lock
	// if the index file is not present we need to create an empty one
	// Do so after the lock to try and prevent races around store creation.
	if err := fileutils.Exists(artifactStore.indexPath()); err != nil && errors.Is(err, os.ErrNotExist) {
		if createErr := artifactStore.createEmptyManifest(); createErr != nil {
			return nil, createErr
		}
	}
	return artifactStore, nil
}

// EventChannel creates a buffered channel for events that the ArtifactStore will use
// to write events to. Callers are expected to read from the channel in a
// timely manner.
// Can be called once for a given ArtifactStore.
func (as *ArtifactStore) EventChannel() chan *Event {
	if as.eventChannel != nil {
		return as.eventChannel
	}
	as.eventChannel = make(chan *Event, 100)
	return as.eventChannel
}

// CloseEventChannel closes the event channel to signal listeners that
// the artifact store has stopped emitting events.
// WARNING: EventChannel and CloseEventChannel are not safe for concurrent use.
func (as *ArtifactStore) CloseEventChannel() {
	if as.eventChannel != nil {
		close(as.eventChannel)
		as.eventChannel = nil
	}
}

// lookupArtifactLocked looks up an artifact by fully qualified name,
// or name@digest, full ID, or partial ID.
// note: lookupArtifactLocked must be called while under a store lock
func (as *ArtifactStore) lookupArtifactLocked(ctx context.Context, asr ArtifactStoreReference) (*Artifact, error) {
	artifacts, err := as.getArtifacts(ctx, nil)
	if err != nil {
		return nil, err
	}

	if asr.ref != nil {
		lookupRef := *asr.ref
		digestedRef, isDigested := lookupRef.(reference.Digested)
		if isDigested {
			for _, a := range artifacts {
				if len(a.Name) == 0 {
					continue
				}
				storedRef, err := reference.ParseNamed(a.Name)
				if err != nil {
					logrus.Error(fmt.Errorf("error parsing %s in %s: %q", a.Name, as.storePath, err))
					continue
				}
				if storedRef.Name() == lookupRef.Name() {
					if a.Digest == digestedRef.Digest() {
						return a, nil
					}
				}
			}
		} else {
			lookupName := lookupRef.String()
			for _, a := range artifacts {
				if a.Name == lookupName {
					return a, nil
				}
			}
		}
		return nil, fmt.Errorf("%s: %w", lookupRef, libartTypes.ErrArtifactNotExist)
	}

	// It has been established that we need to preserve lookup by full or partial
	// ID like images or containers.  In this case, the "ID" is actually the digest
	if len(asr.possibleDigest) == 0 {
		return nil, errors.New("reference did not have name or id")
	}
	var returnArtifacts []*Artifact
	for _, a := range artifacts {
		if strings.HasPrefix(a.Digest.Encoded(), asr.possibleDigest) {
			returnArtifacts = append(returnArtifacts, a)
		}
	}
	// Now deal with the consequences of looking up by "ID"
	if len(returnArtifacts) == 0 {
		return nil, fmt.Errorf("%s: %w", asr.possibleDigest, libartTypes.ErrArtifactNotExist)
	}
	if len(returnArtifacts) > 1 {
		names := make([]string, 0, len(returnArtifacts))
		for _, a := range returnArtifacts {
			names = append(names, a.Name)
		}
		return nil, fmt.Errorf("multiple artifacts found with matching digest: %q", strings.Join(names, ","))
	}
	return returnArtifacts[0], nil
}

// Remove an artifact from the local artifact store.
func (as *ArtifactStore) Remove(ctx context.Context, asr ArtifactStoreReference) (_ *digest.Digest, removeErr error) {
	as.lock.Lock()
	defer as.lock.Unlock()

	arty, err := as.lookupArtifactLocked(ctx, asr)
	if err != nil {
		return nil, err
	}

	ir, err := layout.NewReference(as.storePath, arty.Name)
	if err != nil {
		return nil, err
	}
	if err := ir.DeleteImage(ctx, as.SystemContext); err != nil {
		return nil, err
	}

	if as.eventChannel != nil {
		as.writeEvent(&Event{ID: arty.Digest.String(), Name: arty.Name, Time: time.Now(), Type: EventTypeArtifactRemove})
	}
	return &arty.Digest, nil
}

// Inspect an artifact in a local store.
func (as *ArtifactStore) Inspect(ctx context.Context, asr ArtifactStoreReference) (*Artifact, error) {
	as.lock.RLock()
	defer as.lock.Unlock()

	return as.lookupArtifactLocked(ctx, asr)
}

// List artifacts in the local store.
func (as *ArtifactStore) List(ctx context.Context) (ArtifactList, error) {
	as.lock.RLock()
	defer as.lock.Unlock()

	return as.getArtifacts(ctx, nil)
}

// Execute fn while holding the store lock and providing a reference to the local layout.
func (as *ArtifactStore) withLockedLayout(localName string, fn func(localRef types.ImageReference) (digest.Digest, error)) (digest.Digest, error) {
	as.lock.Lock()
	defer as.lock.Unlock()

	ref, err := layout.NewReference(as.storePath, localName)
	if err != nil {
		return "", err
	}

	return fn(ref)
}

// Copy artifact from srcRef to destRef and return the digest of the copied artifact.
func (as *ArtifactStore) copyArtifact(ctx context.Context, srcRef types.ImageReference, destRef types.ImageReference, opts libimage.CopyOptions) (_ digest.Digest, err error) {
	copyer, err := libimage.NewCopier(&opts, as.SystemContext)
	if err != nil {
		return "", err
	}
	defer func() {
		closeErr := copyer.Close()
		if err == nil {
			err = closeErr
		}
	}()
	artifactBytes, err := copyer.Copy(ctx, srcRef, destRef)
	if err != nil {
		return "", err
	}
	return digest.FromBytes(artifactBytes), nil
}

// Pull an artifact from an image registry to a local store.
func (as *ArtifactStore) Pull(ctx context.Context, ref ArtifactReference, opts libimage.CopyOptions) (_ digest.Digest, pullErr error) {
	srcRef, err := docker.NewReference(ref.ref)
	if err != nil {
		return "", err
	}
	artifactDigest, err := as.withLockedLayout(ref.String(), func(localRef types.ImageReference) (digest.Digest, error) {
		return as.copyArtifact(ctx, srcRef, localRef, opts)
	})
	if err != nil {
		return "", err
	}
	if as.eventChannel != nil {
		as.writeEvent(&Event{ID: artifactDigest.String(), Name: ref.String(), Time: time.Now(), Type: EventTypeArtifactPull})
	}
	return artifactDigest, nil
}

// Push an artifact to an image registry.
func (as *ArtifactStore) Push(ctx context.Context, src, dest ArtifactReference, opts libimage.CopyOptions) (_ digest.Digest, pushErr error) {
	destRef, err := docker.NewReference(dest.ref)
	if err != nil {
		return "", err
	}
	artifactDigest, err := as.withLockedLayout(src.String(), func(localRef types.ImageReference) (digest.Digest, error) {
		return as.copyArtifact(ctx, localRef, destRef, opts)
	})
	if err != nil {
		return "", err
	}
	if as.eventChannel != nil {
		as.writeEvent(&Event{ID: artifactDigest.String(), Name: dest.String(), Time: time.Now(), Type: EventTypeArtifactPush})
	}
	return artifactDigest, nil
}

// createNewArtifactManifest creates a new manifest with annotations.
func createNewArtifactManifest(options *libartTypes.AddOptions) specV1.Manifest {
	// Set creation timestamp and other annotations
	annotations := make(map[string]string)
	if options.Annotations != nil {
		annotations = maps.Clone(options.Annotations)
	}
	annotations[specV1.AnnotationCreated] = time.Now().UTC().Format(time.RFC3339Nano)

	return specV1.Manifest{
		Versioned:    specs.Versioned{SchemaVersion: ManifestSchemaVersion},
		MediaType:    specV1.MediaTypeImageManifest,
		ArtifactType: options.ArtifactMIMEType, // TODO This should probably be configurable once the CLI is capable
		Config:       specV1.DescriptorEmptyJSON,
		Layers:       make([]specV1.Descriptor, 0),
		Annotations:  annotations,
	}
}

// cleanupAfterAppend removes previous image when doing an append.
func cleanupAfterAppend(ctx context.Context, oldDigest digest.Digest, as *ArtifactStore) error {
	lrs, err := layout.List(as.storePath)
	if err != nil {
		return err
	}

	for _, l := range lrs {
		if oldDigest.String() == l.ManifestDescriptor.Digest.String() {
			if _, ok := l.ManifestDescriptor.Annotations[specV1.AnnotationRefName]; ok {
				continue
			}

			if err := l.Reference.DeleteImage(ctx, as.SystemContext); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// Add takes one or more artifact blobs and add them to the local artifact store.  The empty
// string input is for possible custom artifact types.
func (as *ArtifactStore) Add(ctx context.Context, dest ArtifactReference, artifactBlobs []libartTypes.ArtifactBlob, options *libartTypes.AddOptions) (_ *digest.Digest, addErr error) {
	if options.Append && len(options.ArtifactMIMEType) > 0 {
		return nil, errors.New("append option is not compatible with type option")
	}
	if options.Append && options.Replace {
		return nil, errors.New("append and replace options are mutually exclusive")
	}

	locked := true
	as.lock.Lock()
	defer func() {
		if locked {
			as.lock.Unlock()
		}
	}()

	var artifactManifest specV1.Manifest
	var oldDigest digest.Digest
	fileNames := map[string]struct{}{}

	existingArtifact, lookupErr := as.lookupArtifactLocked(ctx, dest.ToArtifactStoreReference())

	switch {
	case options.Append:
		// Append to existing artifact
		if lookupErr != nil {
			return nil, lookupErr
		}
		artifactManifest = existingArtifact.Manifest.Manifest
		oldDigest = existingArtifact.Digest
		for _, layer := range artifactManifest.Layers {
			if value, ok := layer.Annotations[specV1.AnnotationTitle]; ok && value != "" {
				fileNames[value] = struct{}{}
			}
		}
	case options.Replace:
		// Replace existing artifact - delete old one if it exists, then create new
		if lookupErr == nil {
			ir, err := layout.NewReference(as.storePath, dest.String())
			if err != nil {
				return nil, err
			}
			if err := ir.DeleteImage(ctx, as.SystemContext); err != nil {
				return nil, err
			}
		}

		// Create new manifest
		artifactManifest = createNewArtifactManifest(options)
	default:
		// Add new artifact - error if it already exists
		if lookupErr == nil {
			return nil, fmt.Errorf("%s: %w", dest.String(), libartTypes.ErrArtifactAlreadyExists)
		}
		artifactManifest = createNewArtifactManifest(options)
	}

	for _, artifact := range artifactBlobs {
		fileName := artifact.FileName
		if _, ok := fileNames[fileName]; ok {
			return nil, fmt.Errorf("%s: %w", fileName, libartTypes.ErrArtifactFileExists)
		}
		fileNames[fileName] = struct{}{}
	}

	ir, err := layout.NewReference(as.storePath, dest.String())
	if err != nil {
		return nil, err
	}

	imageDest, err := ir.NewImageDestination(ctx, as.SystemContext)
	if err != nil {
		return nil, err
	}
	defer imageDest.Close()

	// Unlock around the actual pull of the blobs.
	// This is ugly as hell, but should be safe.
	locked = false
	as.lock.Unlock()

	// ImageDestination, in general, requires the caller to write a full image; here we may write only the added layers.
	// This works for the oci/layout transport we hard-code.
	for _, artifactBlob := range artifactBlobs {
		if (artifactBlob.BlobFilePath == "" && artifactBlob.BlobReader == nil) || (artifactBlob.BlobFilePath != "" && artifactBlob.BlobReader != nil) {
			return nil, errors.New("Artifact.BlobFile or Artifact.BlobReader must be provided")
		}

		annotations := make(map[string]string)
		if options.Annotations != nil {
			annotations = maps.Clone(options.Annotations)
		}
		if title, ok := annotations[specV1.AnnotationTitle]; ok {
			// Verify a duplicate AnnotationTitle is not in use in a different layer.
			for _, layer := range artifactManifest.Layers {
				if title == layer.Annotations[specV1.AnnotationTitle] {
					return nil, fmt.Errorf("duplicate layers %s labels within an artifact not allowed", specV1.AnnotationTitle)
				}
			}
		} else {
			// Only override if the user did not specify the Title
			annotations[specV1.AnnotationTitle] = artifactBlob.FileName
		}

		newLayer := specV1.Descriptor{
			MediaType:   options.FileMIMEType,
			Annotations: annotations,
		}

		// If we did not receive an override for the layer's mediatype, use
		// detection to determine it.
		if options.FileMIMEType == "" {
			artifactBlob.BlobReader, newLayer.MediaType, err = determineBlobMIMEType(artifactBlob)
			if err != nil {
				return nil, err
			}
		}

		// get the new artifact into the local store
		if artifactBlob.BlobFilePath != "" {
			newBlobDigest, newBlobSize, err := layout.PutBlobFromLocalFile(ctx, imageDest, artifactBlob.BlobFilePath)
			if err != nil {
				return nil, err
			}
			newLayer.Digest = newBlobDigest
			newLayer.Size = newBlobSize
		} else {
			blobInfo, err := imageDest.PutBlob(ctx, artifactBlob.BlobReader, types.BlobInfo{Size: -1}, none.NoCache, false)
			if err != nil {
				return nil, err
			}
			newLayer.Digest = blobInfo.Digest
			newLayer.Size = blobInfo.Size
		}

		artifactManifest.Layers = append(artifactManifest.Layers, newLayer)
	}

	as.lock.Lock()
	locked = true

	rawData, err := json.Marshal(artifactManifest)
	if err != nil {
		return nil, err
	}
	if err := imageDest.PutManifest(ctx, rawData, nil); err != nil {
		return nil, err
	}

	unparsed := newUnparsedArtifactImage(ir, artifactManifest)
	if err := imageDest.Commit(ctx, unparsed); err != nil {
		return nil, err
	}

	artifactManifestDigest := digest.FromBytes(rawData)

	// the config is an empty JSON stanza i.e. '{}'; if it does not yet exist, it needs
	// to be created
	if err := createEmptyStanza(filepath.Join(as.storePath, specV1.ImageBlobsDir, artifactManifestDigest.Algorithm().String(), artifactManifest.Config.Digest.Encoded())); err != nil {
		logrus.Errorf("failed to check or write empty stanza file: %v", err)
	}

	// Clean up after append. Remove previous artifact from store.
	if oldDigest != "" {
		if err := cleanupAfterAppend(ctx, oldDigest, as); err != nil {
			return nil, err
		}
	}

	if as.eventChannel != nil {
		as.writeEvent(&Event{ID: artifactManifestDigest.String(), Name: dest.String(), Time: time.Now(), Type: EventTypeArtifactAdd})
	}
	return &artifactManifestDigest, nil
}

func getArtifactAndImageSource(ctx context.Context, as *ArtifactStore, asr ArtifactStoreReference, options *libartTypes.FilterBlobOptions) (*Artifact, types.ImageSource, error) {
	if len(options.Digest) > 0 && len(options.Title) > 0 {
		return nil, nil, errors.New("cannot specify both digest and title")
	}
	arty, err := as.lookupArtifactLocked(ctx, asr)
	if err != nil {
		return nil, nil, err
	}
	if len(arty.Manifest.Layers) == 0 {
		return nil, nil, errors.New("the artifact has no blobs, nothing to extract")
	}

	ir, err := layout.NewReference(as.storePath, arty.Name)
	if err != nil {
		return nil, nil, err
	}
	imgSrc, err := ir.NewImageSource(ctx, as.SystemContext)
	return arty, imgSrc, err
}

// BlobMountPaths allows the caller to access the file names from the store and how they should be mounted.
func (as *ArtifactStore) BlobMountPaths(ctx context.Context, asr ArtifactStoreReference, options *libartTypes.BlobMountPathOptions) ([]libartTypes.BlobMountPath, error) {
	// FIX ME
	// LOCKING BUG: getArtifactAndImageSource assumes a locked ArtifactStore
	arty, imgSrc, err := getArtifactAndImageSource(ctx, as, asr, &options.FilterBlobOptions)
	if err != nil {
		return nil, err
	}
	defer imgSrc.Close()

	if len(options.Digest) > 0 || len(options.Title) > 0 {
		digest, err := findDigest(arty, &options.FilterBlobOptions)
		if err != nil {
			return nil, err
		}
		// In case the digest is set we always use it as target name
		// so we do not have to get the actual title annotation form the blob.
		// Passing options.Title is enough because we know it is empty when digest
		// is set as we only allow either one.
		filename, err := generateArtifactBlobName(options.Title, digest)
		if err != nil {
			return nil, err
		}

		path, err := layout.GetLocalBlobPath(ctx, imgSrc, digest)
		if err != nil {
			return nil, err
		}
		return []libartTypes.BlobMountPath{{
			SourcePath: path,
			Name:       filename,
		}}, nil
	}

	mountPaths := make([]libartTypes.BlobMountPath, 0, len(arty.Manifest.Layers))
	for _, l := range arty.Manifest.Layers {
		title := l.Annotations[specV1.AnnotationTitle]
		for _, mp := range mountPaths {
			if title == mp.Name {
				return nil, fmt.Errorf("annotation %q:%q is used in multiple different layers within artifact", specV1.AnnotationTitle, title)
			}
		}
		filename, err := generateArtifactBlobName(title, l.Digest)
		if err != nil {
			return nil, err
		}

		path, err := layout.GetLocalBlobPath(ctx, imgSrc, l.Digest)
		if err != nil {
			return nil, err
		}
		mountPaths = append(mountPaths, libartTypes.BlobMountPath{
			SourcePath: path,
			Name:       filename,
		})
	}
	return mountPaths, nil
}

// Extract an artifact to local file or directory.
func (as *ArtifactStore) Extract(ctx context.Context, nameOrDigest ArtifactStoreReference, target string, options *libartTypes.ExtractOptions) error {
	// FIX ME
	// LOCKING BUG: getArtifactAndImageSource assumes a locked ArtifactStore
	arty, imgSrc, err := getArtifactAndImageSource(ctx, as, nameOrDigest, &options.FilterBlobOptions)
	if err != nil {
		return err
	}
	defer imgSrc.Close()

	// check if dest is a dir to know if we can copy more than one blob
	destIsFile := true
	stat, err := os.Stat(target)
	if err == nil {
		destIsFile = !stat.IsDir()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if destIsFile {
		var digest digest.Digest
		if len(arty.Manifest.Layers) > 1 {
			if len(options.Digest) == 0 && len(options.Title) == 0 {
				return fmt.Errorf("the artifact consists of several blobs and the target %q is not a directory and neither digest or title was specified to only copy a single blob", target)
			}
			digest, err = findDigest(arty, &options.FilterBlobOptions)
			if err != nil {
				return err
			}
		} else {
			digest = arty.Manifest.Layers[0].Digest
		}

		return copyTrustedImageBlobToFile(ctx, imgSrc, digest, target)
	}

	if len(options.Digest) > 0 || len(options.Title) > 0 {
		digest, err := findDigest(arty, &options.FilterBlobOptions)
		if err != nil {
			return err
		}
		// In case the digest is set we always use it as target name
		// so we do not have to get the actual title annotation form the blob.
		// Passing options.Title is enough because we know it is empty when digest
		// is set as we only allow either one.
		filename, err := generateArtifactBlobName(options.Title, digest)
		if err != nil {
			return err
		}

		return copyTrustedImageBlobToFile(ctx, imgSrc, digest, filepath.Join(target, filename))
	}

	for _, l := range arty.Manifest.Layers {
		title := l.Annotations[specV1.AnnotationTitle]
		filename, err := generateArtifactBlobName(title, l.Digest)
		if err != nil {
			return err
		}

		err = copyTrustedImageBlobToFile(ctx, imgSrc, l.Digest, filepath.Join(target, filename))
		if err != nil {
			return err
		}
	}

	return nil
}

// Extract an artifact to tar stream.
func (as *ArtifactStore) ExtractTarStream(ctx context.Context, w io.Writer, asr ArtifactStoreReference, options *libartTypes.ExtractOptions) error {
	if options == nil {
		options = &libartTypes.ExtractOptions{}
	}

	// FIX ME
	// LOCKING BUG: getArtifactAndImageSource assumes a locked ArtifactStore
	arty, imgSrc, err := getArtifactAndImageSource(ctx, as, asr, &options.FilterBlobOptions)
	if err != nil {
		return err
	}
	defer imgSrc.Close()

	// Return early if only a single blob is requested via title or digest
	if len(options.Digest) > 0 || len(options.Title) > 0 {
		digest, err := findDigest(arty, &options.FilterBlobOptions)
		if err != nil {
			return err
		}

		// In case the digest is set we always use it as target name
		// so we do not have to get the actual title annotation form the blob.
		// Passing options.Title is enough because we know it is empty when digest
		// is set as we only allow either one.
		var filename string
		if !options.ExcludeTitle {
			filename, err = generateArtifactBlobName(options.Title, digest)
			if err != nil {
				return err
			}
		}

		tw := tar.NewWriter(w)
		defer tw.Close()

		err = copyTrustedImageBlobToTarStream(ctx, imgSrc, digest, filename, tw)
		if err != nil {
			return err
		}

		return nil
	}

	artifactBlobCount := len(arty.Manifest.Layers)

	type blob struct {
		name   string
		digest digest.Digest
	}
	blobs := make([]blob, 0, artifactBlobCount)

	// Gather blob details and return error on any illegal names
	for _, l := range arty.Manifest.Layers {
		title := l.Annotations[specV1.AnnotationTitle]
		digest := l.Digest
		var name string

		if artifactBlobCount != 1 || !options.ExcludeTitle {
			name, err = generateArtifactBlobName(title, digest)
			if err != nil {
				return err
			}
		}

		blobs = append(blobs, blob{
			name:   name,
			digest: digest,
		})
	}

	// Wrap io.Writer in a tar.Writer
	tw := tar.NewWriter(w)
	defer tw.Close()

	// Write each blob to tar.Writer then close
	for _, b := range blobs {
		err := copyTrustedImageBlobToTarStream(ctx, imgSrc, b.digest, b.name, tw)
		if err != nil {
			return err
		}
	}

	return nil
}

func generateArtifactBlobName(title string, digest digest.Digest) (string, error) {
	filename := title
	if len(filename) == 0 {
		// No filename given, use the digest. But because ":" is not a valid path char
		// on all platforms replace it with "-".
		filename = strings.ReplaceAll(digest.String(), ":", "-")
	}

	// Important: A potentially malicious artifact could contain a title name with "/"
	// and could try via relative paths such as "../" try to overwrite files on the host
	// the user did not intend. As there is no use for directories in this path we
	// disallow all of them and not try to "make it safe" via securejoin or others.
	// We must use os.IsPathSeparator() as on Windows it checks both "\\" and "/".
	for i := range len(filename) {
		if os.IsPathSeparator(filename[i]) {
			return "", fmt.Errorf("invalid name: %q cannot contain %c: %w", filename, filename[i], libartTypes.ErrArtifactBlobTitleInvalid)
		}
	}
	return filename, nil
}

func findDigest(arty *Artifact, options *libartTypes.FilterBlobOptions) (digest.Digest, error) {
	var digest digest.Digest
	for _, l := range arty.Manifest.Layers {
		if options.Digest == l.Digest.String() {
			if len(digest.String()) > 0 {
				return digest, fmt.Errorf("more than one match for the digest %q", options.Digest)
			}
			digest = l.Digest
		}
		if len(options.Title) > 0 {
			if val, ok := l.Annotations[specV1.AnnotationTitle]; ok &&
				val == options.Title {
				if len(digest.String()) > 0 {
					return digest, fmt.Errorf("more than one match for the title %q", options.Title)
				}
				digest = l.Digest
			}
		}
	}
	if len(digest.String()) == 0 {
		if len(options.Title) > 0 {
			return digest, fmt.Errorf("no blob with the title %q", options.Title)
		}
		return digest, fmt.Errorf("no blob with the digest %q", options.Digest)
	}
	return digest, nil
}

// copyTrustedImageBlobToFile copies blob identified by digest in imgSrc to file target.
//
// WARNING: This does not validate the contents against the expected digest, so it should only
// be used to read from trusted sources!
func copyTrustedImageBlobToFile(ctx context.Context, imgSrc types.ImageSource, digest digest.Digest, target string) error {
	src, _, err := imgSrc.GetBlob(ctx, types.BlobInfo{Digest: digest}, nil)
	if err != nil {
		return fmt.Errorf("failed to get artifact file: %w", err)
	}
	defer src.Close()
	dest, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer dest.Close()

	// By default the c/image oci layout API for GetBlob() should always return a os.File in our usage here.
	// And since it is a file we can try to reflink it. In case it is not we should default to the normal copy.
	if file, ok := src.(*os.File); ok {
		return fileutils.ReflinkOrCopy(file, dest)
	}

	_, err = io.Copy(dest, src)
	return err
}

// copyTrustedImageBlobToStream copies blob identified by digest in imgSrc to io.writer target.
//
// WARNING: This does not validate the contents against the expected digest, so it should only
// be used to read from trusted sources!
func copyTrustedImageBlobToTarStream(ctx context.Context, imgSrc types.ImageSource, digest digest.Digest, filename string, tw *tar.Writer) error {
	src, srcSize, err := imgSrc.GetBlob(ctx, types.BlobInfo{Digest: digest}, nil)
	if err != nil {
		return fmt.Errorf("failed to get artifact blob: %w", err)
	}
	defer src.Close()

	if srcSize == -1 {
		return errors.New("internal error: oci layout image is missing blob size")
	}

	// Note: We can't assume imgSrc will return an *os.File so we must generate the tar header
	now := time.Now()
	header := tar.Header{
		Name:       filename,
		Mode:       0o600,
		Size:       srcSize,
		ModTime:    now,
		ChangeTime: now,
		AccessTime: now,
	}

	if err := tw.WriteHeader(&header); err != nil {
		return fmt.Errorf("error writing tar header for %s: %w", filename, err)
	}

	// Copy the file content to the tar archive.
	_, err = io.Copy(tw, src)
	if err != nil {
		return fmt.Errorf("error copying content of %s to tar archive: %w", filename, err)
	}

	return nil
}

func (as *ArtifactStore) createEmptyManifest() error {
	as.lock.Lock()
	defer as.lock.Unlock()
	index := specV1.Index{
		MediaType: specV1.MediaTypeImageIndex,
		Versioned: specs.Versioned{SchemaVersion: ManifestSchemaVersion},
	}
	rawData, err := json.Marshal(&index)
	if err != nil {
		return err
	}

	return os.WriteFile(as.indexPath(), rawData, 0o644)
}

func (as *ArtifactStore) indexPath() string {
	return filepath.Join(as.storePath, specV1.ImageIndexFile)
}

// getArtifacts returns an ArtifactList based on the artifact's store.  The return error and
// unused opts is meant for future growth like filters, etc so the API does not change.
func (as *ArtifactStore) getArtifacts(ctx context.Context, _ *libartTypes.GetArtifactOptions) (ArtifactList, error) {
	var al ArtifactList

	lrs, err := layout.List(as.storePath)
	if err != nil {
		return nil, err
	}
	for _, l := range lrs {
		imgSrc, err := l.Reference.NewImageSource(ctx, as.SystemContext)
		if err != nil {
			return nil, err
		}
		artManifest, b, err := getManifest(ctx, imgSrc)
		imgSrc.Close()
		if err != nil {
			return nil, err
		}
		artifact := Artifact{
			Digest:      digest.FromBytes(b),
			Manifest:    artManifest,
			rawManifest: b,
		}
		if val, ok := l.ManifestDescriptor.Annotations[specV1.AnnotationRefName]; ok {
			artifact.SetName(val)
		}

		al = append(al, &artifact)
	}
	return al, nil
}

// getManifest takes an imgSrc and returns the manifest for the imgSrc and
// the blob as it was originally read. A OCI index list is not supported and
// will return an error.
func getManifest(ctx context.Context, imgSrc types.ImageSource) (*manifest.OCI1, []byte, error) {
	b, manifestType, err := image.UnparsedInstance(imgSrc, nil).Manifest(ctx)
	if err != nil {
		return nil, nil, err
	}

	// We only support a single flat manifest and not an oci index list
	if manifest.MIMETypeIsMultiImage(manifestType) {
		return nil, nil, fmt.Errorf("manifest %q is index list", imgSrc.Reference().StringWithinTransport())
	}

	// parse the single manifest
	mani, err := manifest.OCI1FromManifest(b)
	if err != nil {
		return nil, nil, err
	}
	return mani, b, nil
}

func createEmptyStanza(path string) error {
	if err := fileutils.Exists(path); err == nil {
		return nil
	}
	return os.WriteFile(path, specV1.DescriptorEmptyJSON.Data, 0o644)
}

// determineBlobMIMEType reads up to 512 bytes into a buffer
// without advancing the read position of the io.Reader.
// If http.DetectContentType is unable to determine a valid
// MIME type, the default of "application/octet-stream" will be
// returned.
// Either an io.Reader or *os.File can be provided, if an io.Reader
// is provided, a new io.Reader will be returned to be used for
// subsequent reads.
func determineBlobMIMEType(ab libartTypes.ArtifactBlob) (io.Reader, string, error) {
	if ab.BlobFilePath == "" && ab.BlobReader == nil || ab.BlobFilePath != "" && ab.BlobReader != nil {
		return nil, "", errors.New("Artifact.BlobFile or Artifact.BlobReader must be provided")
	}

	var (
		err        error
		mimeBuf    []byte
		peekBuffer *bufio.Reader
	)

	maxBytes := 512

	if ab.BlobFilePath != "" {
		f, err := os.Open(ab.BlobFilePath)
		if err != nil {
			return nil, "", err
		}
		defer f.Close()

		buf := make([]byte, maxBytes)

		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			return nil, "", err
		}

		mimeBuf = buf[:n]
	}

	if ab.BlobReader != nil {
		peekBuffer = bufio.NewReader(ab.BlobReader)

		mimeBuf, err = peekBuffer.Peek(maxBytes)
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) && !errors.Is(err, io.EOF) {
			return nil, "", err
		}
	}

	return peekBuffer, http.DetectContentType(mimeBuf), nil
}
