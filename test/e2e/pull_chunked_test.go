//go:build linux || freebsd

package integration

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	. "go.podman.io/podman/v6/test/utils"
)

func pullChunkedTests() { // included in pull_test.go, must use a Ginkgo DSL at the top level
	// This must use a Serial decorator because it uses (podman system reset),
	// affecting all other concurrent Podman runners.  Plausibly we could just delete all images/layers
	// from the test-private store, but we also need to delete BlobInfoCache, and that
	// is currently not private to each test run.
	Describe("podman pull chunked images", Serial, func() {
		// We collect the detailed output of commands, and try to only print it on failure.
		// This is, nominally, a built-in feature of Ginkgo, but we run the tests with -vv, making the
		// full output always captured in a log. So, here, we need to conditionalize explicitly.
		var lastPullOutput bytes.Buffer
		ReportAfterEach(func(_ SpecContext, report SpecReport) {
			if report.Failed() {
				AddReportEntry("last pull operation", lastPullOutput.String())
			}
		})

		It("uses config-based image IDs and enforces DiffID matching", func() {
			if podmanTest.ImageCacheFS == "vfs" {
				Skip("VFS does not support chunked pulls") // We could still test that we enforce DiffID correctness.
			}
			if os.Getenv("CI_DESIRED_COMPOSEFS") != "" {
				// With convert_images, we use the partial pulls for all layers, and do not exercise the full pull code path.
				Skip("The composefs configuration (with convert_images = true) interferes with this test's image ID expectations")
			}
			SkipIfRemote("(podman system reset) is required for this test")
			if isRootless() {
				err := podmanTest.RestoreArtifact(REGISTRY_IMAGE)
				Expect(err).ToNot(HaveOccurred())
			}
			lock := GetPortLock(pullChunkedRegistryPort)
			defer lock.Unlock()

			pullChunkedRegistryPrefix := "docker://localhost:" + pullChunkedRegistryPort + "/"

			imageDir := filepath.Join(tempdir, "images")
			err := os.MkdirAll(imageDir, 0o700)
			Expect(err).NotTo(HaveOccurred())

			var chunkedNormal, chunkedMismatch, chunkedMissing, chunkedEmpty, nonchunkedNormal,
				nonchunkedMismatch, nonchunkedMissing, nonchunkedEmpty,
				schema1 *pullChunkedTestImage
			By("Preparing test images", func() {
				pullChunkedStartRegistry()
				chunkedNormal = &pullChunkedTestImage{
					registryRef: pullChunkedRegistryPrefix + "chunked-normal",
					dirPath:     filepath.Join(imageDir, "chunked-normal"),
				}
				chunkedNormalContentPath := "chunked-normal-image-content"
				err := os.WriteFile(filepath.Join(podmanTest.TempDir, chunkedNormalContentPath), fmt.Appendf(nil, "content-%d", rand.Int64()), 0o600)
				Expect(err).NotTo(HaveOccurred())
				chunkedNormalContainerFile := fmt.Sprintf("FROM scratch\nADD %s /content", chunkedNormalContentPath)
				podmanTest.BuildImage(chunkedNormalContainerFile, chunkedNormal.localTag(), "true")
				podmanTest.PodmanExitCleanly("push", "-q", "--tls-verify=false", "--force-compression", "--compression-format=zstd:chunked", chunkedNormal.localTag(), chunkedNormal.registryRef)
				skopeo := SystemExec("skopeo", []string{"copy", "-q", "--preserve-digests", "--all", "--src-tls-verify=false", chunkedNormal.registryRef, "dir:" + chunkedNormal.dirPath})
				skopeo.WaitWithDefaultTimeout()
				Expect(skopeo).Should(ExitCleanly())
				jq := SystemExec("jq", []string{"-r", ".config.digest", filepath.Join(chunkedNormal.dirPath, "manifest.json")})
				jq.WaitWithDefaultTimeout()
				Expect(jq).Should(ExitCleanly())
				cd, err := digest.Parse(jq.OutputToString())
				Expect(err).NotTo(HaveOccurred())
				chunkedNormal.configDigest = cd

				schema1 = &pullChunkedTestImage{
					registryRef:  pullChunkedRegistryPrefix + "schema1",
					dirPath:      filepath.Join(imageDir, "schema1"),
					configDigest: "",
				}
				skopeo = SystemExec("skopeo", []string{"copy", "-q", "--format=v2s1", "--dest-compress=true", "--dest-compress-format=gzip", "dir:" + chunkedNormal.dirPath, "dir:" + schema1.dirPath})
				skopeo.WaitWithDefaultTimeout()
				Expect(skopeo).Should(ExitCleanly())

				createChunkedImage := func(name string, editDiffIDs func([]digest.Digest) []digest.Digest) *pullChunkedTestImage {
					name = "chunked-" + name
					res := pullChunkedTestImage{
						registryRef: pullChunkedRegistryPrefix + name,
						dirPath:     filepath.Join(imageDir, name),
					}
					cmd := SystemExec("cp", []string{"-a", chunkedNormal.dirPath, res.dirPath})
					cmd.WaitWithDefaultTimeout()
					Expect(cmd).Should(ExitCleanly())

					configBytes, err := os.ReadFile(filepath.Join(chunkedNormal.dirPath, chunkedNormal.configDigest.Encoded()))
					Expect(err).NotTo(HaveOccurred())
					configBytes = editJSON(configBytes, func(config *imgspecv1.Image) {
						config.RootFS.DiffIDs = editDiffIDs(config.RootFS.DiffIDs)
					})
					res.configDigest = digest.FromBytes(configBytes)
					err = os.WriteFile(filepath.Join(res.dirPath, res.configDigest.Encoded()), configBytes, 0o600)
					Expect(err).NotTo(HaveOccurred())

					manifestBytes, err := os.ReadFile(filepath.Join(chunkedNormal.dirPath, "manifest.json"))
					Expect(err).NotTo(HaveOccurred())
					manifestBytes = editJSON(manifestBytes, func(manifest *imgspecv1.Manifest) {
						manifest.Config.Digest = res.configDigest
						manifest.Config.Size = int64(len(configBytes))
					})
					err = os.WriteFile(filepath.Join(res.dirPath, "manifest.json"), manifestBytes, 0o600)
					Expect(err).NotTo(HaveOccurred())

					return &res
				}
				createNonchunkedImage := func(name string, input *pullChunkedTestImage) *pullChunkedTestImage {
					name = "nonchunked-" + name
					res := pullChunkedTestImage{
						registryRef:  pullChunkedRegistryPrefix + name,
						dirPath:      filepath.Join(imageDir, name),
						configDigest: input.configDigest,
					}
					cmd := SystemExec("cp", []string{"-a", input.dirPath, res.dirPath})
					cmd.WaitWithDefaultTimeout()
					Expect(cmd).Should(ExitCleanly())

					manifestBytes, err := os.ReadFile(filepath.Join(input.dirPath, "manifest.json"))
					Expect(err).NotTo(HaveOccurred())
					manifestBytes = editJSON(manifestBytes, func(manifest *imgspecv1.Manifest) {
						manifest.Layers = slices.Clone(manifest.Layers)
						for i := range manifest.Layers {
							delete(manifest.Layers[i].Annotations, "io.github.containers.zstd-chunked.manifest-checksum")
						}
					})
					err = os.WriteFile(filepath.Join(res.dirPath, "manifest.json"), manifestBytes, 0o600)
					Expect(err).NotTo(HaveOccurred())

					return &res
				}
				chunkedMismatch = createChunkedImage("mismatch", func(diffIDs []digest.Digest) []digest.Digest {
					modified := slices.Clone(diffIDs)
					digestBytes, err := hex.DecodeString(diffIDs[0].Encoded())
					Expect(err).NotTo(HaveOccurred())
					digestBytes[len(digestBytes)-1] ^= 1
					modified[0] = digest.NewDigestFromEncoded(diffIDs[0].Algorithm(), hex.EncodeToString(digestBytes))
					return modified
				})
				chunkedMissing = createChunkedImage("missing", func(_ []digest.Digest) []digest.Digest {
					return nil
				})
				chunkedEmpty = createChunkedImage("empty", func(diffIDs []digest.Digest) []digest.Digest {
					res := make([]digest.Digest, len(diffIDs))
					for i := range res {
						res[i] = ""
					}
					return res
				})
				nonchunkedNormal = createNonchunkedImage("normal", chunkedNormal)
				nonchunkedMismatch = createNonchunkedImage("mismatch", chunkedMismatch)
				nonchunkedMissing = createNonchunkedImage("missing", chunkedMissing)
				nonchunkedEmpty = createNonchunkedImage("empty", chunkedEmpty)
				pullChunkedStopRegistry()
			})

			// The actual test
			for _, c := range []struct {
				img             *pullChunkedTestImage
				insecureStorage bool
				fresh           pullChunkedExpectation
				reuse           pullChunkedExpectation
				onSuccess       []string
				onFailure       []string
			}{
				// == Pulls of chunked images
				{
					img:   chunkedNormal,
					fresh: pullChunkedExpectation{success: []string{"Created zstd:chunked differ for blob"}}, // Is a partial pull
					reuse: pullChunkedExpectation{success: []string{"Skipping blob .*already present"}},
				},
				{
					img: chunkedMismatch,
					fresh: pullChunkedExpectation{failure: []string{
						"Created zstd:chunked differ for blob", // Is a partial pull
						"partial pull of blob.*uncompressed digest of layer.*is.*config claims",
					}},
					reuse: pullChunkedExpectation{failure: []string{"trying to reuse blob.*layer.*does not match config's DiffID"}},
				},
				{
					img: chunkedMissing,
					fresh: pullChunkedExpectation{success: []string{
						"Failed to retrieve partial blob: DiffID value for layer .* is unknown or explicitly empty", // Partial pull rejected
						"Detected compression format zstd", // Non-partial pull happens
					}},
					reuse: pullChunkedExpectation{success: []string{
						"Not using TOC .* to look for layer reuse: DiffID value for layer .* is unknown or explicitly empty", // Partial pull reuse rejected
						"Skipping blob .*already present", // Non-partial reuse happens
					}},
				},
				{
					img: chunkedEmpty,
					fresh: pullChunkedExpectation{success: []string{
						"Failed to retrieve partial blob: DiffID value for layer .* is unknown or explicitly empty", // Partial pull rejected
						"Detected compression format zstd", // Non-partial pull happens
					}},
					reuse: pullChunkedExpectation{success: []string{
						"Not using TOC .* to look for layer reuse: DiffID value for layer .* is unknown or explicitly empty", // Partial pull reuse rejected
						"Skipping blob .*already present", // Non-partial reuse happens
					}},
				},
				// == Pulls of images without zstd-chunked metadata (although the layer files are actually zstd:chunked, so blob digest match chunkedNormal and trigger reuse)
				{
					img: nonchunkedNormal,
					fresh: pullChunkedExpectation{success: []string{
						"Failed to retrieve partial blob: no TOC found and convert_images is not configured", // Partial pull not possible
						"Detected compression format zstd",                                                   // Non-partial pull happens
					}},
					reuse: pullChunkedExpectation{success: []string{"Skipping blob .*already present"}},
				},
				{
					img: nonchunkedMismatch,
					fresh: pullChunkedExpectation{failure: []string{
						"Failed to retrieve partial blob: no TOC found and convert_images is not configured", // Partial pull not possible
						"Detected compression format zstd",                                                   // Non-partial pull happens
						"writing blob: layer .* does not match config's DiffID",
					}},
					reuse: pullChunkedExpectation{failure: []string{"trying to reuse blob.*layer.*does not match config's DiffID"}},
				},
				{
					img: nonchunkedMissing,
					fresh: pullChunkedExpectation{success: []string{
						"Failed to retrieve partial blob: no TOC found and convert_images is not configured", // Partial pull not possible
						"Detected compression format zstd",                                                   // Non-partial pull happens
					}},
					reuse: pullChunkedExpectation{success: []string{"Skipping blob .*already present"}}, // Non-partial reuse happens
				},
				{
					img: nonchunkedEmpty,
					fresh: pullChunkedExpectation{success: []string{
						"Failed to retrieve partial blob: no TOC found and convert_images is not configured", // Partial pull not possible
						"Detected compression format zstd",                                                   // Non-partial pull happens
					}},
					reuse: pullChunkedExpectation{success: []string{"Skipping blob .*already present"}}, // Non-partial reuse happens
				},
				// == Pulls of chunked images with insecure_allow_unpredictable_image_contents
				// NOTE: This tests current behavior, but we don't promise users that insecure_allow_unpredictable_image_contents is any faster
				// nor that it sets any particular image IDs.
				{
					img:             chunkedNormal,
					insecureStorage: true,
					fresh: pullChunkedExpectation{success: []string{
						"Created zstd:chunked differ for blob", // Is a partial pull
						"Ordinary storage image ID .*; a layer was looked up by TOC, so using image ID .*",
					}},
					reuse: pullChunkedExpectation{success: []string{
						"Skipping blob .*already present",
						"Ordinary storage image ID .*; a layer was looked up by TOC, so using image ID .*",
					}},
				},
				{
					// WARNING: It happens to be the case that with insecure_allow_unpredictable_image_contents , images with non-matching DiffIDs
					// can be pulled in these situations.
					// WE ARE MAKING NO PROMISES THAT THEY WILL WORK. The images are invalid and need to be fixed.
					// Today, in other situations (e.g. after pulling nonchunkedNormal), c/image will know the uncompressed digest despite insecure_allow_unpredictable_image_contents,
					// and reject the image as not matching the config.
					// As implementations change, the conditions when images with invalid DiffIDs will / will not work may also change, without
					// notice.
					img:             chunkedMismatch,
					insecureStorage: true,
					fresh: pullChunkedExpectation{success: []string{
						"Created zstd:chunked differ for blob", // Is a partial pull
						"Ordinary storage image ID .*; a layer was looked up by TOC, so using image ID .*",
					}},
					reuse: pullChunkedExpectation{success: []string{
						"Skipping blob .*already present",
						"Ordinary storage image ID .*; a layer was looked up by TOC, so using image ID .*",
					}},
				},
				{
					img:             chunkedMissing,
					insecureStorage: true,
					fresh: pullChunkedExpectation{success: []string{
						"Failed to retrieve partial blob: DiffID value for layer .* is unknown or explicitly empty", // Partial pull rejected (the storage option does not actually make a difference)
						"Detected compression format zstd", // Non-partial pull happens
					}},
					reuse: pullChunkedExpectation{success: []string{
						"Not using TOC .* to look for layer reuse: DiffID value for layer .* is unknown or explicitly empty", // Partial pull reuse rejected (the storage option does not actually make a difference)
						"Skipping blob .*already present", // Non-partial reuse happens
					}},
				},
				{
					img:             chunkedEmpty,
					insecureStorage: true,
					fresh: pullChunkedExpectation{success: []string{
						"Failed to retrieve partial blob: DiffID value for layer .* is unknown or explicitly empty", // Partial pull rejected (the storage option does not actually make a difference)
						"Detected compression format zstd", // Non-partial pull happens
					}},
					reuse: pullChunkedExpectation{success: []string{
						"Not using TOC .* to look for layer reuse: DiffID value for layer .* is unknown or explicitly empty", // Partial pull reuse rejected (the storage option does not actually make a difference)
						"Skipping blob .*already present", // Non-partial reuse happens
					}},
				},
				// Schema1
				{
					img: schema1,
					fresh: pullChunkedExpectation{success: []string{
						"Failed to retrieve partial blob: no TOC found and convert_images is not configured", // Partial pull not possible
						"Detected compression format gzip",                                                   // Non-partial pull happens
					}},
					reuse: pullChunkedExpectation{success: []string{"Skipping blob .*already present"}},
				},
				// == No tests of estargz images (Podman can’t create them)
			} {
				testDescription := "Testing " + c.img.registryRef
				if c.insecureStorage {
					testDescription += " with insecure config"
				}
				By(testDescription, func() {
					// Do each test with a clean slate: no layer metadata known, no blob info cache.
					// Annoyingly, we have to re-start and re-populate the registry as well.
					podmanTest.PodmanExitCleanly("system", "reset", "-f")

					pullChunkedStartRegistry()
					c.img.push()
					chunkedNormal.push()

					// Test fresh pull
					c.fresh.testPull(c.img, c.insecureStorage, &lastPullOutput)

					podmanTest.PodmanExitCleanly("--pull-option=enable_partial_images=true", fmt.Sprintf("--pull-option=insecure_allow_unpredictable_image_contents=%v", c.insecureStorage),
						"pull", "-q", "--tls-verify=false", chunkedNormal.registryRef)

					// Test pull after chunked layers are already known, to trigger the layer reuse code
					c.reuse.testPull(c.img, c.insecureStorage, &lastPullOutput)

					pullChunkedStopRegistry()
				})
			}
		})
	})
}

const pullChunkedRegistryPort = "5013"

// pullChunkedStartRegistry creates a registry listening at pullChunkedRegistryPort within the current Podman environment.
func pullChunkedStartRegistry() {
	podmanTest.PodmanExitCleanly("run", "-d", "--name", "registry", "--rm", "-p", pullChunkedRegistryPort+":5000", "-e", "REGISTRY_COMPATIBILITY_SCHEMA1_ENABLED=true", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml")
	if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
		Fail("Cannot start docker registry.")
	}
}

// pullChunkedStopRegistry stops a registry started by pullChunkedStartRegistry.
func pullChunkedStopRegistry() {
	podmanTest.PodmanExitCleanly("stop", "registry")
}

// pullChunkedTestImage centralizes data about a single test image in pullChunkedTests.
type pullChunkedTestImage struct {
	registryRef, dirPath string
	configDigest         digest.Digest // "" for a schema1 image
}

// localTag returns the tag used for the image in Podman’s storage (without the docker:// prefix)
func (img *pullChunkedTestImage) localTag() string {
	return strings.TrimPrefix(img.registryRef, "docker://")
}

// push copies the image from dirPath to registryRef.
func (img *pullChunkedTestImage) push() {
	skopeo := SystemExec("skopeo", []string{"copy", "-q", fmt.Sprintf("--preserve-digests=%v", img.configDigest != ""), "--all", "--dest-tls-verify=false", "dir:" + img.dirPath, img.registryRef})
	skopeo.WaitWithDefaultTimeout()
	Expect(skopeo).Should(ExitCleanly())
}

// pullChunkedExpectations records the expected output of a single "podman pull" command.
type pullChunkedExpectation struct {
	success []string // Expected debug log strings; should succeed if != nil
	failure []string // Expected debug log strings; should fail if != nil
}

// testPull performs one pull
// It replaces *lastPullOutput with the output of the current command
func (expectation *pullChunkedExpectation) testPull(image *pullChunkedTestImage, insecureStorage bool, lastPullOutput *bytes.Buffer) {
	lastPullOutput.Reset()
	session := podmanTest.PodmanWithOptions(PodmanExecOptions{
		FullOutputWriter: lastPullOutput,
	}, "--log-level=debug", "--pull-option=enable_partial_images=true", fmt.Sprintf("--pull-option=insecure_allow_unpredictable_image_contents=%v", insecureStorage),
		"pull", "--tls-verify=false", image.registryRef)
	session.WaitWithDefaultTimeout()
	log := session.ErrorToString()
	if expectation.success != nil {
		Expect(session).Should(Exit(0))
		for _, s := range expectation.success {
			Expect(regexp.MatchString(".*"+s+".*", log)).To(BeTrue(), s)
		}

		if image.configDigest != "" && !insecureStorage {
			s2 := podmanTest.PodmanExitCleanly("image", "inspect", "--format={{.ID}}", image.localTag())
			Expect(s2.OutputToString()).Should(Equal(image.configDigest.Encoded()))
		}
	} else {
		Expect(session).Should(Exit(125))
		for _, s := range expectation.failure {
			Expect(regexp.MatchString(".*"+s+".*", log)).To(BeTrue(), s)
		}
	}
}

// editJSON modifies a JSON-formatted input using the provided edit function.
func editJSON[T any](input []byte, edit func(*T)) []byte {
	var value T
	err = json.Unmarshal(input, &value)
	Expect(err).NotTo(HaveOccurred())
	edit(&value)
	res, err := json.Marshal(value)
	Expect(err).NotTo(HaveOccurred())
	return res
}
