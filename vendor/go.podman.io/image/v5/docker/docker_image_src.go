package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution/registry/api/errcode"
	v2 "github.com/docker/distribution/registry/api/v2"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/internal/imagesource/impl"
	"go.podman.io/image/v5/internal/imagesource/stubs"
	"go.podman.io/image/v5/internal/iolimits"
	"go.podman.io/image/v5/internal/private"
	"go.podman.io/image/v5/internal/signature"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/pkg/blobinfocache/none"
	"go.podman.io/image/v5/pkg/sysregistriesv2"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage/pkg/regexp"
)

// maxLookasideSignatures is an arbitrary limit for the total number of signatures we would try to read from a lookaside server,
// even if it were broken or malicious and it continued serving an enormous number of items.
const maxLookasideSignatures = 128

type dockerImageSource struct {
	impl.Compat
	impl.PropertyMethodsInitialize
	impl.DoesNotAffectLayerInfosForCopy
	stubs.ImplementsGetBlobAt

	logicalRef  dockerReference // The reference the user requested. This must satisfy !isUnknownDigest
	physicalRef dockerReference // The actual reference we are accessing (possibly a mirror). This must satisfy !isUnknownDigest
	c           *dockerClient
	// State
	cachedManifest         []byte // nil if not loaded yet
	cachedManifestMIMEType string // Only valid if cachedManifest != nil

	// Mirror fallback: when a blob fetch fails with a fallback-worthy error,
	// try remaining pull sources before giving up. Protected by mirrorMu.
	mirrorMu          sync.Mutex
	mirrorOverride    *mirrorSource                // If non-nil, this is the mirror and physicalRef for the override client
	prevOverrides     []*dockerClient              // mirror clients replaced by a newer mirrorOverride; kept open because other goroutines may still be reading blobs through them, closed in Close()
	remainingSources  []sysregistriesv2.PullSource // later pull sources not yet tried, consumed front-to-back during fallback; nil when selected source is already the original source.
	fallbackSys       *types.SystemContext
	fallbackRegConfig *registryConfiguration
}

type mirrorSource struct {
	client *dockerClient
	ref    dockerReference
}

type pullEndpoint struct {
	client      *dockerClient
	ref         dockerReference
	endpointSys *types.SystemContext
}

type sourceAttempt struct {
	ref reference.Named
	err error
}

// newImageSource creates a new ImageSource for the specified image reference.
// The caller must call .Close() on the returned ImageSource.
// The caller must ensure !ref.isUnknownDigest.
func newImageSource(ctx context.Context, sys *types.SystemContext, ref dockerReference) (*dockerImageSource, error) {
	if ref.isUnknownDigest {
		return nil, fmt.Errorf("reading images from docker: reference %q without a tag or digest is not supported", ref.StringWithinTransport())
	}

	registryConfig, err := loadRegistryConfiguration(sys)
	if err != nil {
		return nil, err
	}
	registry, err := sysregistriesv2.FindRegistry(sys, ref.ref.Name())
	if err != nil {
		return nil, fmt.Errorf("loading registries configuration: %w", err)
	}
	if registry == nil {
		// No configuration was found for the provided reference, so use the
		// equivalent of a default configuration.
		registry = &sysregistriesv2.Registry{
			Endpoint: sysregistriesv2.Endpoint{
				Location: ref.ref.String(),
			},
			Prefix: ref.ref.String(),
		}
	}

	// Check all endpoints for the manifest availability. If we find one that does
	// contain the image, it will be used for all future pull actions.  Always try the
	// non-mirror original location last; this both transparently handles the case
	// of no mirrors configured, and ensures we return the error encountered when
	// accessing the upstream location if all endpoints fail.
	pullSources, err := registry.PullSourcesFromReference(ref.ref)
	if err != nil {
		return nil, err
	}
	attempts := []sourceAttempt{}
	for i, pullSource := range pullSources {
		if sys != nil && sys.DockerLogMirrorChoice {
			logrus.Infof("Trying to access %q", pullSource.Reference)
		} else {
			logrus.Debugf("Trying to access %q", pullSource.Reference)
		}
		s, err := newImageSourceAttempt(ctx, sys, ref, pullSource, registryConfig)
		if err == nil {
			if i+1 < len(pullSources) {
				s.remainingSources = pullSources[i+1:]
				s.fallbackSys = sys
				s.fallbackRegConfig = registryConfig
			}
			return s, nil
		}
		logrus.Debugf("Accessing %q failed: %v", pullSource.Reference, err)
		attempts = append(attempts, sourceAttempt{
			ref: pullSource.Reference,
			err: err,
		})
	}
	switch len(attempts) {
	case 0:
		return nil, errors.New("Internal error: newImageSource returned without trying any endpoint")
	case 1:
		return nil, attempts[0].err // If no mirrors are used, perfectly preserve the error type and add no noise.
	default:
		// Don’t just build a string, try to preserve the typed error.
		primary := &attempts[len(attempts)-1]
		extras := []string{}
		for _, attempt := range attempts[:len(attempts)-1] {
			// This is difficult to fit into a single-line string, when the error can contain arbitrary strings including any metacharacters we decide to use.
			// The paired [] at least have some chance of being unambiguous.
			extras = append(extras, fmt.Sprintf("[%s: %v]", attempt.ref.String(), attempt.err))
		}
		return nil, fmt.Errorf("(Mirrors also failed: %s): %s: %w", strings.Join(extras, "\n"), primary.ref.String(), primary.err)
	}
}

// newImageSourceAttempt is an internal helper for newImageSource. Everyone else must call newImageSource.
// Given a logicalReference and a pullSource, return a dockerImageSource if it is reachable.
// The caller must call .Close() on the returned ImageSource.
func newImageSourceAttempt(ctx context.Context, sys *types.SystemContext, logicalRef dockerReference, pullSource sysregistriesv2.PullSource,
	registryConfig *registryConfiguration,
) (*dockerImageSource, error) {
	endpoint, err := newPullClient(sys, logicalRef, pullSource, registryConfig)
	if err != nil {
		return nil, err
	}

	s := &dockerImageSource{
		PropertyMethodsInitialize: impl.PropertyMethods(impl.Properties{
			HasThreadSafeGetBlob: true,
		}),

		logicalRef:  logicalRef,
		physicalRef: endpoint.ref,
		c:           endpoint.client,
	}
	s.Compat = impl.AddCompat(s)

	if err := s.ensureManifestIsLoaded(ctx); err != nil {
		endpoint.client.Close()
		return nil, err
	}

	if h, err := sysregistriesv2.AdditionalLayerStoreAuthHelper(endpoint.endpointSys); err == nil && h != "" {
		acf := map[string]struct {
			Username      string `json:"username,omitempty"`
			Password      string `json:"password,omitempty"`
			IdentityToken string `json:"identityToken,omitempty"`
		}{
			endpoint.ref.ref.String(): {
				Username:      endpoint.client.auth.Username,
				Password:      endpoint.client.auth.Password,
				IdentityToken: endpoint.client.auth.IdentityToken,
			},
		}
		acfD, err := json.Marshal(acf)
		if err != nil {
			logrus.Warnf("failed to marshal auth config: %v", err)
		} else {
			cmd := exec.Command(h)
			cmd.Stdin = bytes.NewReader(acfD)
			if err := cmd.Run(); err != nil {
				var stderr string
				if ee, ok := err.(*exec.ExitError); ok {
					stderr = string(ee.Stderr)
				}
				logrus.Warnf("Failed to call additional-layer-store-auth-helper (stderr:%s): %v", stderr, err)
			}
		}
	}
	return s, nil
}

// newPullClient creates a dockerClient, reference, and endpoint-specific SystemContext
// for the given pull source.
// The caller must call client.Close() when done.
func newPullClient(sys *types.SystemContext, logicalRef dockerReference, pullSource sysregistriesv2.PullSource,
	registryConfig *registryConfiguration,
) (*pullEndpoint, error) {
	physicalRef, err := newReference(pullSource.Reference, false)
	if err != nil {
		return nil, err
	}

	endpointSys := sys
	// sys.DockerAuthConfig does not explicitly specify a registry; we must not blindly send the credentials intended for the primary endpoint to mirrors.
	if endpointSys != nil && endpointSys.DockerAuthConfig != nil && reference.Domain(physicalRef.ref) != reference.Domain(logicalRef.ref) {
		copy := *endpointSys
		copy.DockerAuthConfig = nil
		copy.DockerBearerRegistryToken = ""
		endpointSys = &copy
	}

	client, err := newDockerClientFromRef(endpointSys, physicalRef, registryConfig, false, "pull")
	if err != nil {
		return nil, err
	}
	client.tlsClientConfig.InsecureSkipVerify = pullSource.Endpoint.Insecure
	client.namespaceProxy = pullSource.Endpoint.NamespaceProxy

	return &pullEndpoint{client: client, ref: physicalRef, endpointSys: endpointSys}, nil
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *dockerImageSource) Reference() types.ImageReference {
	return s.logicalRef
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *dockerImageSource) Close() error {
	s.mirrorMu.Lock()
	prev := s.prevOverrides
	override := s.mirrorOverride
	s.prevOverrides = nil
	s.mirrorOverride = nil
	s.mirrorMu.Unlock()
	for _, m := range prev {
		m.Close()
	}
	if override != nil {
		override.client.Close()
	}
	return s.c.Close()
}

// simplifyContentType drops parameters from a HTTP media type (see https://tools.ietf.org/html/rfc7231#section-3.1.1.1)
// Alternatively, an empty string is returned unchanged, and invalid values are "simplified" to an empty string.
func simplifyContentType(contentType string) string {
	if contentType == "" {
		return contentType
	}
	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return mimeType
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
func (s *dockerImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if instanceDigest != nil {
		if err := instanceDigest.Validate(); err != nil { // Make sure instanceDigest.String() does not contain any unexpected characters
			return nil, "", err
		}
		return s.fetchManifest(ctx, instanceDigest.String())
	}
	err := s.ensureManifestIsLoaded(ctx)
	if err != nil {
		return nil, "", err
	}
	return s.cachedManifest, s.cachedManifestMIMEType, nil
}

// fetchManifest fetches a manifest for tagOrDigest.
// The caller is responsible for ensuring tagOrDigest uses the expected format.
func (s *dockerImageSource) fetchManifest(ctx context.Context, tagOrDigest string) ([]byte, string, error) {
	return s.c.fetchManifest(ctx, s.physicalRef, tagOrDigest)
}

// ensureManifestIsLoaded sets s.cachedManifest and s.cachedManifestMIMEType
//
// ImageSource implementations are not required or expected to do any caching,
// but because our signatures are “attached” to the manifest digest,
// we need to ensure that the digest of the manifest returned by GetManifest(ctx, nil)
// and used by GetSignatures(ctx, nil) are consistent, otherwise we would get spurious
// signature verification failures when pulling while a tag is being updated.
func (s *dockerImageSource) ensureManifestIsLoaded(ctx context.Context) error {
	if s.cachedManifest != nil {
		return nil
	}

	reference, err := s.physicalRef.tagOrDigest()
	if err != nil {
		return err
	}

	manblob, mt, err := s.fetchManifest(ctx, reference)
	if err != nil {
		return err
	}
	// We might validate manblob against the Docker-Content-Digest header here to protect against transport errors.
	s.cachedManifest = manblob
	s.cachedManifestMIMEType = mt
	return nil
}

// splitHTTP200ResponseToPartial splits a 200 response in multiple streams as specified by the chunks
func splitHTTP200ResponseToPartial(streams chan io.ReadCloser, errs chan error, body io.ReadCloser, chunks []private.ImageSourceChunk) {
	defer close(streams)
	defer close(errs)
	currentOffset := uint64(0)

	body = makeBufferedNetworkReader(body, 64, 16384)
	defer body.Close()
	for _, c := range chunks {
		if c.Offset != currentOffset {
			if c.Offset < currentOffset {
				errs <- fmt.Errorf("invalid chunk offset specified %v (expected >= %v)", c.Offset, currentOffset)
				break
			}
			toSkip := c.Offset - currentOffset
			if _, err := io.Copy(io.Discard, io.LimitReader(body, int64(toSkip))); err != nil {
				errs <- err
				break
			}
			currentOffset += toSkip
		}
		var reader io.Reader
		if c.Length == math.MaxUint64 {
			reader = body
		} else {
			reader = io.LimitReader(body, int64(c.Length))
		}
		s := signalCloseReader{
			closed:        make(chan struct{}),
			stream:        io.NopCloser(reader),
			consumeStream: true,
		}
		streams <- s

		// Wait until the stream is closed before going to the next chunk
		<-s.closed
		currentOffset += c.Length
	}
}

// handle206Response reads a 206 response and send each part as a separate ReadCloser to the streams chan.
func handle206Response(streams chan io.ReadCloser, errs chan error, body io.ReadCloser, chunks []private.ImageSourceChunk, mediaType string, params map[string]string) {
	defer close(streams)
	defer close(errs)
	if !strings.HasPrefix(mediaType, "multipart/") {
		streams <- body
		return
	}
	boundary, found := params["boundary"]
	if !found {
		errs <- errors.New("could not find boundary")
		body.Close()
		return
	}
	buffered := makeBufferedNetworkReader(body, 64, 16384)
	defer buffered.Close()
	mr := multipart.NewReader(buffered, boundary)
	parts := 0
	for {
		p, err := mr.NextPart()
		if err != nil {
			if err != io.EOF {
				errs <- err
			}
			if parts != len(chunks) {
				errs <- errors.New("invalid number of chunks returned by the server")
			}
			return
		}
		if parts >= len(chunks) {
			errs <- errors.New("too many parts returned by the server")
			break
		}
		s := signalCloseReader{
			closed: make(chan struct{}),
			stream: p,
		}
		streams <- s
		// NextPart() cannot be called while the current part
		// is being read, so wait until it is closed
		<-s.closed
		parts++
	}
}

var multipartByteRangesRe = regexp.Delayed("multipart/byteranges; boundary=([A-Za-z-0-9:]+)")

func parseMediaType(contentType string) (string, map[string]string, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		if err == mime.ErrInvalidMediaParameter {
			// CloudFront returns an invalid MIME type, that contains an unquoted ":" in the boundary
			// param, let's handle it here.
			matches := multipartByteRangesRe.FindStringSubmatch(contentType)
			if len(matches) == 2 {
				mediaType = "multipart/byteranges"
				params = map[string]string{
					"boundary": matches[1],
				}
				err = nil
			}
		}
		if err != nil {
			return "", nil, err
		}
	}
	return mediaType, params, err
}

// GetBlobAt returns a sequential channel of readers that contain data for the requested
// blob chunks, and a channel that might get a single error value.
// The specified chunks must be not overlapping and sorted by their offset.
// The readers must be fully consumed, in the order they are returned, before blocking
// to read the next chunk.
// If the Length for the last chunk is set to math.MaxUint64, then it
// fully fetches the remaining data from the offset to the end of the blob.
func (s *dockerImageSource) GetBlobAt(ctx context.Context, info types.BlobInfo, chunks []private.ImageSourceChunk) (chan io.ReadCloser, chan error, error) {
	headers := make(map[string][]string)

	rangeVals := make([]string, 0, len(chunks))
	lastFound := false
	for _, c := range chunks {
		if lastFound {
			return nil, nil, fmt.Errorf("internal error: another chunk requested after an util-EOF chunk")
		}
		// If the Length is set to -1, then request anything after the specified offset.
		if c.Length == math.MaxUint64 {
			lastFound = true
			rangeVals = append(rangeVals, fmt.Sprintf("%d-", c.Offset))
		} else {
			rangeVals = append(rangeVals, fmt.Sprintf("%d-%d", c.Offset, c.Offset+c.Length-1))
		}
	}

	headers["Range"] = []string{fmt.Sprintf("bytes=%s", strings.Join(rangeVals, ","))}

	if len(info.URLs) != 0 {
		return nil, nil, fmt.Errorf("external URLs not supported with GetBlobAt")
	}

	if err := info.Digest.Validate(); err != nil { // Make sure info.Digest.String() does not contain any unexpected characters
		return nil, nil, err
	}

	client, physRef, hasRemaining := s.getActiveSource()
	streams, errs, err := tryGetBlobAt(ctx, client, physRef, info, chunks, headers, hasRemaining)
	if err == nil || !hasRemaining {
		return streams, errs, err
	}
	if isMirrorTransientError(err) || isMirrorFallbackError(err) {
		logrus.Debugf("Partial blob %s fetch from %q failed (%v), trying fallback sources", info.Digest, physRef.ref, err)
		return s.getBlobAtWithMirrorFallback(ctx, info, chunks, headers, err, client)
	}
	return streams, errs, err
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *dockerImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	client, physRef, hasRemaining := s.getActiveSource()
	reader, size, err := tryGetBlob(ctx, client, physRef, info, cache, hasRemaining)
	if err == nil || !hasRemaining {
		return reader, size, err
	}
	if isMirrorTransientError(err) || isMirrorFallbackError(err) {
		logrus.Debugf("Blob %s fetch from %q failed (%v), trying fallback sources", info.Digest, physRef.ref, err)
		return s.getBlobWithMirrorFallback(ctx, info, cache, err, client)
	}
	return reader, size, err
}

func (s *dockerImageSource) getActiveSource() (*dockerClient, dockerReference, bool) {
	s.mirrorMu.Lock()
	defer s.mirrorMu.Unlock()
	hasRemaining := len(s.remainingSources) > 0
	if s.mirrorOverride != nil {
		return s.mirrorOverride.client, s.mirrorOverride.ref, hasRemaining
	}
	return s.c, s.physicalRef, hasRemaining
}

// tryGetBlob attempts to fetch a blob, retrying once on transient errors if fallback mirrors remain.
func tryGetBlob(ctx context.Context, client *dockerClient, physRef dockerReference,
	info types.BlobInfo, cache types.BlobInfoCache, hasRemainingSources bool,
) (io.ReadCloser, int64, error) {
	reader, size, err := client.getBlob(ctx, physRef, info, cache)
	if err != nil && hasRemainingSources && isMirrorTransientError(err) {
		logrus.Debugf("Transient error fetching blob %s from %q, retrying: %v", info.Digest, physRef.ref, err)
		delay := time.Second + rand.N(time.Second/10) // same as retry.IfNecessary first attempt: 1s + 10% jitter
		select {
		case <-ctx.Done():
			return nil, 0, fmt.Errorf("%w (while retrying after: %v)", ctx.Err(), err)
		case <-time.After(delay):
		}
		reader, size, err = client.getBlob(ctx, physRef, info, cache)
	}
	return reader, size, err
}

// tryGetBlobAt attempts to fetch a partial blob (range request), retrying once on transient errors
// if fallback mirrors remain. Mirrors tryGetBlob for the partial-pull (zstd:chunked) path.
func tryGetBlobAt(ctx context.Context, client *dockerClient, physRef dockerReference,
	info types.BlobInfo, chunks []private.ImageSourceChunk, headers map[string][]string, hasRemainingSources bool,
) (chan io.ReadCloser, chan error, error) {
	streams, errs, err := fetchBlobAt(ctx, client, physRef, info, chunks, headers)
	if err != nil && hasRemainingSources && isMirrorTransientError(err) {
		logrus.Debugf("Transient error fetching partial blob %s from %q, retrying: %v", info.Digest, physRef.ref, err)
		delay := time.Second + rand.N(time.Second/10)
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("%w (while retrying after: %v)", ctx.Err(), err)
		case <-time.After(delay):
		}
		streams, errs, err = fetchBlobAt(ctx, client, physRef, info, chunks, headers)
	}
	return streams, errs, err
}

func fetchBlobAt(ctx context.Context, client *dockerClient, physRef dockerReference,
	info types.BlobInfo, chunks []private.ImageSourceChunk, headers map[string][]string,
) (chan io.ReadCloser, chan error, error) {
	path := fmt.Sprintf(blobsPath, reference.Path(physRef.ref), info.Digest.String())
	logrus.Debugf("Downloading %s", path)
	res, err := client.makeRequest(ctx, http.MethodGet, path, headers, nil, v2Auth, nil)
	if err != nil {
		return nil, nil, err
	}

	switch res.StatusCode {
	case http.StatusOK:
		// if the server replied with a 200 status code, convert the full body response to a series of
		// streams as it would have been done with 206.
		streams := make(chan io.ReadCloser)
		errs := make(chan error)
		go splitHTTP200ResponseToPartial(streams, errs, res.Body, chunks)
		return streams, errs, nil
	case http.StatusPartialContent:
		mediaType, params, err := parseMediaType(res.Header.Get("Content-Type"))
		if err != nil {
			return nil, nil, err
		}
		streams := make(chan io.ReadCloser)
		errs := make(chan error)
		go handle206Response(streams, errs, res.Body, chunks, mediaType, params)
		return streams, errs, nil
	case http.StatusBadRequest:
		res.Body.Close()
		return nil, nil, private.BadPartialRequestError{Status: res.Status}
	default:
		httpErr := registryHTTPResponseToError(res)
		res.Body.Close()
		return nil, nil, fmt.Errorf("fetching partial blob: %w", httpErr)
	}
}

// isMirrorFallbackError returns true for errors where the blob is not present
// on this source but may be available from another — warranting a fallback attempt.
// All tested registries (docker.io, registry.redhat.io, quay.io, registry.access.redhat.com)
// return BLOB_UNKNOWN for blob 404s, matching the OCI distribution spec.
func isMirrorFallbackError(err error) bool {
	var ec errcode.ErrorCoder
	if errors.As(err, &ec) {
		switch ec.ErrorCode() {
		case v2.ErrorCodeBlobUnknown:
			// Blob endpoint returns HTTP 404 with errcode JSON body
			// {"errors":[{"code":"BLOB_UNKNOWN",...}]}.
			// registryHTTPResponseToError calls handleErrorResponse → parseHTTPErrorResponse,
			// then unwraps errcode.Errors to the first errcode.Error.
			return true
		case errcode.ErrorCodeTooManyRequests:
			// makeRequestToResolvedURL already retried 5 times with exponential
			// backoff. This source's rate limit is exhausted — skip straight to
			// the next mirror which has its own rate limit budget.
			return true
		}
	}

	// UnexpectedHTTPStatusError (capital U) is NOT matched here — for regular blobs,
	// handleErrorResponse never produces it for 4xx (only for 5xx). For external blobs,
	// getExternalBlob() produces it via newUnexpectedHTTPStatusError() directly for any
	// non-200 including 404, but external URLs are fixed and mirror-independent —
	// a different registry mirror cannot serve them.
	return false
}

// isMirrorTransientError returns true for errors that are transient; the source
// probably has the blob but temporarily cannot serve it.
func isMirrorTransientError(err error) bool {
	// HTTP 5xx: handleErrorResponse returns UnexpectedHTTPStatusError for status
	// codes outside 400–499. Server-side error, another mirror may succeed.
	var httpErr UnexpectedHTTPStatusError
	if errors.As(err, &httpErr) && httpErr.StatusCode >= 500 {
		return true
	}

	// Network timeout: makeRequest returns net.Error with Timeout() == true.
	// The mirror is reachable but slow — worth retrying on another.
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// retireStaleOverride checks if the current mirrorOverride was set by another
// goroutine and differs from failedClient. If so, it returns the override's
// client and ref for the caller to retry. If the override is the same client
// that already failed, or is nil, it returns nil. Must be called with mirrorMu held.
func (s *dockerImageSource) retireStaleOverride(failedClient *dockerClient) (*dockerClient, dockerReference, bool) {
	if s.mirrorOverride != nil && s.mirrorOverride.client != failedClient {
		return s.mirrorOverride.client, s.mirrorOverride.ref, true
	}
	return nil, dockerReference{}, false
}

// retireCurrentOverride moves the current mirrorOverride to prevOverrides.
// Must be called with mirrorMu held.
func (s *dockerImageSource) retireCurrentOverride() {
	if s.mirrorOverride != nil {
		s.prevOverrides = append(s.prevOverrides, s.mirrorOverride.client)
		s.mirrorOverride = nil
	}
}

func formatFallbackError(attempts []sourceAttempt, originalErr error) error {
	if len(attempts) == 0 {
		return originalErr
	}
	extras := make([]string, 0, len(attempts))
	for _, a := range attempts {
		extras = append(extras, fmt.Sprintf("[%s: %v]", a.ref.String(), a.err))
	}
	return fmt.Errorf("(Fallback sources also failed: %s): %w", strings.Join(extras, "\n"), originalErr)
}

func (s *dockerImageSource) getBlobWithMirrorFallback(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, originalErr error, failedClient *dockerClient) (io.ReadCloser, int64, error) {
	// Held for the full fallback loop, including network I/O. This serializes
	// concurrent blob fetchers during fallback — only one goroutine probes
	// sources while the rest block on getActiveSource(). Acceptable trade-off:
	// fallback is rare and serialization prevents thundering-herd probing.
	s.mirrorMu.Lock()
	defer s.mirrorMu.Unlock()

	if client, physRef, ok := s.retireStaleOverride(failedClient); ok {
		reader, size, err := tryGetBlob(ctx, client, physRef, info, cache, len(s.remainingSources) > 0)
		if err == nil {
			return reader, size, nil
		}
		s.retireCurrentOverride()
	}

	attempts := []sourceAttempt{}
	for len(s.remainingSources) > 0 {
		pullSource := s.remainingSources[0]
		s.remainingSources = s.remainingSources[1:]

		logrus.Debugf("Trying to access %q", pullSource.Reference)

		fallbackEndpoint, clientErr := newPullClient(s.fallbackSys, s.logicalRef, pullSource, s.fallbackRegConfig)
		if clientErr != nil {
			logrus.Debugf("Accessing %q failed: %v", pullSource.Reference, clientErr)
			attempts = append(attempts, sourceAttempt{ref: pullSource.Reference, err: clientErr})
			continue
		}

		reader, size, err := tryGetBlob(ctx, fallbackEndpoint.client, fallbackEndpoint.ref, info, cache, len(s.remainingSources) > 0)
		if err != nil {
			fallbackEndpoint.client.Close()
			logrus.Debugf("Accessing %q failed: %v", pullSource.Reference, err)
			attempts = append(attempts, sourceAttempt{ref: pullSource.Reference, err: err})
			if !isMirrorTransientError(err) && !isMirrorFallbackError(err) {
				// non-transient errors are unlikely to be resolved by trying another source: stop probing further.
				// break here to log the attempts and return originalErr below.
				break
			}
			continue
		}

		s.retireCurrentOverride()
		s.mirrorOverride = &mirrorSource{client: fallbackEndpoint.client, ref: fallbackEndpoint.ref}
		logrus.Debugf("Blob fetch succeeded from fallback source %q, switching to it for future requests", pullSource.Reference)

		return reader, size, nil
	}
	return nil, 0, formatFallbackError(attempts, originalErr)
}

func (s *dockerImageSource) getBlobAtWithMirrorFallback(ctx context.Context, info types.BlobInfo, chunks []private.ImageSourceChunk, headers map[string][]string, originalErr error, failedClient *dockerClient) (chan io.ReadCloser, chan error, error) {
	s.mirrorMu.Lock()
	defer s.mirrorMu.Unlock()

	if client, physRef, ok := s.retireStaleOverride(failedClient); ok {
		streams, errs, err := tryGetBlobAt(ctx, client, physRef, info, chunks, headers, len(s.remainingSources) > 0)
		if err == nil {
			return streams, errs, nil
		}
		s.retireCurrentOverride()
	}

	attempts := []sourceAttempt{}
	for len(s.remainingSources) > 0 {
		pullSource := s.remainingSources[0]
		s.remainingSources = s.remainingSources[1:]

		logrus.Debugf("Trying to access %q", pullSource.Reference)

		fallbackEndpoint, clientErr := newPullClient(s.fallbackSys, s.logicalRef, pullSource, s.fallbackRegConfig)
		if clientErr != nil {
			logrus.Debugf("Accessing %q failed: %v", pullSource.Reference, clientErr)
			attempts = append(attempts, sourceAttempt{ref: pullSource.Reference, err: clientErr})
			continue
		}

		streams, errs, err := tryGetBlobAt(ctx, fallbackEndpoint.client, fallbackEndpoint.ref, info, chunks, headers, len(s.remainingSources) > 0)
		if err != nil {
			fallbackEndpoint.client.Close()
			logrus.Debugf("Accessing %q failed: %v", pullSource.Reference, err)
			attempts = append(attempts, sourceAttempt{ref: pullSource.Reference, err: err})
			if !isMirrorTransientError(err) && !isMirrorFallbackError(err) {
				// non-transient errors are unlikely to be resolved by trying another source: stop probing further.
				// break here to log the attempts and return originalErr below.
				break
			}
			continue
		}

		s.retireCurrentOverride()
		s.mirrorOverride = &mirrorSource{client: fallbackEndpoint.client, ref: fallbackEndpoint.ref}
		logrus.Debugf("Partial blob fetch succeeded from fallback source %q, switching to it for future requests", pullSource.Reference)

		return streams, errs, nil
	}
	return nil, nil, formatFallbackError(attempts, originalErr)
}

// GetSignaturesWithFormat returns the image's signatures.  It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
func (s *dockerImageSource) GetSignaturesWithFormat(ctx context.Context, instanceDigest *digest.Digest) ([]signature.Signature, error) {
	if err := s.c.detectProperties(ctx); err != nil {
		return nil, err
	}
	var res []signature.Signature
	switch {
	case s.c.supportsSignatures:
		if err := s.appendSignaturesFromAPIExtension(ctx, &res, instanceDigest); err != nil {
			return nil, err
		}
	case s.c.signatureBase != nil:
		if err := s.appendSignaturesFromLookaside(ctx, &res, instanceDigest); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("Internal error: X-Registry-Supports-Signatures extension not supported, and lookaside should not be empty configuration")
	}

	if err := s.appendSignaturesFromSigstoreAttachments(ctx, &res, instanceDigest); err != nil {
		return nil, err
	}
	return res, nil
}

// manifestDigest returns a digest of the manifest, from instanceDigest if non-nil; or from the supplied reference,
// or finally, from a fetched manifest.
func (s *dockerImageSource) manifestDigest(ctx context.Context, instanceDigest *digest.Digest) (digest.Digest, error) {
	if instanceDigest != nil {
		return *instanceDigest, nil
	}
	if digested, ok := s.physicalRef.ref.(reference.Digested); ok {
		d := digested.Digest()
		if d.Algorithm() == digest.Canonical {
			return d, nil
		}
	}
	if err := s.ensureManifestIsLoaded(ctx); err != nil {
		return "", err
	}
	return manifest.Digest(s.cachedManifest)
}

// appendSignaturesFromLookaside implements GetSignaturesWithFormat() from the lookaside location configured in s.c.signatureBase,
// which is not nil, storing the signatures to *dest.
// On error, the contents of *dest are undefined.
func (s *dockerImageSource) appendSignaturesFromLookaside(ctx context.Context, dest *[]signature.Signature, instanceDigest *digest.Digest) error {
	manifestDigest, err := s.manifestDigest(ctx, instanceDigest)
	if err != nil {
		return err
	}

	// NOTE: Keep this in sync with docs/signature-protocols.md!
	for i := 0; ; i++ {
		if i >= maxLookasideSignatures {
			return fmt.Errorf("server provided %d signatures, assuming that's unreasonable and a server error", maxLookasideSignatures)
		}

		sigURL, err := lookasideStorageURL(s.c.signatureBase, manifestDigest, i)
		if err != nil {
			return err
		}
		signature, missing, err := s.getOneSignature(ctx, sigURL)
		if err != nil {
			return err
		}
		if missing {
			break
		}
		*dest = append(*dest, signature)
	}
	return nil
}

// getOneSignature downloads one signature from sigURL, and returns (signature, false, nil)
// If it successfully determines that the signature does not exist, returns (nil, true, nil).
// NOTE: Keep this in sync with docs/signature-protocols.md!
func (s *dockerImageSource) getOneSignature(ctx context.Context, sigURL *url.URL) (signature.Signature, bool, error) {
	switch sigURL.Scheme {
	case "file":
		logrus.Debugf("Reading %s", sigURL.Path)
		sigBlob, err := os.ReadFile(sigURL.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, true, nil
			}
			return nil, false, err
		}
		sig, err := signature.FromBlob(sigBlob)
		if err != nil {
			return nil, false, fmt.Errorf("parsing signature %q: %w", sigURL.Path, err)
		}
		return sig, false, nil

	case "http", "https":
		logrus.Debugf("GET %s", sigURL.Redacted())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, sigURL.String(), nil)
		if err != nil {
			return nil, false, err
		}
		res, err := s.c.client.Do(req)
		if err != nil {
			return nil, false, err
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusNotFound {
			logrus.Debugf("... got status 404, as expected = end of signatures")
			return nil, true, nil
		} else if res.StatusCode != http.StatusOK {
			return nil, false, fmt.Errorf("reading signature from %s: %w", sigURL.Redacted(), newUnexpectedHTTPStatusError(res))
		}

		contentType := res.Header.Get("Content-Type")
		if mimeType := simplifyContentType(contentType); mimeType == "text/html" {
			logrus.Warnf("Signature %q has Content-Type %q, unexpected for a signature", sigURL.Redacted(), contentType)
			// Don’t immediately fail; the lookaside spec does not place any requirements on Content-Type.
			// If the content really is HTML, it’s going to fail in signature.FromBlob.
		}

		sigBlob, err := iolimits.ReadAtMost(res.Body, iolimits.MaxSignatureBodySize)
		if err != nil {
			return nil, false, err
		}
		sig, err := signature.FromBlob(sigBlob)
		if err != nil {
			return nil, false, fmt.Errorf("parsing signature %s: %w", sigURL.Redacted(), err)
		}
		return sig, false, nil

	default:
		return nil, false, fmt.Errorf("Unsupported scheme when reading signature from %s", sigURL.Redacted())
	}
}

// appendSignaturesFromAPIExtension implements GetSignaturesWithFormat() using the X-Registry-Supports-Signatures API extension,
// storing the signatures to *dest.
// On error, the contents of *dest are undefined.
func (s *dockerImageSource) appendSignaturesFromAPIExtension(ctx context.Context, dest *[]signature.Signature, instanceDigest *digest.Digest) error {
	manifestDigest, err := s.manifestDigest(ctx, instanceDigest)
	if err != nil {
		return err
	}

	parsedBody, err := s.c.getExtensionsSignatures(ctx, s.physicalRef, manifestDigest)
	if err != nil {
		return err
	}

	for _, sig := range parsedBody.Signatures {
		if sig.Version == extensionSignatureSchemaVersion && sig.Type == extensionSignatureTypeAtomic {
			*dest = append(*dest, signature.SimpleSigningFromBlob(sig.Content))
		}
	}
	return nil
}

// appendSignaturesFromSigstoreAttachments implements GetSignaturesWithFormat() using the sigstore tag convention,
// storing the signatures to *dest.
// On error, the contents of *dest are undefined.
func (s *dockerImageSource) appendSignaturesFromSigstoreAttachments(ctx context.Context, dest *[]signature.Signature, instanceDigest *digest.Digest) error {
	if !s.c.useSigstoreAttachments {
		logrus.Debugf("Not looking for sigstore attachments: disabled by configuration")
		return nil
	}

	manifestDigest, err := s.manifestDigest(ctx, instanceDigest)
	if err != nil {
		return err
	}

	ociManifest, err := s.c.getSigstoreAttachmentManifest(ctx, s.physicalRef, manifestDigest)
	if err != nil {
		return err
	}
	if ociManifest == nil {
		return nil
	}

	logrus.Debugf("Found a sigstore attachment manifest with %d layers", len(ociManifest.Layers))
	for layerIndex, layer := range ociManifest.Layers {
		// Note that this copies all kinds of attachments: attestations, and whatever else is there,
		// not just signatures. We leave the signature consumers to decide based on the MIME type.
		logrus.Debugf("Fetching sigstore attachment %d/%d: %s", layerIndex+1, len(ociManifest.Layers), layer.Digest.String())
		// We don’t benefit from a real BlobInfoCache here because we never try to reuse/mount attachment payloads.
		// That might eventually need to change if payloads grow to be not just signatures, but something
		// significantly large.
		payload, err := s.c.getOCIDescriptorContents(ctx, s.physicalRef, layer, iolimits.MaxSignatureBodySize,
			none.NoCache)
		if err != nil {
			return err
		}
		*dest = append(*dest, signature.SigstoreFromComponents(layer.MediaType, payload, layer.Annotations))
	}
	return nil
}

// deleteImage deletes the named image from the registry, if supported.
func deleteImage(ctx context.Context, sys *types.SystemContext, ref dockerReference) error {
	if ref.isUnknownDigest {
		return fmt.Errorf("Docker reference without a tag or digest cannot be deleted")
	}

	registryConfig, err := loadRegistryConfiguration(sys)
	if err != nil {
		return err
	}
	// docker/distribution does not document what action should be used for deleting images.
	//
	// Current docker/distribution requires "pull" for reading the manifest and "delete" for deleting it.
	// quay.io requires "push" (an explicit "pull" is unnecessary), does not grant any token (fails parsing the request) if "delete" is included.
	// OpenShift ignores the action string (both the password and the token is an OpenShift API token identifying a user).
	//
	// We have to hard-code a single string, luckily both docker/distribution and quay.io support "*" to mean "everything".
	c, err := newDockerClientFromRef(sys, ref, registryConfig, true, "*")
	if err != nil {
		return err
	}
	defer c.Close()

	headers := map[string][]string{
		"Accept": manifest.DefaultRequestedManifestMIMETypes,
	}
	refTail, err := ref.tagOrDigest()
	if err != nil {
		return err
	}
	getPath := fmt.Sprintf(manifestPath, reference.Path(ref.ref), refTail)
	get, err := c.makeRequest(ctx, http.MethodGet, getPath, headers, nil, v2Auth, nil)
	if err != nil {
		return err
	}
	defer get.Body.Close()
	switch get.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return fmt.Errorf("Unable to delete %v. Image may not exist or is not stored with a v2 Schema in a v2 registry", ref.ref)
	default:
		return fmt.Errorf("deleting %v: %w", ref.ref, registryHTTPResponseToError(get))
	}
	manifestBody, err := iolimits.ReadAtMost(get.Body, iolimits.MaxManifestBodySize)
	if err != nil {
		return err
	}

	manifestDigest, err := manifest.Digest(manifestBody)
	if err != nil {
		return fmt.Errorf("computing manifest digest: %w", err)
	}
	deletePath := fmt.Sprintf(manifestPath, reference.Path(ref.ref), manifestDigest)

	// When retrieving the digest from a registry >= 2.3 use the following header:
	//   "Accept": "application/vnd.docker.distribution.manifest.v2+json"
	delete, err := c.makeRequest(ctx, http.MethodDelete, deletePath, headers, nil, v2Auth, nil)
	if err != nil {
		return err
	}
	defer delete.Body.Close()
	if delete.StatusCode != http.StatusAccepted {
		return fmt.Errorf("deleting %v: %w", ref.ref, registryHTTPResponseToError(delete))
	}

	for i := 0; ; i++ {
		sigURL, err := lookasideStorageURL(c.signatureBase, manifestDigest, i)
		if err != nil {
			return err
		}
		missing, err := c.deleteOneSignature(sigURL)
		if err != nil {
			return err
		}
		if missing {
			break
		}
	}

	return nil
}

type bufferedNetworkReaderBuffer struct {
	data     []byte
	len      int
	consumed int
	err      error
}

type bufferedNetworkReader struct {
	stream      io.ReadCloser
	emptyBuffer chan *bufferedNetworkReaderBuffer
	readyBuffer chan *bufferedNetworkReaderBuffer
	terminate   chan bool
	current     *bufferedNetworkReaderBuffer
	mutex       sync.Mutex
	gotEOF      bool
}

// handleBufferedNetworkReader runs in a goroutine
func handleBufferedNetworkReader(br *bufferedNetworkReader) {
	defer close(br.readyBuffer)
	for {
		select {
		case b := <-br.emptyBuffer:
			b.len, b.err = br.stream.Read(b.data)
			br.readyBuffer <- b
			if b.err != nil {
				return
			}
		case <-br.terminate:
			return
		}
	}
}

func (n *bufferedNetworkReader) Close() error {
	close(n.terminate)
	close(n.emptyBuffer)
	return n.stream.Close()
}

func (n *bufferedNetworkReader) read(p []byte) (int, error) {
	if n.current != nil {
		copied := copy(p, n.current.data[n.current.consumed:n.current.len])
		n.current.consumed += copied
		if n.current.consumed == n.current.len {
			n.emptyBuffer <- n.current
			n.current = nil
		}
		if copied > 0 {
			return copied, nil
		}
	}
	if n.gotEOF {
		return 0, io.EOF
	}

	var b *bufferedNetworkReaderBuffer

	select {
	case b = <-n.readyBuffer:
		if b.err != nil {
			if b.err != io.EOF {
				return b.len, b.err
			}
			n.gotEOF = true
		}
		b.consumed = 0
		n.current = b
		return n.read(p)
	case <-n.terminate:
		return 0, io.EOF
	}
}

func (n *bufferedNetworkReader) Read(p []byte) (int, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	return n.read(p)
}

func makeBufferedNetworkReader(stream io.ReadCloser, nBuffers, bufferSize uint) *bufferedNetworkReader {
	br := bufferedNetworkReader{
		stream:      stream,
		emptyBuffer: make(chan *bufferedNetworkReaderBuffer, nBuffers),
		readyBuffer: make(chan *bufferedNetworkReaderBuffer, nBuffers),
		terminate:   make(chan bool),
	}

	go func() {
		handleBufferedNetworkReader(&br)
	}()

	for range nBuffers {
		b := bufferedNetworkReaderBuffer{
			data: make([]byte, bufferSize),
		}
		br.emptyBuffer <- &b
	}

	return &br
}

type signalCloseReader struct {
	closed        chan struct{}
	stream        io.ReadCloser
	consumeStream bool
}

func (s signalCloseReader) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s signalCloseReader) Close() error {
	defer close(s.closed)
	if s.consumeStream {
		if _, err := io.Copy(io.Discard, s.stream); err != nil {
			s.stream.Close()
			return err
		}
	}
	return s.stream.Close()
}
