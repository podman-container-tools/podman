package layout

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/docker/go-connections/tlsconfig"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"go.podman.io/image/v5/internal/imagesource/impl"
	"go.podman.io/image/v5/internal/imagesource/stubs"
	"go.podman.io/image/v5/internal/manifest"
	"go.podman.io/image/v5/internal/private"
	"go.podman.io/image/v5/pkg/tlsclientconfig"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage/pkg/fileutils"
)

// ImageNotFoundError is used when the OCI structure, in principle, exists and seems valid enough,
// but nothing matches the “image” part of the provided reference.
type ImageNotFoundError struct {
	ref ociReference
	// We may make members public, or add methods, in the future.
}

func (e ImageNotFoundError) Error() string {
	return fmt.Sprintf("no descriptor found for reference %q", e.ref.image)
}

type ociImageSource struct {
	impl.Compat
	impl.PropertyMethodsInitialize
	impl.NoSignatures
	impl.DoesNotAffectLayerInfosForCopy
	stubs.NoGetBlobAtInitialize

	ref              ociReference
	fs               fs.FS // For accessing the OCI structure
	blobFS           fs.FS // May be equal to fs, or point to OCISharedBlobDirPath
	useSharedBlobDir bool
	blobFSLocalPath  string // The base of blobFS IF it is safe to access using ordinary filepath.Join().
	index            *imgspecv1.Index
	descriptor       imgspecv1.Descriptor
	client           *http.Client
}

// newImageSource returns an ImageSource for reading from an existing directory.
func newImageSource(sys *types.SystemContext, ref ociReference) (private.ImageSource, error) {
	tr := tlsclientconfig.NewTransport()
	if sys != nil && sys.BaseTLSConfig != nil {
		tr.TLSClientConfig = sys.BaseTLSConfig.Clone()
	} else {
		tr.TLSClientConfig = &tls.Config{
			// As of 2025-08, tlsconfig.ClientDefault() differs from Go 1.23 defaults only in CipherSuites;
			// so, limit us to only using that value. If go-connections/tlsconfig changes its policy, we
			// will want to consider that and make a decision whether to follow suit.
			// There is some chance that eventually the Go default will be to require TLS 1.3, and that point
			// we might want to drop the dependency on go-connections entirely.
			CipherSuites: tlsconfig.ClientDefault().CipherSuites,
		}
	}

	if sys != nil && sys.OCICertPath != "" {
		if err := tlsclientconfig.SetupCertificates(sys.OCICertPath, tr.TLSClientConfig); err != nil {
			return nil, err
		}
		tr.TLSClientConfig.InsecureSkipVerify = sys.OCIInsecureSkipTLSVerify
	}
	client := &http.Client{}
	client.Transport = tr

	s := &ociImageSource{
		PropertyMethodsInitialize: impl.PropertyMethods(impl.Properties{
			HasThreadSafeGetBlob: false,
		}),
		NoGetBlobAtInitialize: stubs.NoGetBlobAt(ref),

		ref:    ref,
		client: client,
	}
	if ref.reader != nil {
		s.fs = ref.reader.root.FS()
	} else {
		s.fs = os.DirFS(ref.dir)
	}
	if sys != nil && sys.OCISharedBlobDirPath != "" {
		// TODO(jonboulle): check dir existence?
		s.blobFS = os.DirFS(sys.OCISharedBlobDirPath)
		s.useSharedBlobDir = true
		s.blobFSLocalPath = sys.OCISharedBlobDirPath
	} else {
		s.blobFS = s.fs
		if ref.reader == nil {
			s.blobFSLocalPath = s.ref.dir
		}
	}
	index, err := srcGetIndex(s.fs)
	if err != nil {
		return nil, err
	}
	s.index = index
	s.descriptor, _, err = ref.getManifestDescriptor(index)
	if err != nil {
		return nil, err
	}

	s.Compat = impl.AddCompat(s)
	return s, nil
}

// Reference returns the reference used to set up this source.
func (s *ociImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *ociImageSource) Close() error {
	s.client.CloseIdleConnections()
	return nil
}

// srcGetIndex reads an index within the OCI layout used in ref.
func srcGetIndex(ociFS fs.FS) (*imgspecv1.Index, error) {
	content, err := ociFS.Open(indexFSPath())
	if err != nil {
		return nil, err
	}
	defer content.Close()

	var index imgspecv1.Index
	if err := json.NewDecoder(content).Decode(&index); err != nil {
		return nil, err
	}
	return &index, nil
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
func (s *ociImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	var dig digest.Digest
	var mimeType string
	var err error

	if instanceDigest == nil {
		dig = s.descriptor.Digest
		mimeType = s.descriptor.MediaType
	} else {
		dig = *instanceDigest
		for _, md := range s.index.Manifests {
			if md.Digest == dig {
				mimeType = md.MediaType
				break
			}
		}
	}

	manifestFSPath, err := blobFSPath(dig, s.useSharedBlobDir)
	if err != nil {
		return nil, "", err
	}
	m, err := fs.ReadFile(s.blobFS, manifestFSPath)
	if err != nil {
		return nil, "", err
	}

	if mimeType == "" {
		mimeType = manifest.GuessMIMEType(m)
	}

	return m, mimeType, nil
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *ociImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	if len(info.URLs) != 0 {
		r, s, err := s.getExternalBlob(ctx, info.URLs)
		if err != nil {
			return nil, 0, err
		} else if r != nil {
			return r, s, nil
		}
	}

	blobFSPath, err := blobFSPath(info.Digest, s.useSharedBlobDir)
	if err != nil {
		return nil, 0, err
	}
	r, err := s.blobFS.Open(blobFSPath)
	if err != nil {
		return nil, 0, err
	}
	fi, err := r.Stat()
	if err != nil {
		return nil, 0, err
	}
	return r, fi.Size(), nil
}

// getExternalBlob returns the reader of the first available blob URL from urls, which must not be empty.
// This function can return nil reader when no url is supported by this function. In this case, the caller
// should fallback to fetch the non-external blob (i.e. pull from the registry).
func (s *ociImageSource) getExternalBlob(ctx context.Context, urls []string) (io.ReadCloser, int64, error) {
	if len(urls) == 0 {
		return nil, 0, errors.New("internal error: getExternalBlob called with no URLs")
	}

	errWrap := errors.New("failed fetching external blob from all urls")
	hasSupportedURL := false
	for _, u := range urls {
		if u, err := url.Parse(u); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue // unsupported url. skip this url.
		}
		hasSupportedURL = true
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			errWrap = fmt.Errorf("fetching %q failed %s: %w", u, err.Error(), errWrap)
			continue
		}

		resp, err := s.client.Do(req)
		if err != nil {
			errWrap = fmt.Errorf("fetching %q failed %s: %w", u, err.Error(), errWrap)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			errWrap = fmt.Errorf("fetching %q failed, response code not 200: %w", u, errWrap)
			continue
		}

		return resp.Body, getBlobSize(resp), nil
	}
	if !hasSupportedURL {
		return nil, 0, nil // fallback to non-external blob
	}

	return nil, 0, errWrap
}

func getBlobSize(resp *http.Response) int64 {
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	return size
}

// GetLocalBlobPath returns the local path to the blob file with the given digest.
// The returned path is checked for existence so when a non existing digest is
// given an error will be returned.
//
// Important: The returned path must be treated as read only, writing the file will
// corrupt the oci layout as the digest no longer matches.
func GetLocalBlobPath(ctx context.Context, src types.ImageSource, digest digest.Digest) (string, error) {
	s, ok := src.(*ociImageSource)
	if !ok {
		return "", errors.New("caller error: GetLocalBlobPath called with a non-oci: source")
	}

	if s.blobFSLocalPath == "" {
		return "", errors.New("GetLocalBlobPath is not supported in root-restricted configurations")
	}

	fsPath, err := blobFSPath(digest, s.useSharedBlobDir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(s.blobFSLocalPath, filepath.FromSlash(fsPath))
	if err := fileutils.Exists(path); err != nil {
		return "", err
	}

	return path, nil
}

// LoadManifestDescriptor loads the manifest descriptor to be used to retrieve the image name
// when pulling an image
func LoadManifestDescriptor(imgRef types.ImageReference) (imgspecv1.Descriptor, error) {
	ociRef, ok := imgRef.(ociReference)
	if !ok {
		return imgspecv1.Descriptor{}, errors.New("error typecasting, need type ociRef")
	}

	var ociFS fs.FS
	if ociRef.reader != nil {
		ociFS = ociRef.reader.root.FS()
	} else {
		ociFS = os.DirFS(ociRef.dir)
	}

	index, err := srcGetIndex(ociFS)
	if err != nil {
		return imgspecv1.Descriptor{}, err
	}
	md, _, err := ociRef.getManifestDescriptor(index)
	return md, err
}

// indexFSPath returns a path for the index.json within a directory using OCI conventions,
// satisfying fs.ValidPath.
func indexFSPath() string {
	return imgspecv1.ImageIndexFile
}

// blobFSPath returns a path for a blob within a directory using OCI conventions,
// or within sharedBlobDir, depending on useSharedBlobDir.
func blobFSPath(digest digest.Digest, useSharedBlobDir bool) (string, error) {
	if err := digest.Validate(); err != nil {
		return "", fmt.Errorf("unexpected digest reference %s: %w", digest, err)
	}
	if useSharedBlobDir {
		return path.Join(digest.Algorithm().String(), digest.Encoded()), nil
	} else {
		return path.Join(imgspecv1.ImageBlobsDir, digest.Algorithm().String(), digest.Encoded()), nil
	}
}
