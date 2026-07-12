//go:build linux || freebsd

package integration

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/image/v5/signature/simplesequoia"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/archive"
	"go.podman.io/storage/pkg/fileutils"
)

// testSequoiaKeyFingerprint is a fingerprint of a test Sequoia key in testdata.
const testSequoiaKeyFingerprint = "50DDE898DF4E48755C8C2B7AF6F908B6FA48A229"

var _ = Describe("Podman push", func() {
	BeforeEach(func() {
		podmanTest.AddImageToRWStore(ALPINE)
	})

	It("podman push to containers/storage", func() {
		SkipIfRemote("Remote push does not support containers-storage transport")
		session := podmanTest.Podman([]string{"push", "-q", ALPINE, "containers-storage:busybox:test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman push to dir", func() {
		SkipIfRemote("Remote push does not support dir transport")
		bbdir := filepath.Join(podmanTest.TempDir, "busybox")
		session := podmanTest.Podman([]string{
			"push", "-q", "--remove-signatures", ALPINE,
			fmt.Sprintf("dir:%s", bbdir),
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		bbdir = filepath.Join(podmanTest.TempDir, "busybox")
		session = podmanTest.Podman([]string{
			"push", "-q", "--format", "oci", ALPINE,
			fmt.Sprintf("dir:%s", bbdir),
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman push to oci with compression-format and compression-level", func() {
		SkipIfRemote("Remote push does not support dir transport")
		bbdir := filepath.Join(podmanTest.TempDir, "busybox-oci")

		// Invalid compression format specified, it must fail
		session := podmanTest.Podman([]string{"push", "-q", "--compression-format=gzip", "--compression-level=40", ALPINE, fmt.Sprintf("oci:%s", bbdir)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "writing blob: happened during read: gzip: invalid compression level: 40"))

		session = podmanTest.Podman([]string{
			"push", "-q", "--compression-format=zstd", "--remove-signatures", ALPINE,
			fmt.Sprintf("oci:%s", bbdir),
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		foundZstdFile := false

		blobsDir := filepath.Join(bbdir, "blobs/sha256")

		blobs, err := os.ReadDir(blobsDir)
		Expect(err).ToNot(HaveOccurred())

		for _, f := range blobs {
			blobPath := filepath.Join(blobsDir, f.Name())

			sourceFile, err := os.ReadFile(blobPath)
			Expect(err).ToNot(HaveOccurred())

			compressionType := archive.DetectCompression(sourceFile)
			if compressionType == archive.Zstd {
				foundZstdFile = true
				break
			}
		}
		Expect(foundZstdFile).To(BeTrue(), "found zstd file")
	})

	It("push test --force-compression", func() {
		if runtime.GOARCH == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		if isRootless() {
			err := podmanTest.RestoreArtifact(REGISTRY_IMAGE)
			Expect(err).ToNot(HaveOccurred())
		}
		lock := GetPortLock("5002")
		defer lock.Unlock()
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5002:5000", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		session = podmanTest.Podman([]string{"build", "-t", "imageone", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--force-compression=true", "--compression-format", "gzip", "--remove-signatures", "imageone", "localhost:5002/image"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())

		skopeoInspect := []string{"inspect", "--tls-verify=false", "--raw", "docker://localhost:5002/image:latest"}
		skopeo := SystemExec("skopeo", skopeoInspect)
		skopeo.WaitWithDefaultTimeout()
		Expect(skopeo).Should(ExitCleanly())
		output := skopeo.OutputToString()
		Expect(output).ToNot(ContainSubstring("zstd"))

		push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--force-compression=false", "--compression-format", "zstd", "--remove-signatures", "imageone", "localhost:5002/image"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())

		skopeo = SystemExec("skopeo", skopeoInspect)
		skopeo.WaitWithDefaultTimeout()
		Expect(skopeo).Should(ExitCleanly())
		output = skopeo.OutputToString()
		// Although `--compression-format` is `zstd` but still no traces of `zstd` should be in image
		// since blobs must be reused from last `gzip` image.
		Expect(output).ToNot(ContainSubstring("zstd"))

		push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--compression-format", "zstd", "--force-compression", "--remove-signatures", "imageone", "localhost:5002/image"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())

		skopeo = SystemExec("skopeo", skopeoInspect)
		skopeo.WaitWithDefaultTimeout()
		Expect(skopeo).Should(ExitCleanly())
		output = skopeo.OutputToString()
		// Should contain `zstd` layer, substring `zstd` is enough to confirm in skopeo inspect output that `zstd` layer is present.
		Expect(output).To(ContainSubstring("zstd"))
	})

	It("push test --force-compression --format=v2s2", func() {
		if runtime.GOARCH == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		if isRootless() {
			err := podmanTest.RestoreArtifact(REGISTRY_IMAGE)
			Expect(err).ToNot(HaveOccurred())
		}
		lock := GetPortLock("5005")
		defer lock.Unlock()
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5005:5000", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		session = podmanTest.Podman([]string{"build", "-t", "imageone", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--force-compression=true", "--compression-format", "gzip", "--format", "v2s2", "--remove-signatures", "imageone", "localhost:5005/image"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())

		skopeoInspect := []string{"inspect", "--tls-verify=false", "--raw", "docker://localhost:5005/image:latest"}
		skopeo := SystemExec("skopeo", skopeoInspect)
		skopeo.WaitWithDefaultTimeout()
		Expect(skopeo).Should(ExitCleanly())
		output := skopeo.OutputToString()
		Expect(output).To(ContainSubstring("gzip"))

		push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--force-compression=true", "--compression-format", "zstd", "--format", "v2s2", "--remove-signatures", "imageone", "localhost:5005/image"})
		push.WaitWithDefaultTimeout()
		// the command is asking for an impossible thing; per containers/common#1869, we should detect
		// that and fail with a precise error, but that does not exist, so we end up reporting this
		// misleading text. When that is resolved, this test will need updating.
		Expect(push).Should(ExitWithError(125, "cannot use ForceCompressionFormat with undefined default compression format"))
	})

	It("podman push to local registry", func() {
		if runtime.GOARCH == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		if isRootless() {
			err := podmanTest.RestoreArtifact(REGISTRY_IMAGE)
			Expect(err).ToNot(HaveOccurred())
		}
		lock := GetPortLock("5003")
		defer lock.Unlock()
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5003:5000", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5003/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())

		push = podmanTest.Podman([]string{"push", "--compression-format=gzip", "--compression-level=1", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5003/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		output := push.ErrorToString()
		Expect(output).To(ContainSubstring("Copying blob "))
		Expect(output).To(ContainSubstring("Copying config "))
		Expect(output).To(ContainSubstring("Writing manifest to image destination"))

		bitSize := 1024
		keyFileName := filepath.Join(podmanTest.TempDir, "key")
		publicKeyFileName, _, err := WriteRSAKeyPair(keyFileName, bitSize)
		Expect(err).ToNot(HaveOccurred())

		if !IsRemote() { // Remote does not support --encryption-key
			// Explicitly specify compression-format because encryption and zstd:chunked together triggers a warning:
			//	Compression using zstd:chunked is not beneficial for encrypted layers, using plain zstd instead
			push = podmanTest.Podman([]string{"push", "-q", "--encryption-key", "jwe:" + publicKeyFileName, "--tls-verify=false", "--remove-signatures", "--compression-format=zstd", ALPINE, "localhost:5003/my-alpine"})
			push.WaitWithDefaultTimeout()
			Expect(push).Should(ExitCleanly())
		}

		// Test --digestfile option
		digestFile := filepath.Join(podmanTest.TempDir, "digestfile.txt")
		push2 := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--digestfile=" + digestFile, "--remove-signatures", ALPINE, "localhost:5003/my-alpine"})
		push2.WaitWithDefaultTimeout()
		fi, err := os.Lstat(digestFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(fi.Name()).To(Equal("digestfile.txt"))
		Expect(push2).Should(ExitCleanly())

		if !IsRemote() { // Remote does not support signing
			// Ideally, this should set SystemContext.RegistriesDirPath, but Podman currently doesn’t
			// expose that as an option. So, for now, modify /etc/directly, and skip testing sigstore if
			// we don’t have permission to do so.
			lookasideDir, err := filepath.Abs(filepath.Join(podmanTest.TempDir, "test-lookaside"))
			Expect(err).ToNot(HaveOccurred())
			systemRegistriesDAddition := "/etc/containers/registries.d/podman-test-only-temporary-addition.yaml"
			registriesDFragment, err := os.ReadFile("testdata/sigstore-registries.d-fragment.yaml")
			Expect(err).ToNot(HaveOccurred())
			registriesDFragment = bytes.ReplaceAll(registriesDFragment, []byte("@lookasideDir@"), []byte(lookasideDir))
			err = os.WriteFile(systemRegistriesDAddition, registriesDFragment, 0o644)
			if err != nil {
				GinkgoWriter.Printf("Skipping sigstore tests because /etc/containers/registries.d isn’t writable: %s\n", err)
			} else {
				By("pushing and pulling with --sign-by-sigstore-private-key")
				defer func() {
					err := os.Remove(systemRegistriesDAddition)
					Expect(err).ToNot(HaveOccurred())
				}()
				// Generate a signature verification policy file
				policyPath := generatePolicyFile(podmanTest.TempDir, 5003, "testdata/sequoia-key.pub")
				defer os.Remove(policyPath)

				// Verify that the policy rejects unsigned images
				push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5003/sigstore-signed"})
				push.WaitWithDefaultTimeout()
				Expect(push).Should(ExitCleanly())

				pull := podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5003/sigstore-signed"})
				pull.WaitWithDefaultTimeout()
				Expect(pull).To(ExitWithError(125, "A signature was required, but no signature exists"))

				// Sign an image, and verify it is accepted.
				push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", "--sign-by-sigstore-private-key", "testdata/sigstore-key.key", "--sign-passphrase-file", "testdata/sigstore-key.key.pass", ALPINE, "localhost:5003/sigstore-signed"})
				push.WaitWithDefaultTimeout()
				Expect(push).Should(ExitCleanly())

				pull = podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5003/sigstore-signed"})
				pull.WaitWithDefaultTimeout()
				Expect(pull).Should(ExitCleanly())

				By("pushing and pulling with --sign-by-sigstore")
				// Verify that the policy rejects unsigned images
				push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5003/sigstore-signed-params"})
				push.WaitWithDefaultTimeout()
				Expect(push).Should(ExitCleanly())

				pull = podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5003/sigstore-signed-params"})
				pull.WaitWithDefaultTimeout()
				Expect(pull).To(ExitWithError(125, "A signature was required, but no signature exists"))

				// Sign an image, and verify it is accepted.
				push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", "--sign-by-sigstore", "testdata/sigstore-signing-params.yaml", ALPINE, "localhost:5003/sigstore-signed-params"})
				push.WaitWithDefaultTimeout()
				Expect(push).Should(ExitCleanly())

				pull = podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5003/sigstore-signed-params"})
				pull.WaitWithDefaultTimeout()
				Expect(pull).Should(ExitCleanly())

				signer, err := simplesequoia.NewSigner(
					simplesequoia.WithSequoiaHome("testdata"),
					simplesequoia.WithKeyFingerprint(testSequoiaKeyFingerprint),
				)
				if err != nil {
					GinkgoWriter.Printf("Skipping Sequoia tests because simplesequoia.NewSigner failed: %s\n", err)
				} else {
					signer.Close()

					By("pushing and pulling with --sign-by-sq-fingerprint")
					absSequoiaHome, err := filepath.Abs("testdata")
					Expect(err).ToNot(HaveOccurred())
					defer os.Unsetenv("SEQUOIA_HOME")
					os.Setenv("SEQUOIA_HOME", absSequoiaHome)

					// Verify that the policy rejects unsigned images
					push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5003/simple-sq-signed"})
					push.WaitWithDefaultTimeout()
					Expect(push).Should(ExitCleanly())

					pull = podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5003/simple-sq-signed"})
					pull.WaitWithDefaultTimeout()
					Expect(pull).To(ExitWithError(125, "A signature was required, but no signature exists"))

					// Sign an image, and verify it is accepted.
					push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", "--sign-by-sq-fingerprint", testSequoiaKeyFingerprint, ALPINE, "localhost:5003/simple-sq-signed"})
					push.WaitWithDefaultTimeout()
					Expect(push).Should(ExitCleanly())

					pull = podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5003/simple-sq-signed"})
					pull.WaitWithDefaultTimeout()
					Expect(pull).Should(ExitCleanly())
				}
			}
		}
	})

	It("podman push from local storage with nothing-allowed signature policy", func() {
		SkipIfRemote("Remote push does not support dir transport")
		denyAllPolicy := filepath.Join(INTEGRATION_ROOT, "test/deny.json")

		inspect := podmanTest.Podman([]string{"inspect", "--format={{.ID}}", ALPINE})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		imageID := inspect.OutputToString()

		// FIXME FIXME
		push := podmanTest.Podman([]string{"push", "--signature-policy", denyAllPolicy, "-q", imageID, "dir:" + filepath.Join(podmanTest.TempDir, imageID)})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())
	})

	It("podman push to local registry with authorization", func() {
		SkipIfRootless("/etc/containers/certs.d not writable")
		if runtime.GOARCH == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		authPath := filepath.Join(podmanTest.TempDir, "auth")
		err = os.Mkdir(authPath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		err = os.MkdirAll("/etc/containers/certs.d/localhost:5004", os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll("/etc/containers/certs.d/localhost:5004")

		cwd, _ := os.Getwd()
		certPath := filepath.Join(cwd, "../", "certs")

		lock := GetPortLock("5004")
		defer lock.Unlock()

		f, err := os.Create(filepath.Join(authPath, "htpasswd"))
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		_, err = f.WriteString(htpasswdLine)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{
			"run", "-d", "-p", "5004:5000", "--name", "registry", "-v",
			strings.Join([]string{authPath, "/auth", "z"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs", "z"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", REGISTRY_IMAGE,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(WaitContainerReady(podmanTest, "registry", "listening on", 20, 1)).To(BeTrue(), "registry container ready")

		push := podmanTest.Podman([]string{"push", "--tls-verify=true", "--format=v2s2", "--creds=" + registryUser + ":" + registryPassword, ALPINE, "localhost:5004/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError(125, "x509: certificate signed by unknown authority"))

		push = podmanTest.Podman([]string{"push", "--creds=" + registryUser + ":" + registryPassword, "--tls-verify=false", ALPINE, "localhost:5004/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		Expect(push.ErrorToString()).To(ContainSubstring("Writing manifest to image destination"))

		_, err = fileutils.CopyFile(filepath.Join(certPath, "domain.crt"), "/etc/containers/certs.d/localhost:5004/ca.crt")
		Expect(err).ToNot(HaveOccurred())

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:wrongpasswd", ALPINE, "localhost:5004/credstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError(125, "/credstest: authentication required"))

		if !IsRemote() {
			// remote does not support --cert-dir
			push = podmanTest.Podman([]string{"push", "--tls-verify=true", "--creds=" + registryUser + ":" + registryPassword, "--cert-dir=fakedir", ALPINE, "localhost:5004/certdirtest"})
			push.WaitWithDefaultTimeout()
			Expect(push).To(ExitWithError(125, "x509: certificate signed by unknown authority"))
		}

		push = podmanTest.Podman([]string{"push", "--creds=" + registryUser + ":" + registryPassword, ALPINE, "localhost:5004/defaultflags"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		Expect(push.ErrorToString()).To(ContainSubstring("Writing manifest to image destination"))

		// create and push manifest
		session = podmanTest.Podman([]string{"manifest", "create", "localhost:5004/manifesttest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"manifest", "push", "--creds=" + registryUser + ":" + registryPassword, "--tls-verify=false", "--all", "localhost:5004/manifesttest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring("Writing manifest list to image destination"))
	})

	It("podman push and encrypt to oci", func() {
		SkipIfRemote("Remote push neither supports oci transport, nor encryption")

		bbdir := filepath.Join(podmanTest.TempDir, "busybox-oci")

		bitSize := 1024
		keyFileName := filepath.Join(podmanTest.TempDir, "key")
		publicKeyFileName, _, err := WriteRSAKeyPair(keyFileName, bitSize)
		Expect(err).ToNot(HaveOccurred())

		// Explicitly specify compression-format because encryption and zstd:chunked together triggers a warning:
		//	Compression using zstd:chunked is not beneficial for encrypted layers, using plain zstd instead
		session := podmanTest.Podman([]string{"push", "-q", "--encryption-key", "jwe:" + publicKeyFileName, "--compression-format=zstd", ALPINE, fmt.Sprintf("oci:%s", bbdir)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman push to docker-archive", func() {
		SkipIfRemote("Remote push does not support docker-archive transport")
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.Podman([]string{
			"push", "-q", ALPINE,
			fmt.Sprintf("docker-archive:%s:latest", tarfn),
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman push to docker daemon", func() {
		SkipIfRemote("Remote push does not support docker-daemon transport")
		SkipIfRootless("rootless user has no permission to use default docker.sock")
		setup := SystemExec("bash", []string{"-c", "systemctl status docker 2>&1"})

		if setup.LineInOutputContains("Active: inactive") {
			setup = SystemExec("systemctl", []string{"start", "docker"})
			Expect(setup).Should(ExitCleanly())
			defer func() {
				stop := SystemExec("systemctl", []string{"stop", "docker"})
				Expect(stop).Should(Exit(0))
			}()
		} else if setup.ExitCode() != 0 {
			Skip("Docker is not available")
		}

		session := podmanTest.Podman([]string{"push", "-q", ALPINE, "docker-daemon:alpine:podmantest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := SystemExec("docker", []string{"images", "--format", "{{.Repository}}:{{.Tag}}"})
		Expect(check).Should(ExitCleanly())
		Expect(check.OutputToString()).To(ContainSubstring("alpine:podmantest"))

		clean := SystemExec("docker", []string{"rmi", "alpine:podmantest"})
		Expect(clean).Should(ExitCleanly())
	})

	It("podman push to oci-archive", func() {
		SkipIfRemote("Remote push does not support oci-archive transport")
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.Podman([]string{
			"push", "-q", ALPINE,
			fmt.Sprintf("oci-archive:%s:latest", tarfn),
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman push to docker-archive no reference", func() {
		SkipIfRemote("Remote push does not support docker-archive transport")
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.Podman([]string{
			"push", "-q", ALPINE,
			fmt.Sprintf("docker-archive:%s", tarfn),
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman push to oci-archive no reference", func() {
		SkipIfRemote("Remote push does not support oci-archive transport")
		ociarc := filepath.Join(podmanTest.TempDir, "alp-oci")
		session := podmanTest.Podman([]string{
			"push", "-q", ALPINE,
			fmt.Sprintf("oci-archive:%s", ociarc),
		})

		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})
})
