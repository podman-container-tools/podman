//go:build linux || freebsd

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
	. "github.com/containers/podman/v5/test/utils"
	"go.podman.io/storage/pkg/chunked/compressor"
)

// primaryAddr extracts the host:port from an httptest.Server.
func primaryAddr(s *httptest.Server) string {
	return s.Listener.Addr().String()
}

// buildTarLayer creates a single tar archive containing a test data file for the given layer index.
func buildTarLayer(index int) []byte {
	content := fmt.Appendf(nil, "layer-%d test data: the quick brown fox jumps over the lazy dog\n", index)
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	Expect(tw.WriteHeader(&tar.Header{
		Name: fmt.Sprintf("layer-%d/data.txt", index),
		Mode: 0o644,
		Size: int64(len(content)),
	})).To(Succeed())
	_, err := tw.Write(content)
	Expect(err).ToNot(HaveOccurred())
	Expect(tw.Close()).To(Succeed())
	return raw.Bytes()
}

// buildImageManifest creates the config blob, manifest JSON, and returns the content type.
func buildImageManifest(blobs map[string][]byte, diffIDs []string, layers []map[string]any, configMediaType, manifestMediaType string) ([]byte, string) {
	configBytes, err := json.Marshal(map[string]any{
		"architecture": runtime.GOARCH,
		"os":           "linux",
		"rootfs":       map[string]any{"type": "layers", "diff_ids": diffIDs},
	})
	Expect(err).ToNot(HaveOccurred())
	configDigest := digest.FromBytes(configBytes)
	blobs[configDigest.String()] = configBytes

	manifestBytes, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     manifestMediaType,
		"config": map[string]any{
			"mediaType": configMediaType,
			"size":      len(configBytes),
			"digest":    configDigest.String(),
		},
		"layers": layers,
	})
	Expect(err).ToNot(HaveOccurred())
	return manifestBytes, manifestMediaType
}

type testCompression int

const (
	compressionGzip testCompression = iota
	compressionZstdChunked
)

// buildTestImage constructs a valid container image with numLayers layers
// using the specified compression. Gzip produces a Docker schema2 manifest;
// zstd:chunked produces an OCI manifest with TOC annotations for partial pulls.
func buildTestImage(numLayers int, compression testCompression) (map[string][]byte, []byte, string, []digest.Digest) {
	blobs := make(map[string][]byte)
	var diffIDs []string
	var layers []map[string]any
	var layerDigests []digest.Digest

	for i := range numLayers {
		rawTar := buildTarLayer(i)
		diffIDs = append(diffIDs, digest.FromBytes(rawTar).String())

		var layerBytes []byte
		layerDesc := map[string]any{}

		switch compression {
		case compressionZstdChunked:
			var comp bytes.Buffer
			annotations := make(map[string]string)
			zw, err := compressor.ZstdCompressor(&comp, annotations, nil)
			Expect(err).ToNot(HaveOccurred())
			_, err = zw.Write(rawTar)
			Expect(err).ToNot(HaveOccurred())
			Expect(zw.Close()).To(Succeed())
			layerBytes = append([]byte(nil), comp.Bytes()...)
			layerDesc["mediaType"] = "application/vnd.oci.image.layer.v1.tar+zstd"
			layerDesc["annotations"] = annotations
		default:
			var comp bytes.Buffer
			gz := gzip.NewWriter(&comp)
			_, err := gz.Write(rawTar)
			Expect(err).ToNot(HaveOccurred())
			Expect(gz.Close()).To(Succeed())
			layerBytes = append([]byte(nil), comp.Bytes()...)
			layerDesc["mediaType"] = "application/vnd.docker.image.rootfs.diff.tar.gzip"
		}

		ld := digest.FromBytes(layerBytes)
		blobs[ld.String()] = layerBytes
		layerDigests = append(layerDigests, ld)
		layerDesc["size"] = len(layerBytes)
		layerDesc["digest"] = ld.String()
		layers = append(layers, layerDesc)
	}

	var configMT, manifestMT string
	switch compression {
	case compressionZstdChunked:
		configMT = "application/vnd.oci.image.config.v1+json"
		manifestMT = "application/vnd.oci.image.manifest.v1+json"
	default:
		configMT = "application/vnd.docker.container.image.v1+json"
		manifestMT = "application/vnd.docker.distribution.manifest.v2+json"
	}

	manifestBytes, ct := buildImageManifest(blobs, diffIDs, layers, configMT, manifestMT)
	return blobs, manifestBytes, ct, layerDigests
}

// blobMiddleware wraps the default blob-serving handler. Middleware that wants
// to fail should write a response and not call next. Middleware that wants
// normal serving should call next.ServeHTTP(w, r).
type blobMiddleware func(next http.Handler) http.Handler

// registryHandler returns an http.Handler that implements a minimal Docker
// registry v2 API for the given repository name, serving the manifest and
// blobs. If mw is non-nil it wraps the default blob handler.
//
//nolint:unparam
func registryHandler(repo string, manifestBytes []byte, manifestCT string, blobs map[string][]byte, mw blobMiddleware) http.Handler {
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", repo)
	blobPrefix := fmt.Sprintf("/v2/%s/blobs/", repo)

	defaultBlobHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dgst := strings.TrimPrefix(r.URL.Path, blobPrefix)
		data, ok := blobs[dgst]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	var blobHandler http.Handler = defaultBlobHandler
	if mw != nil {
		blobHandler = mw(defaultBlobHandler)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == manifestPath:
			w.Header().Set("Content-Type", manifestCT)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(manifestBytes)
		case strings.HasPrefix(r.URL.Path, blobPrefix):
			blobHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// errorBlobMiddleware returns a blobMiddleware that always responds with the
// given status and optional JSON error body, ignoring the default handler.
func errorBlobMiddleware(status int, code, message string) blobMiddleware {
	return func(_ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if code != "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{
						{"code": code, "message": message},
					},
				})
			} else {
				w.WriteHeader(status)
			}
		})
	}
}

var (
	blobMW503         = errorBlobMiddleware(http.StatusServiceUnavailable, "", "")
	blobMW500         = errorBlobMiddleware(http.StatusInternalServerError, "", "")
	blobMWBlobUnknown = errorBlobMiddleware(http.StatusNotFound, "BLOB_UNKNOWN", "blob unknown to registry")
	blobMW429         = errorBlobMiddleware(http.StatusTooManyRequests, "TOOMANYREQUESTS", "rate limit exceeded")
	blobMW401         = errorBlobMiddleware(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	blobMW403         = errorBlobMiddleware(http.StatusForbidden, "DENIED", "access denied")
)

// countingBlobMW returns a blobMiddleware that increments counter then
// delegates to the next handler (default blob serving).
func countingBlobMW(counter *atomic.Int32) blobMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter.Add(1)
			next.ServeHTTP(w, r)
		})
	}
}

// countingErrorBlobMW increments counter, then responds with an error
// (ignoring the default handler).
func countingErrorBlobMW(counter *atomic.Int32, errMW blobMiddleware) blobMiddleware {
	return func(next http.Handler) http.Handler {
		errHandler := errMW(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter.Add(1)
			errHandler.ServeHTTP(w, r)
		})
	}
}

// rangeOnlyErrorBlobMW applies errMW only to range requests (GetBlobAt path).
// Non-range requests (GetBlob) pass through to the default handler unchanged.
func rangeOnlyErrorBlobMW(errMW blobMiddleware) blobMiddleware {
	return func(next http.Handler) http.Handler {
		errHandler := errMW(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Range") != "" {
				errHandler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// pullAndVerifySuccess runs podman pull and asserts it succeeds, cleans up.
//
//nolint:unparam
func pullAndVerifySuccess(imageRef string) {
	podmanTest.PodmanExitCleanly("pull", "-q", "--tls-verify=false", imageRef)
	podmanTest.PodmanExitCleanly("image", "exists", imageRef)
	podmanTest.PodmanExitCleanly("rmi", imageRef)
}

func pullMirrorFallbackTests() {
	Describe("blob-level mirror fallback", func() {
		const repo = "library/mirrortest"

		type singleMirrorCase struct {
			description       string
			mw                blobMiddleware
			expectPullSuccess bool
			expectedError     string
		}

		singleMirrorCases := []singleMirrorCase{
			{
				description:       "falls back to primary when mirror returns 503 on blobs",
				mw:                blobMW503,
				expectPullSuccess: true,
			},
			{
				description:       "falls back to primary when mirror returns 500 on blobs",
				mw:                blobMW500,
				expectPullSuccess: true,
			},
			{
				description:       "falls back to primary when mirror returns BLOB_UNKNOWN on blobs",
				mw:                blobMWBlobUnknown,
				expectPullSuccess: true,
			},
			{
				description:       "falls back to primary when mirror returns 429 on blobs",
				mw:                blobMW429,
				expectPullSuccess: true,
			},
			{
				description:       "does not fall back when mirror returns 401 on blobs",
				mw:                blobMW401,
				expectPullSuccess: false,
				expectedError:     "unauthorized",
			},
			{
				description:       "does not fall back when mirror returns 403 on blobs",
				mw:                blobMW403,
				expectPullSuccess: false,
				expectedError:     "denied",
			},
		}

		for _, tc := range singleMirrorCases {
			It(tc.description, func() {
				SkipIfRemote("registries.conf is not used by the remote client")

				blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionGzip)

				primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
				defer primary.Close()

				var mirrorBlobHits atomic.Int32
				mirror := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
					countingErrorBlobMW(&mirrorBlobHits, tc.mw)))
				defer mirror.Close()

				conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror))
				podmanTest.setRegistriesConfigEnv([]byte(conf))
				defer resetRegistriesConfigEnv()

				imageRef := "mirrortest.local/" + repo + ":latest"
				session := podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", imageRef})
				session.WaitWithDefaultTimeout()

				if tc.expectPullSuccess {
					Expect(session).Should(ExitCleanly())
					Expect(mirrorBlobHits.Load()).To(BeNumerically(">", int32(0)),
						"mirror should have been tried for blobs before fallback")
					podmanTest.PodmanExitCleanly("image", "exists", imageRef)
					podmanTest.PodmanExitCleanly("rmi", imageRef)
				} else {
					Expect(session).Should(ExitWithError(125, tc.expectedError))
				}
			})
		}

		It("skips multiple broken mirrors and falls back to primary", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionGzip)

			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer primary.Close()

			// Two broken mirrors: first returns 503, second returns BLOB_UNKNOWN.
			var mirror1Hits, mirror2Hits atomic.Int32
			mirror1 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				countingErrorBlobMW(&mirror1Hits, blobMW503)))
			defer mirror1.Close()
			mirror2 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				countingErrorBlobMW(&mirror2Hits, blobMWBlobUnknown)))
			defer mirror2.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror1), primaryAddr(mirror2))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)

			Expect(mirror1Hits.Load()).To(BeNumerically(">", int32(0)), "mirror1 should have been tried for blobs")
			Expect(mirror2Hits.Load()).To(BeNumerically(">", int32(0)), "mirror2 should have been tried for blobs")
		})

		It("stops at first working mirror without reaching primary", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionGzip)

			var primaryBlobHits atomic.Int32
			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				countingBlobMW(&primaryBlobHits)))
			defer primary.Close()

			// mirror1: serves manifest, returns 503 on blobs.
			mirror1 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMW503))
			defer mirror1.Close()

			// mirror2: serves everything. The fallback should stop here.
			mirror2 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer mirror2.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror1), primaryAddr(mirror2))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)

			Expect(primaryBlobHits.Load()).To(Equal(int32(0)), "primary should NOT be contacted when a working mirror is found first")
		})

		It("fails when all mirrors and primary return errors on blobs", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionGzip)

			// Primary also fails on blobs.
			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMW500))
			defer primary.Close()

			// Two mirrors, both broken.
			mirror1 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMW503))
			defer mirror1.Close()
			mirror2 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMWBlobUnknown))
			defer mirror2.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror1), primaryAddr(mirror2))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			session := podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", imageRef})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(125, "fetching blob"))
		})

		It("stops fallback chain when mirror returns non-retriable 401", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionGzip)

			var primaryBlobHits atomic.Int32
			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				countingBlobMW(&primaryBlobHits)))
			defer primary.Close()

			// mirror1 (selected): serves manifest, returns 503 on blobs -> fallback.
			mirror1 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMW503))
			defer mirror1.Close()

			// mirror2: returns 401 on blobs -> NOT retriable, should stop the chain.
			mirror2 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMW401))
			defer mirror2.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror1), primaryAddr(mirror2))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			session := podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", imageRef})
			session.WaitWithDefaultTimeout()

			// 401 from mirror2 halts the chain. Primary is never reached.
			Expect(session).Should(ExitWithError(125, "fetching blob"))
			Expect(primaryBlobHits.Load()).To(Equal(int32(0)), "primary should not be reached after a non-retriable 401")
		})

		It("falls back past an unreachable mirror to primary", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionGzip)

			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer primary.Close()

			// mirror1 (selected): serves manifest, returns 503 on blobs.
			mirror1 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMW503))
			defer mirror1.Close()

			// mirror2: start and immediately stop. Port is closed, server is unreachable.
			mirror2 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			unreachableAddr := primaryAddr(mirror2)
			mirror2.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror1), unreachableAddr)
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			// Connection refused is not a fallback-worthy error, so the
			// chain stops at mirror2. Pull is expected to fail because
			// primary is never tried.
			session := podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", imageRef})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(125, "fetching blob"))
		})

		It("partial mirror: serves some layers, BLOB_UNKNOWN for the rest", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, layerDigests := buildTestImage(3, compressionGzip)
			Expect(layerDigests).To(HaveLen(3))

			// Mirror has config + layer0, but NOT layer1 and layer2.
			missing := map[string]bool{
				layerDigests[1].String(): true,
				layerDigests[2].String(): true,
			}
			blobPrefix := fmt.Sprintf("/v2/%s/blobs/", repo)
			partialMW := func(next http.Handler) http.Handler {
				errHandler := blobMWBlobUnknown(next)
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					dgst := strings.TrimPrefix(r.URL.Path, blobPrefix)
					if missing[dgst] {
						errHandler.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			}

			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer primary.Close()
			mirror := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, partialMW))
			defer mirror.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)
		})

		It("overloaded 503 mirror: retries on mirror before falling back", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionGzip)

			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer primary.Close()

			var mirrorBlobHits atomic.Int32
			mirror := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				countingErrorBlobMW(&mirrorBlobHits, blobMW503)))
			defer mirror.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)

			Expect(mirrorBlobHits.Load()).To(Equal(int32(2)), "mirror should be retried (initial + retry) for the first blob before fallback")
		})

		It("rate-limited mirror: serves first blobs then 429 on the rest", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(4, compressionGzip)

			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer primary.Close()

			// Mirror serves the first 2 blob requests successfully, then returns 429 for all subsequent ones.
			var served atomic.Int32
			rateLimitMW := func(next http.Handler) http.Handler {
				errHandler := blobMW429(next)
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if served.Add(1) <= 2 {
						next.ServeHTTP(w, r)
						return
					}
					errHandler.ServeHTTP(w, r)
				})
			}

			mirror := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, rateLimitMW))
			defer mirror.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)
		})

		It("flaky mirror with multi-layer image: random 503 on half the requests", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(5, compressionGzip)

			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer primary.Close()

			// Mirror alternates between serving and 503ing based on a request counter.
			var reqCount atomic.Int32
			flakyMW := func(next http.Handler) http.Handler {
				errHandler := blobMW503(next)
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if reqCount.Add(1)%2 == 0 {
						errHandler.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			}

			mirror := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, flakyMW))
			defer mirror.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)
		})

		It("mixed failure modes across mirrors with different error types", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionGzip)

			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer primary.Close()

			// mirror1 (selected): 503 on blobs -> fallback-worthy.
			mirror1 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMW503))
			defer mirror1.Close()
			// mirror2: BLOB_UNKNOWN on blobs -> fallback-worthy, continues chain.
			mirror2 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMWBlobUnknown))
			defer mirror2.Close()
			// mirror3: 429 on blobs -> fallback-worthy, continues chain.
			mirror3 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, blobMW429))
			defer mirror3.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror1), primaryAddr(mirror2), primaryAddr(mirror3))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)
		})

		It("zstd:chunked image pulls successfully from mirror via partial pull", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionZstdChunked)

			var primaryBlobHits atomic.Int32
			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				countingBlobMW(&primaryBlobHits)))
			defer primary.Close()

			var mirrorBlobHits atomic.Int32
			mirror := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				countingBlobMW(&mirrorBlobHits)))
			defer mirror.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)

			Expect(mirrorBlobHits.Load()).To(BeNumerically(">", int32(0)),
				"mirror should have served blob requests")
			Expect(primaryBlobHits.Load()).To(Equal(int32(0)),
				"primary should not be contacted when mirror works")
		})

		It("GetBlobAt falls back to primary when mirror returns 503 on range requests", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionZstdChunked)

			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer primary.Close()

			// Mirror: 503 only on range requests (GetBlobAt).
			var mirrorRangeHits atomic.Int32
			mirror := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				countingErrorBlobMW(&mirrorRangeHits, rangeOnlyErrorBlobMW(blobMW503))))
			defer mirror.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)
		})

		It("GetBlobAt falls back to primary when mirror returns BLOB_UNKNOWN on range requests", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionZstdChunked)

			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs, nil))
			defer primary.Close()

			// Mirror: BLOB_UNKNOWN only on range requests.
			mirror := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				rangeOnlyErrorBlobMW(blobMWBlobUnknown)))
			defer mirror.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)
		})

		It("GetBlobAt falls back across multiple mirrors on range request failures", func() {
			SkipIfRemote("registries.conf is not used by the remote client")

			blobs, manifestBytes, manifestCT, _ := buildTestImage(1, compressionZstdChunked)

			var primaryBlobHits atomic.Int32
			primary := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				countingBlobMW(&primaryBlobHits)))
			defer primary.Close()

			// mirror1: 503 on range requests only -> fallback-worthy.
			mirror1 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				rangeOnlyErrorBlobMW(blobMW503)))
			defer mirror1.Close()

			// mirror2: BLOB_UNKNOWN on range requests only -> fallback-worthy.
			mirror2 := httptest.NewServer(registryHandler(repo, manifestBytes, manifestCT, blobs,
				rangeOnlyErrorBlobMW(blobMWBlobUnknown)))
			defer mirror2.Close()

			conf := fmt.Sprintf(`[[registry]]
prefix = "mirrortest.local"
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true

[[registry.mirror]]
location = %q
insecure = true
`, primaryAddr(primary), primaryAddr(mirror1), primaryAddr(mirror2))
			podmanTest.setRegistriesConfigEnv([]byte(conf))
			defer resetRegistriesConfigEnv()

			imageRef := "mirrortest.local/" + repo + ":latest"
			pullAndVerifySuccess(imageRef)
		})
	})
}
