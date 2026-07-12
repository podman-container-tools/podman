//go:build linux || freebsd

package integration

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/buildah/define"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman build", func() {
	// Let's first do the most simple build possible to make sure stuff is
	// happy and then clean up after ourselves to make sure that works too.
	It("podman build and remove basic alpine", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"build", "--pull-never", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		iid := session.OutputToStringArray()[len(session.OutputToStringArray())-1]

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"inspect", iid})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect(data[0]).To(HaveField("Os", runtime.GOOS))
		Expect(data[0]).To(HaveField("Architecture", runtime.GOARCH))

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman image prune should clean build cache", Serial, func() {
		// try writing something to persistent cache
		session := podmanTest.Podman([]string{"build", "-f", "build/buildkit-mount/Containerfilecachewrite"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// try reading something from persistent cache
		session = podmanTest.Podman([]string{"build", "-f", "build/buildkit-mount/Containerfilecacheread"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("hello"))

		// Prune build cache
		session = podmanTest.Podman([]string{"image", "prune", "-f", "--build-cache"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		expectedErr := "open '/test/world': No such file or directory"
		// try reading something from persistent cache should fail
		session = podmanTest.Podman([]string{"build", "-f", "build/buildkit-mount/Containerfilecacheread"})
		session.WaitWithDefaultTimeout()
		if IsRemote() {
			// In the case of podman remote the error from build is not being propagated to `stderr` instead it appears
			// on the `stdout` this could be a potential bug in `remote build` which needs to be fixed and visited.
			Expect(session.OutputToString()).To(ContainSubstring(expectedErr))
			Expect(session).Should(ExitWithError(1, "exit status 1"))
		} else {
			Expect(session).Should(ExitWithError(1, expectedErr))
		}
	})

	It("podman build and remove basic alpine with TMPDIR as relative", func() {
		// preserve TMPDIR if it was originally set
		if cacheDir, found := os.LookupEnv("TMPDIR"); found {
			defer os.Setenv("TMPDIR", cacheDir)
			os.Unsetenv("TMPDIR")
		} else {
			defer os.Unsetenv("TMPDIR")
		}
		// Test case described here: https://github.com/containers/buildah/pull/5084
		os.Setenv("TMPDIR", ".")
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"build", "--pull-never", "build/basicrun"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman build with a secret from file", func() {
		image := "secret-test"

		session := podmanTest.PodmanExitCleanly("build", "-f", "build/Containerfile.with-secret", "-t", image, "--secret", "id=mysecret,src=build/secret.txt", "build/")
		Expect(session.OutputToString()).To(ContainSubstring("somesecret"))

		session = podmanTest.PodmanExitCleanly("run", "--rm", image, "sh", "-c", "find / -xdev -name 'podman-build-secret*' -print")
		Expect(session.OutputToString()).To(BeEmpty(), "podman-build-secret path leaked into image")

		podmanTest.PodmanExitCleanly("rmi", image)

		// Test for: https://github.com/containers/podman/issues/25314 - file secrets must reach the server when .dockerignore is '*'.
		image = "e2e-remote-secret-dignore"

		session = podmanTest.PodmanExitCleanly("build", "-f", "build/remote-secret-dockerignore-star/Dockerfile", "-t", image, "--secret", "id=MY_SECRET,type=file,src=build/remote-secret-dockerignore-star/host-secret.txt", "build/remote-secret-dockerignore-star")
		Expect(session.OutputToString()).To(ContainSubstring("Super Secret"))

		session = podmanTest.PodmanExitCleanly("run", "--rm", image, "sh", "-c", "find / -xdev -name 'podman-build-secret*' -print")
		Expect(session.OutputToString()).To(BeEmpty(), "podman-build-secret path leaked into image")

		podmanTest.PodmanExitCleanly("rmi", image)

		// build with multiple secrets from files
		image = "multiple-secret-test"
		session = podmanTest.PodmanExitCleanly("build", "-f", "build/Containerfile.with-multiple-secret", "-t", image, "--secret", "id=mysecret,src=build/secret.txt", "--secret", "id=mysecret2,src=build/anothersecret.txt", "build/")
		Expect(session.OutputToString()).To(ContainSubstring("somesecret"))
		Expect(session.OutputToString()).To(ContainSubstring("anothersecret"))

		session = podmanTest.PodmanExitCleanly("run", "--rm", image, "sh", "-c", "find / -xdev -name 'podman-build-secret*' -print")
		Expect(session.OutputToString()).To(BeEmpty(), "podman-build-secret path leaked into image")

		podmanTest.PodmanExitCleanly("rmi", image)
	})

	It("podman build with a secret from env", func() {
		secret := "somesecretvalue"
		GinkgoT().Setenv("MYSECRET", secret)
		image := "secret-test"

		session := podmanTest.PodmanExitCleanly("build", "-f", "build/Containerfile.with-secret", "-t", image, "--secret", "id=mysecret,env=MYSECRET", "build/")
		Expect(session.OutputToString()).To(ContainSubstring(secret))

		session = podmanTest.PodmanExitCleanly("run", "--rm", image, "sh", "-c", "find / -xdev -name 'podman-build-secret*' -print")
		Expect(session.OutputToString()).To(BeEmpty(), "podman-build-secret path leaked into image")

		podmanTest.PodmanExitCleanly("rmi", image)

		// Test for: https://github.com/containers/podman/issues/28334 - env secrets + COPY must not add host-shaped podman-build-secret paths to the image.
		image = "e2e-remote-secret-copy"

		session = podmanTest.PodmanExitCleanly("build", "-f", "build/remote-secret-copy/Dockerfile", "-t", image, "--secret", "id=mysecret,env=MYSECRET", "build/remote-secret-copy")
		Expect(session.OutputToString()).To(ContainSubstring(secret))

		session = podmanTest.PodmanExitCleanly("run", "--rm", image, "sh", "-c", "find / -xdev -name 'podman-build-secret*' -print")
		Expect(session.OutputToString()).To(BeEmpty(), "podman-build-secret path leaked into image")

		podmanTest.PodmanExitCleanly("rmi", image)
	})

	It("podman build with not found Containerfile or Dockerfile", func() {
		targetPath := filepath.Join(podmanTest.TempDir, "notfound")
		err = os.Mkdir(targetPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			Expect(os.RemoveAll(targetPath)).To(Succeed())
		}()

		session := podmanTest.Podman([]string{"build", targetPath})
		session.WaitWithDefaultTimeout()
		expectStderr := fmt.Sprintf("no Containerfile or Dockerfile specified or found in context directory, %s", targetPath)
		Expect(session).Should(ExitWithError(125, expectStderr))

		session = podmanTest.Podman([]string{"build", "-f", "foo", targetPath})
		session.WaitWithDefaultTimeout()
		expectStderr = fmt.Sprintf("the specified Containerfile or Dockerfile does not exist, %s", "foo")
		Expect(session).Should(ExitWithError(125, expectStderr))
	})

	It("podman build with logfile", func() {
		logfile := filepath.Join(podmanTest.TempDir, "logfile")
		session := podmanTest.Podman([]string{"build", "--pull=never", "--tag", "test", "--logfile", logfile, "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"inspect", "test"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect(data[0]).To(HaveField("Os", runtime.GOOS))
		Expect(data[0]).To(HaveField("Architecture", runtime.GOARCH))

		st, err := os.Stat(logfile)
		Expect(err).ToNot(HaveOccurred())
		Expect(st.Size()).To(Not(Equal(int64(0))))

		session = podmanTest.Podman([]string{"rmi", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	// If the context directory is pointing at a file and not a directory,
	// that's a no no, fail out.
	It("podman build context directory a file", func() {
		session := podmanTest.Podman([]string{"build", "--pull=never", "build/context_dir_a_file"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "context must be a directory:"))
	})

	// Check that builds with different values for the squash options
	// create the appropriate number of layers, then clean up after.
	It("podman build basic alpine with squash", func() {
		session := podmanTest.Podman([]string{"build", "-q", "--pull-never", "-f", "build/squash/Dockerfile.squash-a", "-t", "test-squash-a:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Check for two layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(2))

		session = podmanTest.Podman([]string{"build", "-q", "--pull-never", "-f", "build/squash/Dockerfile.squash-b", "--squash", "-t", "test-squash-b:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-b"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Check for three layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(3))

		session = podmanTest.Podman([]string{"build", "-q", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash", "-t", "test-squash-c:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-c"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Check for two layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(2))

		session = podmanTest.Podman([]string{"build", "-q", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash-all", "-t", "test-squash-d:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Check for one layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(1))

		session = podmanTest.Podman([]string{"rm", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman build verify explicit cache use with squash-all and --layers", func() {
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash-all", "--layers", "-t", "test-squash-d:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Check for one layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(1))

		// Second build must use last squashed build from cache
		session = podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash-all", "--layers", "-t", "test", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Test if entire build is used from cache
		Expect(session.OutputToString()).To(ContainSubstring("Using cache"))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Check for one layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(1))
	})

	It("podman build Containerfile locations", func() {
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(os.TempDir())).To(Succeed())
		defer Expect(os.Chdir(cwd)).To(BeNil())

		fakeFile := filepath.Join(os.TempDir(), "Containerfile")
		Expect(os.WriteFile(fakeFile, fmt.Appendf(nil, "FROM %s", CITEST_IMAGE), 0o755)).To(Succeed())

		targetFile := filepath.Join(podmanTest.TempDir, "Containerfile")
		Expect(os.WriteFile(targetFile, []byte("FROM scratch"), 0o755)).To(Succeed())

		defer func() {
			Expect(os.RemoveAll(fakeFile)).To(Succeed())
			Expect(os.RemoveAll(targetFile)).To(Succeed())
		}()

		// When
		session := podmanTest.Podman([]string{
			"build", "--pull-never", "-f", targetFile, "-t", "test-locations",
		})
		session.WaitWithDefaultTimeout()

		// Then
		Expect(session).Should(ExitCleanly())
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement("scratch"))
	})

	It("podman build basic alpine and print id to external file", func() {
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(os.TempDir())).To(Succeed())
		defer Expect(os.Chdir(cwd)).To(BeNil())

		targetFile := filepath.Join(podmanTest.TempDir, "idFile")

		session := podmanTest.Podman([]string{"build", "--pull-never", "build/basicalpine", "--iidfile", targetFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		id, _ := os.ReadFile(targetFile)

		// Verify that id is correct
		inspect := podmanTest.Podman([]string{"inspect", string(id)})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect("sha256:" + data[0].ID).To(Equal(string(id)))
	})

	It("podman build basic alpine and print id to external file without prefix (--iidfile-raw)", func() {
		targetFile := filepath.Join(podmanTest.TempDir, "idFileRaw")

		session := podmanTest.Podman([]string{"build", "--pull-never", "build/basicalpine", "--iidfile-raw", targetFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		id, _ := os.ReadFile(targetFile)

		// Verify that id is correct (no sha256: prefix)
		inspect := podmanTest.Podman([]string{"inspect", string(id)})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect(data[0].ID).To(Equal(string(id)))
	})

	It("podman Test PATH and reserved annotation in built image", func() {
		path := "/tmp:/bin:/usr/bin:/usr/sbin"
		session := podmanTest.Podman([]string{
			"build", "--annotation", "io.podman.annotations.seccomp=foobar", "--pull-never", "-f", "build/basicalpine/Containerfile.path", "-t", "test-path",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", "foobar", "test-path", "printenv", "PATH"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		stdoutLines := session.OutputToStringArray()
		Expect(stdoutLines[0]).Should(Equal(path))

		// Reserved annotation should not be applied from the image to the container.
		session = podmanTest.Podman([]string{"inspect", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).NotTo(ContainSubstring("io.podman.annotations.seccomp"))
	})

	It("podman build where workdir is a symlink and run without creating new workdir", func() {
		session := podmanTest.Podman([]string{
			"build", "-f", "build/workdir-symlink/Dockerfile", "-t", "test-symlink",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--workdir", "/tmp/link", "test-symlink"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman build http proxy test", func() {
		if env, found := os.LookupEnv("http_proxy"); found {
			defer os.Setenv("http_proxy", env)
		} else {
			defer os.Unsetenv("http_proxy")
		}
		os.Setenv("http_proxy", "1.2.3.4")
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
			// set proxy env again so it will only effect the client
			// the remote client should still use the proxy that was set for the server
			os.Setenv("http_proxy", "127.0.0.2")
		}
		podmanTest.AddImageToRWStore(CITEST_IMAGE)
		dockerfile := fmt.Sprintf(`FROM %s
RUN printenv http_proxy`, CITEST_IMAGE)

		dockerfilePath := filepath.Join(podmanTest.TempDir, "Dockerfile")
		err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o755)
		Expect(err).ToNot(HaveOccurred())
		// --http-proxy should be true by default so we do not set it
		session := podmanTest.Podman([]string{"build", "--pull-never", "--file", dockerfilePath, podmanTest.TempDir})
		session.Wait(120)
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("1.2.3.4"))

		// this tries to use the cache so we explicitly disable it
		session = podmanTest.Podman([]string{"build", "--no-cache", "--pull-never", "--http-proxy=false", "--file", dockerfilePath, podmanTest.TempDir})
		session.Wait(120)
		Expect(session).Should(ExitWithError(1, `Error: building at STEP "RUN printenv http_proxy"`))
	})

	It("podman build relay exit code to process", func() {
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		}
		podmanTest.AddImageToRWStore(CITEST_IMAGE)
		dockerfile := fmt.Sprintf(`FROM %s
RUN exit 5`, CITEST_IMAGE)

		dockerfilePath := filepath.Join(podmanTest.TempDir, "Dockerfile")
		err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "-t", "error-test", "--file", dockerfilePath, podmanTest.TempDir})
		session.Wait(120)
		Expect(session).Should(ExitWithError(5, `building at STEP "RUN exit 5": while running runtime: exit status 5`))
	})

	It("podman build and check identity", func() {
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/basicalpine/Containerfile.path", "--no-cache", "-t", "test", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ index .Config.Labels }}", "test"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.OutputToString()
		Expect(data).To(ContainSubstring(define.Version))
	})

	It("podman build and check identity with always", func() {
		// with --pull=always
		session := podmanTest.Podman([]string{"build", "-q", "--pull=always", "-f", "build/basicalpine/Containerfile.path", "--no-cache", "-t", "test1", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ index .Config.Labels }}", "test1"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.OutputToString()
		Expect(data).To(ContainSubstring(define.Version))

		// with --pull-always
		session = podmanTest.Podman([]string{"build", "-q", "--pull-always", "-f", "build/basicalpine/Containerfile.path", "--no-cache", "-t", "test2", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Verify that OS and Arch are being set
		inspect = podmanTest.Podman([]string{"image", "inspect", "--format", "{{ index .Config.Labels }}", "test2"})
		inspect.WaitWithDefaultTimeout()
		data = inspect.OutputToString()
		Expect(data).To(ContainSubstring(define.Version))
	})

	It("podman-remote send correct path to copier", func() {
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		}
		// Write target and fake files
		targetSubPath := filepath.Join(podmanTest.TempDir, "emptydir")
		if _, err = os.Stat(targetSubPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				err = os.Mkdir(targetSubPath, 0o755)
				Expect(err).ToNot(HaveOccurred())
			}
		}

		containerfile := fmt.Sprintf(`FROM %s
COPY /emptydir/* /dir`, CITEST_IMAGE)

		containerfilePath := filepath.Join(podmanTest.TempDir, "ContainerfilePathToCopier")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0o644)
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(containerfilePath)

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "-f", "ContainerfilePathToCopier", podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		// NOTE: Docker and buildah both should error when `COPY /* /dir` is done on emptydir
		// as context. However buildkit simply ignores this so when buildah also starts ignoring
		// for such case edit this test to return 0 and check that no `/dir` should be in the result.
		Expect(session).Should(ExitWithError(125, "can't make relative to"))
	})

	It("podman remote test container/docker file is not inside context dir", func() {
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		// Write target and fake files
		targetPath := podmanTest.TempDir
		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		dummyFile := filepath.Join(targetSubPath, "dummy")
		err = os.WriteFile(dummyFile, []byte("dummy"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		containerfile := fmt.Sprintf(`FROM %s
ADD . /test
RUN find /test`, CITEST_IMAGE)

		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0o644)
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			Expect(os.Chdir(cwd)).To(Succeed())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(Succeed())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "-f", "Containerfile", targetSubPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/test/dummy"))
	})

	It("podman remote build must not allow symlink for ignore files", func() {
		// Create a random file where symlink must be resolved
		// but build should not be able to access it.
		privateFile := filepath.Join(podmanTest.TempDir, "private_file")
		f, err := os.Create(privateFile)
		Expect(err).ToNot(HaveOccurred())
		// Mark hello to be ignored in outerfile, but it should not be ignored.
		_, err = f.WriteString("hello\n")
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		// Create .dockerignore which is a symlink to /tmp/.../private_file outside of the context dir.
		currentDir, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		ignoreFile := filepath.Join(currentDir, "build/containerignore-symlink/.dockerignore")
		err = os.Symlink(privateFile, ignoreFile)
		Expect(err).ToNot(HaveOccurred())
		// Remove created .dockerignore for this test when test ends.
		defer os.Remove(ignoreFile)

		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		} else {
			Skip("Only valid at remote test")
		}

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "build/containerignore-symlink/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "test", "ls", "/dir"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman remote test container/docker file is not at root of context dir", func() {
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		} else {
			Skip("Only valid at remote test")
		}
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(CITEST_IMAGE)

		// Write target and fake files
		targetPath := podmanTest.TempDir
		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		containerfile := fmt.Sprintf("FROM %s", CITEST_IMAGE)

		containerfilePath := filepath.Join(targetSubPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0o644)
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			Expect(os.Chdir(cwd)).To(Succeed())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(Succeed())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "-f", "subdir/Containerfile", "."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman remote test .dockerignore", func() {
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		} else {
			Skip("Only valid at remote test")
		}
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(CITEST_IMAGE)

		// Write target and fake files
		targetPath := filepath.Join(podmanTest.TempDir, "build")
		err = os.Mkdir(targetPath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		containerfile := fmt.Sprintf(`FROM %s
ADD . /testfilter/
RUN find /testfilter/`, CITEST_IMAGE)

		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0o644)
		Expect(err).ToNot(HaveOccurred())

		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		dummyFile1 := filepath.Join(targetPath, "dummy1")
		err = os.WriteFile(dummyFile1, []byte("dummy1"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		dummyFile2 := filepath.Join(targetPath, "dummy2")
		err = os.WriteFile(dummyFile2, []byte("dummy2"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		dummyFile3 := filepath.Join(targetSubPath, "dummy3")
		err = os.WriteFile(dummyFile3, []byte("dummy3"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			Expect(os.Chdir(cwd)).To(Succeed())
			Expect(os.RemoveAll(targetPath)).To(Succeed())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(Succeed())

		dockerignoreContent := `dummy1
subdir**`
		dockerignoreFile := filepath.Join(targetPath, ".dockerignore")

		// test .dockerignore
		By("Test .dockererignore")
		err = os.WriteFile(dockerignoreFile, []byte(dockerignoreContent), 0o644)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"build", "-t", "test", "."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("/testfilter/dummy2"))
		Expect(output).NotTo(ContainSubstring("/testfilter/dummy1"))
		Expect(output).NotTo(ContainSubstring("/testfilter/subdir"))
	})

	// See https://github.com/containers/podman/issues/13535
	It("Remote build .containerignore filtering embedded directory (#13535)", func() {
		SkipIfNotRemote("Testing remote .containerignore file filtering")
		podmanTest.RestartRemoteService()

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(CITEST_IMAGE)

		contents := bytes.Buffer{}
		contents.WriteString("FROM " + CITEST_IMAGE + "\n")
		contents.WriteString("ADD . /testfilter/\n")
		contents.WriteString("RUN find /testfilter/ -print\n")

		containerfile := filepath.Join(tempdir, "Containerfile")
		Expect(os.WriteFile(containerfile, contents.Bytes(), 0o644)).ToNot(HaveOccurred())

		contextDir := filepath.Join(podmanTest.TempDir, "context")
		err = os.MkdirAll(contextDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		Expect(os.WriteFile(filepath.Join(contextDir, "expected"), contents.Bytes(), 0o644)).
			ToNot(HaveOccurred())

		subdirPath := filepath.Join(contextDir, "subdir")
		Expect(os.MkdirAll(subdirPath, 0o755)).ToNot(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(subdirPath, "extra"), contents.Bytes(), 0o644)).
			ToNot(HaveOccurred())
		randomFile := filepath.Join(subdirPath, "randomFile")
		dd := exec.Command("dd", "if=/dev/urandom", "of="+randomFile, "bs=1G", "count=1")
		ddSession, err := Start(dd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(ddSession, "30s", "1s").Should(Exit(0))

		// make cwd as context root path
		Expect(os.Chdir(contextDir)).ToNot(HaveOccurred())
		defer func() {
			err := os.Chdir(cwd)
			Expect(err).ToNot(HaveOccurred())
		}()

		By("Test .containerignore filtering subdirectory")
		err = os.WriteFile(filepath.Join(contextDir, ".containerignore"), []byte(`subdir/`), 0o644)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"build", "-f", containerfile, contextDir})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		output := session.OutputToString()
		Expect(output).To(ContainSubstring("/testfilter/expected"))
		Expect(output).NotTo(ContainSubstring("subdir"))
	})

	It("podman remote test context dir contains empty dirs and symlinks", func() {
		SkipIfNotRemote("Testing remote contextDir empty")
		podmanTest.RestartRemoteService()

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(CITEST_IMAGE)

		// Write target and fake files
		targetPath := podmanTest.TempDir
		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		dummyFile := filepath.Join(targetSubPath, "dummy")
		err = os.WriteFile(dummyFile, []byte("dummy"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		emptyDir := filepath.Join(targetSubPath, "emptyDir")
		err = os.Mkdir(emptyDir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(targetSubPath)).To(Succeed())
		Expect(os.Symlink("dummy", "dummy-symlink")).To(Succeed())

		containerfile := fmt.Sprintf(`FROM %s
ADD . /test
RUN find /test
RUN [[ -L /test/dummy-symlink ]] && echo SYMLNKOK || echo SYMLNKERR`, CITEST_IMAGE)

		containerfilePath := filepath.Join(targetSubPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0o644)
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			Expect(os.Chdir(cwd)).To(Succeed())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(Succeed())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", targetSubPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/test/dummy"))
		Expect(session.OutputToString()).To(ContainSubstring("/test/emptyDir"))
		Expect(session.OutputToString()).To(ContainSubstring("/test/dummy-symlink"))
		Expect(session.OutputToString()).To(ContainSubstring("SYMLNKOK"))
	})

	It("podman build --no-hosts", func() {
		targetPath := podmanTest.TempDir

		containerFile := filepath.Join(targetPath, "Containerfile")
		content := `FROM scratch
RUN echo '56.78.12.34 image.example.com' > /etc/hosts`

		Expect(os.WriteFile(containerFile, []byte(content), 0o755)).To(Succeed())

		defer func() {
			Expect(os.RemoveAll(containerFile)).To(Succeed())
		}()

		session := podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "hosts_test", "--no-hosts", "--from", CITEST_IMAGE, targetPath,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--no-hosts", "--rm", "hosts_test", "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("56.78.12.34 image.example.com"))
	})

	It("podman build --from, --add-host, --cap-drop, --cap-add", func() {
		targetPath := podmanTest.TempDir

		containerFile := filepath.Join(targetPath, "Containerfile")
		content := `FROM scratch
RUN cat /etc/hosts
RUN grep CapEff /proc/self/status`

		Expect(os.WriteFile(containerFile, []byte(content), 0o755)).To(Succeed())

		defer func() {
			Expect(os.RemoveAll(containerFile)).To(Succeed())
		}()

		// When
		session := podmanTest.Podman([]string{
			"build", "--pull-never", "--cap-drop=all", "--cap-add=net_bind_service", "--add-host", "testhost:1.2.3.4", "--from", CITEST_IMAGE, targetPath,
		})
		session.WaitWithDefaultTimeout()

		// Then
		Expect(session).Should(ExitCleanly())
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement(CITEST_IMAGE))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement("testhost"))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement("0000000000000400"))
	})

	It("podman build --isolation && --arch", func() {
		targetPath := podmanTest.TempDir
		containerFile := filepath.Join(targetPath, "Containerfile")
		Expect(os.WriteFile(containerFile, fmt.Appendf(nil, "FROM %s", CITEST_IMAGE), 0o755)).To(Succeed())

		defer func() {
			Expect(os.RemoveAll(containerFile)).To(Succeed())
		}()

		// When
		session := podmanTest.Podman([]string{
			"build", "-q", "--isolation", "oci", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session).Should(ExitCleanly())

		// When
		session = podmanTest.Podman([]string{
			"build", "-q", "--isolation", "chroot", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session).Should(ExitCleanly())

		// When
		session = podmanTest.Podman([]string{
			"build", "-q", "--pull-never", "--isolation", "rootless", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session).Should(ExitCleanly())

		// When
		session = podmanTest.Podman([]string{
			"build", "-q", "--pull-never", "--isolation", "bogus", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session).Should(ExitWithError(125, `unrecognized isolation type "bogus"`))
	})

	It("podman build --timestamp flag", func() {
		containerfile := fmt.Sprintf(`FROM %s
RUN echo hello`, CITEST_IMAGE)

		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0o755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--timestamp", "0", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Created }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("1970-01-01 00:00:00 +0000 UTC"))
	})

	It("podman build --log-rusage", func() {
		targetPath := podmanTest.TempDir

		containerFile := filepath.Join(targetPath, "Containerfile")
		content := `FROM scratch`

		Expect(os.WriteFile(containerFile, []byte(content), 0o755)).To(Succeed())

		session := podmanTest.Podman([]string{"build", "--log-rusage", "--pull-never", targetPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("(system)"))
		Expect(session.OutputToString()).To(ContainSubstring("(user)"))
		Expect(session.OutputToString()).To(ContainSubstring("(elapsed)"))
	})

	It("podman build --arch --os flag", func() {
		containerfile := `FROM scratch`
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0o755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--arch", "foo", "--os", "bar", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Architecture }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("foo"))

		inspect = podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Os }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("bar"))
	})

	It("podman build --os windows flag", func() {
		containerfile := `FROM scratch`
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0o755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--os", "windows", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Architecture }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal(runtime.GOARCH))

		inspect = podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Os }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("windows"))
	})

	It("podman build device test", func() {
		if _, err := os.Lstat("/dev/fuse"); err != nil {
			Skip(fmt.Sprintf("test requires stat /dev/fuse to work: %v", err))
		}
		containerfile := fmt.Sprintf(`FROM %s
RUN ls /dev/fuse`, CITEST_IMAGE)
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0o755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, `building at STEP "RUN ls /dev/fuse": while running runtime: exit status 1`))

		session = podmanTest.Podman([]string{"build", "--pull-never", "--device", "/dev/fuse", "-t", "test", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman build device rename test", func() {
		SkipIfRootless("rootless builds do not currently support renaming devices")
		containerfile := fmt.Sprintf(`FROM %s
RUN ls /dev/test1`, CITEST_IMAGE)
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0o755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, `building at STEP "RUN ls /dev/test1": while running runtime: exit status 1`))

		session = podmanTest.Podman([]string{"build", "--pull-never", "--device", "/dev/zero:/dev/test1", "-t", "test", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman build use absolute path even if given relative", func() {
		containerFile := fmt.Sprintf(`FROM %s`, CITEST_IMAGE)
		relativeDir := filepath.Join(podmanTest.TempDir, "relativeDir")
		containerFilePath := filepath.Join(relativeDir, "Containerfile")
		buildRoot := filepath.Join(relativeDir, "build-root")

		err = os.Mkdir(relativeDir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = os.Mkdir(buildRoot, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(containerFilePath, []byte(containerFile), 0o755)
		Expect(err).ToNot(HaveOccurred())
		build := podmanTest.Podman([]string{"build", "-f", containerFilePath, buildRoot})
		build.WaitWithDefaultTimeout()
		Expect(build).To(ExitCleanly())
	})

	// system reset must run serial: https://github.com/containers/podman/issues/17903
	It("podman system reset must clean host shared cache", Serial, func() {
		SkipIfRemote("podman-remote does not have system reset -f")
		useCustomNetworkDir(podmanTest, tempdir)
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"build", "--pull-never", "--file", "build/cache/Dockerfilecachewrite", "build/cache/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"system", "reset", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"build", "--pull-never", "--file", "build/cache/Dockerfilecacheread", "build/cache/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, `building at STEP "RUN --mount=type=cache,target=/test,z cat /test/world": while running runtime: exit status 1`))
	})

	It("podman build with sbom flags", func() {
		podmanTest.AddImageToRWStore(ALPINE)

		localsbomFile := filepath.Join(podmanTest.TempDir, "localsbom.txt")
		localPurlFile := filepath.Join(podmanTest.TempDir, "localpurl.txt")

		podmanTest.PodmanExitCleanly("build", "-t", "sbom-img", "--sbom-output="+localsbomFile, "--sbom-purl-output="+localPurlFile, "--sbom-image-output=/tmp/sbom.txt", "--sbom-image-purl-output=/tmp/purl.txt",
			"--sbom-scanner-image=alpine", "--sbom-scanner-command=/bin/sh -c 'echo SCANNED ROOT {ROOTFS} > {OUTPUT}'", "--sbom-scanner-command=/bin/sh -c 'echo SCANNED BUILD CONTEXT {CONTEXT} > {OUTPUT}'",
			"--sbom-merge-strategy=cat", "build/basicalpine")

		Expect(localsbomFile).To(BeARegularFile())
		Expect(localPurlFile).To(BeARegularFile())

		session := podmanTest.PodmanExitCleanly("run", "--rm", "sbom-img", "ls", "/tmp")
		Expect(session.OutputToString()).To(ContainSubstring("purl.txt"))
		Expect(session.OutputToString()).To(ContainSubstring("sbom.txt"))
	})

	It("podman build --build-context: local source", func() {
		podmanTest.RestartRemoteService()

		localCtx1 := filepath.Join(podmanTest.TempDir, "context1")
		localCtx2 := filepath.Join(podmanTest.TempDir, "context2")

		Expect(os.MkdirAll(localCtx1, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(localCtx1, "file1.txt"), []byte("Content from context1"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(localCtx1, "config.json"), []byte(`{"source": "context1"}`), 0o644)).To(Succeed())

		Expect(os.MkdirAll(filepath.Join(localCtx2, "subdir"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(localCtx2, "file2.txt"), []byte("Content from context2"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(localCtx2, "subdir", "nested.txt"), []byte("Nested content"), 0o644)).To(Succeed())

		containerfile := `FROM quay.io/libpod/alpine:latest
COPY --from=localctx1 /file1.txt /from-context1.txt
COPY --from=localctx1 /config.json /config1.json`

		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile1")
		Expect(os.WriteFile(containerfilePath, []byte(containerfile), 0o644)).To(Succeed())

		session := podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-local-single",
			"--build-context", fmt.Sprintf("localctx1=%s", localCtx1),
			"-f", containerfilePath, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "test-local-single", "cat", "/from-context1.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("Content from context1"))

		containerfile = `FROM quay.io/libpod/alpine:latest
COPY --from=ctx1 /file1.txt /file1.txt
COPY --from=ctx2 /file2.txt /file2.txt
COPY --from=ctx2 /subdir/nested.txt /nested.txt`

		containerfilePath = filepath.Join(podmanTest.TempDir, "Containerfile2")
		Expect(os.WriteFile(containerfilePath, []byte(containerfile), 0o644)).To(Succeed())

		session = podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-local-multi",
			"--build-context", fmt.Sprintf("ctx1=%s", localCtx1),
			"--build-context", fmt.Sprintf("ctx2=%s", localCtx2),
			"-f", containerfilePath, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "test-local-multi", "cat", "/nested.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("Nested content"))

		mainFile := filepath.Join(podmanTest.TempDir, "main.txt")
		Expect(os.WriteFile(mainFile, []byte("From main context"), 0o644)).To(Succeed())

		containerfile = `FROM quay.io/libpod/alpine:latest
COPY main.txt /main.txt
COPY --from=additional /file1.txt /additional.txt`

		containerfilePath = filepath.Join(podmanTest.TempDir, "Containerfile3")
		Expect(os.WriteFile(containerfilePath, []byte(containerfile), 0o644)).To(Succeed())

		session = podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-local-mixed",
			"--build-context", fmt.Sprintf("additional=%s", localCtx1),
			"-f", containerfilePath, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "test-local-mixed", "cat", "/main.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("From main context"))

		session = podmanTest.Podman([]string{"rmi", "-f", "test-local-single", "test-local-multi", "test-local-mixed"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman build --build-context: URL source", func() {
		podmanTest.RestartRemoteService()

		testRepoURL := "https://github.com/containers/PodmanHello.git"
		testArchiveURL := "https://github.com/containers/PodmanHello/archive/refs/heads/main.tar.gz"

		containerfile := `FROM quay.io/libpod/alpine:latest
COPY --from=urlctx . /url-context/`

		containerfilePath := filepath.Join(podmanTest.TempDir, "ContainerfileURL1")
		Expect(os.WriteFile(containerfilePath, []byte(containerfile), 0o644)).To(Succeed())

		session := podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-url-single",
			"--build-context", fmt.Sprintf("urlctx=%s", testRepoURL),
			"-f", containerfilePath, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "test-url-single", "ls", "/url-context/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("LICENSE"))
		Expect(output).To(ContainSubstring("README.md"))

		containerfile = `FROM quay.io/libpod/alpine:latest
COPY --from=archive . /from-archive/`

		containerfilePath = filepath.Join(podmanTest.TempDir, "ContainerfileURL2")
		Expect(os.WriteFile(containerfilePath, []byte(containerfile), 0o644)).To(Succeed())

		session = podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-archive",
			"--build-context", fmt.Sprintf("archive=%s", testArchiveURL),
			"-f", containerfilePath, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "test-archive", "ls", "/from-archive/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(ContainSubstring("PodmanHello-main"))

		session = podmanTest.Podman([]string{"run", "--rm", "test-archive", "ls", "/from-archive/PodmanHello-main/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(ContainSubstring("LICENSE"))
		Expect(output).To(ContainSubstring("README.md"))

		localCtx := filepath.Join(podmanTest.TempDir, "localcontext")
		Expect(os.MkdirAll(localCtx, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(localCtx, "local.txt"), []byte("Local content"), 0o644)).To(Succeed())

		containerfile = `FROM quay.io/libpod/alpine:latest
COPY --from=urlrepo . /from-url/
COPY --from=localctx /local.txt /local.txt
RUN echo "Combined URL and local contexts" > /combined.txt`

		containerfilePath = filepath.Join(podmanTest.TempDir, "ContainerfileURL3")
		Expect(os.WriteFile(containerfilePath, []byte(containerfile), 0o644)).To(Succeed())

		session = podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-url-mixed",
			"--build-context", fmt.Sprintf("urlrepo=%s", testRepoURL),
			"--build-context", fmt.Sprintf("localctx=%s", localCtx),
			"-f", containerfilePath, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "test-url-mixed", "cat", "/local.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("Local content"))

		mainFile := filepath.Join(podmanTest.TempDir, "main-url-test.txt")
		Expect(os.WriteFile(mainFile, []byte("Main context for URL test"), 0o644)).To(Succeed())

		containerfile = `FROM quay.io/libpod/alpine:latest
COPY main-url-test.txt /main.txt
COPY --from=gitrepo . /git-repo/`

		containerfilePath = filepath.Join(podmanTest.TempDir, "ContainerfileURL5")
		Expect(os.WriteFile(containerfilePath, []byte(containerfile), 0o644)).To(Succeed())

		session = podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-url-main",
			"--build-context", fmt.Sprintf("gitrepo=%s", testRepoURL),
			"-f", containerfilePath, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "test-url-main", "cat", "/main.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("Main context for URL test"))

		session = podmanTest.Podman([]string{"rmi", "-f", "test-url-single", "test-archive", "test-url-mixed", "test-url-main"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman build --build-context: Image source", func() {
		podmanTest.RestartRemoteService()

		alpineImage := "quay.io/libpod/alpine:latest"
		busyboxImage := "quay.io/libpod/busybox:latest"

		containerfile := `FROM quay.io/libpod/busybox:latest AS source
FROM quay.io/libpod/alpine:latest
COPY --from=source /bin/busybox /busybox-from-stage`

		containerfilePath := filepath.Join(podmanTest.TempDir, "ContainerfileMultiStage")
		Expect(os.WriteFile(containerfilePath, []byte(containerfile), 0o644)).To(Succeed())

		session := podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-multi-stage",
			"-f", containerfilePath, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		testCases := []struct {
			name          string
			prefix        string
			image         string
			contextName   string
			containerfile string
			verifyCmd     []string
		}{
			{
				name:        "docker-image-prefix",
				prefix:      "docker-image://",
				image:       alpineImage,
				contextName: "dockerimg",
				containerfile: `FROM quay.io/libpod/alpine:latest
COPY --from=dockerimg /etc/alpine-release /alpine-version.txt`,
				verifyCmd: []string{"cat", "/alpine-version.txt"},
			},
			{
				name:        "container-image-prefix",
				prefix:      "container-image://",
				image:       busyboxImage,
				contextName: "containerimg",
				containerfile: `FROM quay.io/libpod/alpine:latest
COPY --from=containerimg /bin/busybox /busybox-binary`,
				verifyCmd: []string{"/busybox-binary", "--help"},
			},
			{
				name:        "docker-prefix",
				prefix:      "docker://",
				image:       alpineImage,
				contextName: "dockershort",
				containerfile: `FROM quay.io/libpod/alpine:latest
COPY --from=dockershort /etc/os-release /os-release.txt`,
				verifyCmd: []string{"cat", "/os-release.txt"},
			},
		}

		for _, tc := range testCases {
			containerfilePath = filepath.Join(podmanTest.TempDir, fmt.Sprintf("Containerfile_%s", tc.name))
			Expect(os.WriteFile(containerfilePath, []byte(tc.containerfile), 0o644)).To(Succeed())

			session = podmanTest.Podman([]string{
				"build", "--pull-never", "-t", fmt.Sprintf("test-%s", tc.name),
				"--build-context", fmt.Sprintf("%s=%s%s", tc.contextName, tc.prefix, tc.image),
				"-f", containerfilePath, podmanTest.TempDir,
			})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			session = podmanTest.Podman(append([]string{"run", "--rm", fmt.Sprintf("test-%s", tc.name)}, tc.verifyCmd...))
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			if tc.name == "container-image-prefix" {
				Expect(session.OutputToString()).To(ContainSubstring("BusyBox"))
			}
		}

		session = podmanTest.Podman([]string{"rmi", "-f", "test-multi-stage"})
		session.WaitWithDefaultTimeout()
		for _, tc := range testCases {
			session = podmanTest.Podman([]string{"rmi", "-f", fmt.Sprintf("test-%s", tc.name)})
			session.WaitWithDefaultTimeout()
		}
	})

	It("podman build --build-context: Mixed source", func() {
		podmanTest.RestartRemoteService()

		localCtx := filepath.Join(podmanTest.TempDir, "local-mixed")
		Expect(os.MkdirAll(localCtx, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(localCtx, "local-config.json"), []byte(`{"context": "local", "version": "1.0"}`), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(localCtx, "app.conf"), []byte("# Local app configuration\nmode=production\nport=8080"), 0o644)).To(Succeed())

		urlContext := "https://github.com/containers/PodmanHello.git"
		alpineImage := "quay.io/libpod/alpine:latest"
		busyboxImage := "quay.io/libpod/busybox:latest"

		mainFile := filepath.Join(podmanTest.TempDir, "VERSION")
		Expect(os.WriteFile(mainFile, []byte("v1.0.0-mixed"), 0o644)).To(Succeed())

		containerfile := `FROM quay.io/libpod/alpine:latest

# From main build context
COPY VERSION /app/VERSION

# From local directory context
COPY --from=localdir /local-config.json /app/config/local-config.json
COPY --from=localdir /app.conf /app/config/app.conf

# From URL/Git context
COPY --from=gitrepo /LICENSE /app/licenses/podman-hello-LICENSE
COPY --from=gitrepo /README.md /app/docs/podman-hello-README.md

# From image contexts with different prefixes
COPY --from=alpineimg /etc/alpine-release /app/base-images/alpine-version
COPY --from=busyboximg /bin/busybox /app/tools/busybox

# Create a summary file
RUN echo "Build with all context types completed" > /app/build-summary.txt && \
    chmod +x /app/tools/busybox

WORKDIR /app`

		containerfilePath := filepath.Join(podmanTest.TempDir, "ContainerfileMixed")
		Expect(os.WriteFile(containerfilePath, []byte(containerfile), 0o644)).To(Succeed())

		session := podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-all-contexts",
			"--build-context", fmt.Sprintf("localdir=%s", localCtx),
			"--build-context", fmt.Sprintf("gitrepo=%s", urlContext),
			"--build-context", fmt.Sprintf("alpineimg=docker-image://%s", alpineImage),
			"--build-context", fmt.Sprintf("busyboximg=container-image://%s", busyboxImage),
			"-f", containerfilePath, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		verifyTests := []struct {
			cmd      []string
			expected string
		}{
			{[]string{"cat", "/app/VERSION"}, "v1.0.0-mixed"},
			{[]string{"cat", "/app/config/local-config.json"}, `"context": "local"`},
			{[]string{"cat", "/app/config/app.conf"}, "port=8080"},
			{[]string{"test", "-f", "/app/licenses/podman-hello-LICENSE"}, ""},
			{[]string{"test", "-f", "/app/docs/podman-hello-README.md"}, ""},
			{[]string{"cat", "/app/base-images/alpine-version"}, "3."},
			{[]string{"/app/tools/busybox", "--help"}, "BusyBox"},
			{[]string{"cat", "/app/build-summary.txt"}, "Build with all context types completed"},
		}

		for _, test := range verifyTests {
			session = podmanTest.Podman(append([]string{"run", "--rm", "test-all-contexts"}, test.cmd...))
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			if test.expected != "" {
				Expect(session.OutputToString()).To(ContainSubstring(test.expected))
			}
		}

		session = podmanTest.Podman([]string{
			"run", "--rm", "test-all-contexts",
			"/app/tools/busybox", "grep", "port", "/app/config/app.conf",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("port=8080"))

		containerfile2 := `FROM quay.io/libpod/alpine:latest
COPY --from=img1 /etc/os-release /prefix-test/docker-prefix.txt
COPY --from=img2 /etc/alpine-release /prefix-test/container-prefix.txt`

		containerfilePath2 := filepath.Join(podmanTest.TempDir, "ContainerfileMixed2")
		Expect(os.WriteFile(containerfilePath2, []byte(containerfile2), 0o644)).To(Succeed())

		session = podmanTest.Podman([]string{
			"build", "--pull-never", "-t", "test-prefix-mix",
			"--build-context", fmt.Sprintf("img1=docker://%s", alpineImage),
			"--build-context", fmt.Sprintf("img2=container-image://%s", alpineImage),
			"-f", containerfilePath2, podmanTest.TempDir,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "test-prefix-mix", "ls", "/prefix-test/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("docker-prefix.txt"))
		Expect(output).To(ContainSubstring("container-prefix.txt"))

		session = podmanTest.Podman([]string{"rmi", "-f", "test-all-contexts", "test-prefix-mix"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman build basic alpine and write metadata to external file", func() {
		SkipIfRemote("--metadata-file is not supported in remote mode")
		targetFile := filepath.Join(podmanTest.TempDir, "metadata.json")

		session := podmanTest.Podman([]string{"build", "--pull-never", "build/basicalpine", "--metadata-file", targetFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Verify that metadata file exists and contains expected keys
		metadata, err := os.ReadFile(targetFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(metadata).To(ContainSubstring("containerimage.config.digest"))
		Expect(metadata).To(ContainSubstring("containerimage.digest"))
		Expect(metadata).To(ContainSubstring("containerimage.descriptor"))
	})

	It("podman build --output type=local,dest=./folder outputs to ./folder", func() {
		SkipIfRemote("--output is not supported in remote mode")
		podmanTest.PodmanExitCleanly("build", "-f", "build/basicalpine/Containerfile", "--output", fmt.Sprintf("type=local,dest=%v", podmanTest.TempDir))
		files, err := os.ReadDir(podmanTest.TempDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(files)).To(BeNumerically(">", 1))
	})

	// Should error because no type
	It("podman build --output dest=./folder must fail", func() {
		SkipIfRemote("--output is not supported in remote mode")
		session := podmanTest.Podman([]string{"build", "-f", "build/basicalpine/Containerfile", "--output", fmt.Sprintf("dest=%v", podmanTest.TempDir)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `missing required key "type"`))
	})
})
