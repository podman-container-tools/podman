//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/fileutils"
)

const (
	registryUser     = "podmantest"
	registryPassword = "test"
	htpasswdLine     = "podmantest:$2y$05$OIqogLGEkRXVUYqeRemzk.ZAeX4QmchNPQQRZY35AGyAt/dkaSM3W\n"
)

var _ = Describe("Podman login and logout", func() {
	var (
		err                      error
		authPath                 string
		certPath                 string
		certDirPath              string
		server                   string
		testImg                  string
		registriesConfWithSearch []byte
	)

	BeforeEach(func() {
		authPath = filepath.Join(podmanTest.TempDir, "auth")
		err := os.Mkdir(authPath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		f, err := os.Create(filepath.Join(authPath, "htpasswd"))
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		_, err = f.WriteString(htpasswdLine)
		Expect(err).ToNot(HaveOccurred())
		port := GetPort()
		server = strings.Join([]string{"localhost", strconv.Itoa(port)}, ":")

		registriesConfWithSearch = fmt.Appendf(nil, "unqualified-search-registries = ['%s']", server)

		testImg = strings.Join([]string{server, "test-alpine"}, "/")

		certDirPath = filepath.Join(os.Getenv("HOME"), ".config/containers/certs.d", server)
		err = os.MkdirAll(certDirPath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		cwd, _ := os.Getwd()
		certPath = filepath.Join(cwd, "../", "certs")

		_, err = fileutils.CopyFile(filepath.Join(certPath, "domain.crt"), filepath.Join(certDirPath, "ca.crt"))
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{
			"run", "-d", "-p", strings.Join([]string{strconv.Itoa(port), strconv.Itoa(port)}, ":"),
			"-e", strings.Join([]string{"REGISTRY_HTTP_ADDR=0.0.0.0", strconv.Itoa(port)}, ":"), "--name", "registry", "-v",
			strings.Join([]string{authPath, "/auth:Z"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs:z"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", REGISTRY_IMAGE,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		// collision protection: each test uses a unique authfile
		os.Setenv("REGISTRY_AUTH_FILE", filepath.Join(podmanTest.TempDir, "default-auth.json"))
	})

	AfterEach(func() {
		os.Unsetenv("REGISTRY_AUTH_FILE")
		os.RemoveAll(authPath)
		os.RemoveAll(certDirPath)
	})

	readAuthInfo := func(filePath string) map[string]any {
		authBytes, err := os.ReadFile(filePath)
		Expect(err).ToNot(HaveOccurred())

		var authInfo map[string]any
		err = json.Unmarshal(authBytes, &authInfo)
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Println(authInfo)

		const authsKey = "auths"
		Expect(authInfo).To(HaveKey(authsKey))

		auths, ok := authInfo[authsKey].(map[string]any)
		Expect(ok).To(BeTrue(), "authInfo[%s]", authsKey)

		return auths
	}

	It("podman login and logout", func() {
		authFile := os.Getenv("REGISTRY_AUTH_FILE")
		Expect(authFile).NotTo(BeEmpty(), "$REGISTRY_AUTH_FILE")

		session := podmanTest.Podman([]string{"login", "-u", registryUser, "-p", registryPassword, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Confirm that file was created, with the desired credentials
		auths := readAuthInfo(authFile)
		Expect(auths).To(HaveKey(server))
		// base64-encoded "podmantest:test"
		Expect(auths[server]).To(HaveKeyWithValue("auth", "cG9kbWFudGVzdDp0ZXN0"))

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, ": authentication required"))
	})

	It("podman login and logout without registry parameter", func() {
		registriesConf, err := os.CreateTemp("", "TestLoginWithoutParameter")
		Expect(err).ToNot(HaveOccurred())
		defer registriesConf.Close()
		defer os.Remove(registriesConf.Name())

		err = os.WriteFile(registriesConf.Name(), registriesConfWithSearch, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		// Environment is per-process, so this looks very unsafe; actually it seems fine because tests are not
		// run in parallel unless they opt in by calling t.Parallel().  So don’t do that.
		oldRCP, hasRCP := os.LookupEnv("CONTAINERS_REGISTRIES_CONF")
		defer func() {
			if hasRCP {
				os.Setenv("CONTAINERS_REGISTRIES_CONF", oldRCP)
			} else {
				os.Unsetenv("CONTAINERS_REGISTRIES_CONF")
			}
		}()
		os.Setenv("CONTAINERS_REGISTRIES_CONF", registriesConf.Name())

		session := podmanTest.Podman([]string{"login", "-u", registryUser, "-p", registryPassword})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"logout"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman login and logout with flag --authfile", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")
		session := podmanTest.Podman([]string{"login", "--username", registryUser, "--password", registryPassword, "--authfile", authFile, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		readAuthInfo(authFile)

		// push should fail with nonexistent authfile
		session = podmanTest.Podman([]string{"push", "-q", "--authfile", "/tmp/nonexistent", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "credential file is not accessible: faccessat /tmp/nonexistent: no such file or directory"))

		session = podmanTest.Podman([]string{"push", "-q", "--authfile", authFile, ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-q", "--authfile", authFile, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// logout should fail with nonexistent authfile
		session = podmanTest.Podman([]string{"logout", "--authfile", "/tmp/nonexistent", server})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "credential file is not accessible: faccessat /tmp/nonexistent: no such file or directory"))

		session = podmanTest.Podman([]string{"logout", "--authfile", authFile, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman login and logout --compat-auth-file flag handling", func() {
		// A minimal smoke test
		compatAuthFile := filepath.Join(podmanTest.TempDir, "config.json")
		session := podmanTest.Podman([]string{"login", "--username", registryUser, "--password", registryPassword, "--compat-auth-file", compatAuthFile, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		readAuthInfo(compatAuthFile)

		session = podmanTest.Podman([]string{"logout", "--compat-auth-file", compatAuthFile, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// logout should fail with nonexistent authfile
		session = podmanTest.Podman([]string{"logout", "--compat-auth-file", "/tmp/nonexistent", server})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "credential file is not accessible: faccessat /tmp/nonexistent: no such file or directory"))

		// inconsistent command line flags are rejected
		// Pre-create the files to make sure we are not hitting the “file not found” path
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")
		err := os.WriteFile(authFile, []byte("{}"), 0o700)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(compatAuthFile, []byte("{}"), 0o700)
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{
			"login", "--username", registryUser, "--password", registryPassword,
			"--authfile", authFile, "--compat-auth-file", compatAuthFile, server,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "options for paths to the credential file and to the Docker-compatible credential file can not be set simultaneously"))

		session = podmanTest.Podman([]string{"logout", "--authfile", authFile, "--compat-auth-file", compatAuthFile, server})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "options for paths to the credential file and to the Docker-compatible credential file can not be set simultaneously"))
	})

	It("podman manifest with --authfile", func() {
		os.Unsetenv("REGISTRY_AUTH_FILE")

		authFile := filepath.Join(podmanTest.TempDir, "auth.json")
		session := podmanTest.Podman([]string{"login", "--username", registryUser, "--password", registryPassword, "--authfile", authFile, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		readAuthInfo(authFile)

		session = podmanTest.Podman([]string{"manifest", "create", testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"manifest", "push", "-q", testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, ": authentication required"))

		session = podmanTest.Podman([]string{"manifest", "push", "-q", "--authfile", authFile, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Now remove the local manifest to trigger remote inspection
		session = podmanTest.Podman([]string{"manifest", "rm", testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"manifest", "inspect", testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, ": authentication required"))

		session = podmanTest.Podman([]string{"manifest", "inspect", "--authfile", authFile, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman login and logout with --tls-verify", func() {
		session := podmanTest.Podman([]string{"login", "--username", registryUser, "--password", registryPassword, "--tls-verify=false", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})
	It("podman login and logout with --cert-dir", func() {
		certDir := filepath.Join(podmanTest.TempDir, "certs")
		err := os.MkdirAll(certDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		_, err = fileutils.CopyFile(filepath.Join(certPath, "domain.crt"), filepath.Join(certDir, "ca.crt"))
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"login", "--username", registryUser, "--password", registryPassword, "--cert-dir", certDir, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"push", "-q", "--cert-dir", certDir, ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})
	It("podman login and logout with multi registry", func() {
		certDir := filepath.Join(os.Getenv("HOME"), ".config/containers/certs.d", "localhost:9001")
		err = os.MkdirAll(certDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		_, err = fileutils.CopyFile(filepath.Join(certPath, "domain.crt"), filepath.Join(certDir, "ca.crt"))
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(certDir)

		// N/B: This second registry container shares the same auth and cert dirs
		//      as the registry started from BeforeEach().  Since this one starts
		//      second, re-labeling the volumes should keep SELinux happy.
		session := podmanTest.Podman([]string{
			"run", "-d", "-p", "9001:9001", "-e", "REGISTRY_HTTP_ADDR=0.0.0.0:9001", "--name", "registry1", "-v",
			strings.Join([]string{authPath, "/auth:z"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs:z"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", REGISTRY_IMAGE,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry1", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		session = podmanTest.Podman([]string{"login", "--username", registryUser, "--password", registryPassword, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "/test-alpine: authentication required"))

		session = podmanTest.Podman([]string{"login", "--username", registryUser, "--password", registryPassword, "localhost:9001"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "/test-alpine: authentication required"))

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"login", "--username", registryUser, "--password", registryPassword, "localhost:9001"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"logout", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "/test-alpine: authentication required"))

		session = podmanTest.Podman([]string{"push", "-q", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "/test-alpine: authentication required"))
	})

	It("podman login and logout with repository", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		testRepository := server + "/podmantest"
		session := podmanTest.Podman([]string{
			"login",
			"-u", registryUser,
			"-p", registryPassword,
			"--authfile", authFile,
			testRepository,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		authInfo := readAuthInfo(authFile)
		Expect(authInfo).To(HaveKey(testRepository))

		session = podmanTest.Podman([]string{
			"logout",
			"--authfile", authFile,
			testRepository,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		authInfo = readAuthInfo(authFile)
		Expect(authInfo).NotTo(HaveKey(testRepository))
	})

	It("podman login and logout with repository and specified image", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		testTarget := server + "/podmantest/test-alpine"
		session := podmanTest.Podman([]string{
			"login",
			"-u", registryUser,
			"-p", registryPassword,
			"--authfile", authFile,
			testTarget,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		authInfo := readAuthInfo(authFile)
		Expect(authInfo).To(HaveKey(testTarget))

		session = podmanTest.Podman([]string{
			"push", "-q",
			"--authfile", authFile,
			ALPINE, testTarget,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman login and logout with repository with fallback", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		testRepos := []string{
			server + "/podmantest",
			server,
		}
		for _, testRepo := range testRepos {
			session := podmanTest.Podman([]string{
				"login",
				"-u", registryUser,
				"-p", registryPassword,
				"--authfile", authFile,
				testRepo,
			})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		}

		authInfo := readAuthInfo(authFile)
		Expect(authInfo).To(HaveKey(testRepos[0]))
		Expect(authInfo).To(HaveKey(testRepos[1]))

		session := podmanTest.Podman([]string{
			"push", "-q",
			"--authfile", authFile,
			ALPINE, testRepos[0] + "/test-image-alpine",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{
			"logout",
			"--authfile", authFile,
			testRepos[0],
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{
			"push", "-q",
			"--authfile", authFile,
			ALPINE, testRepos[0] + "/test-image-alpine",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{
			"logout",
			"--authfile", authFile,
			testRepos[1],
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		authInfo = readAuthInfo(authFile)
		Expect(authInfo).NotTo(HaveKey(testRepos[0]))
		Expect(authInfo).NotTo(HaveKey(testRepos[1]))
	})

	It("podman login with http{s} prefix", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		for _, invalidArg := range []string{
			"https://" + server + "/podmantest",
			"http://" + server + "/podmantest/image:latest",
		} {
			session := podmanTest.Podman([]string{
				"login",
				"-u", registryUser,
				"-p", registryPassword,
				"--authfile", authFile,
				invalidArg,
			})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitCleanly())
		}
	})

	It("podman login and logout with repository push with invalid auth.json credentials", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")
		// only `server` contains the correct login data
		err := os.WriteFile(authFile, fmt.Appendf(nil, `{"auths": {
			"%s/podmantest": { "auth": "cG9kbWFudGVzdDp3cm9uZw==" },
			"%s": { "auth": "cG9kbWFudGVzdDp0ZXN0" }
		}}`, server, server), 0o644)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{
			"push", "-q",
			"--authfile", authFile,
			ALPINE, server + "/podmantest/test-image",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "/test-image: authentication required"))

		session = podmanTest.Podman([]string{
			"push", "-q",
			"--authfile", authFile,
			ALPINE, server + "/test-image",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())
	})

	It("podman login and logout with repository pull with wrong auth.json credentials", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		testTarget := server + "/podmantest/test-alpine"
		session := podmanTest.Podman([]string{
			"login",
			"-u", registryUser,
			"-p", registryPassword,
			"--authfile", authFile,
			testTarget,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{
			"push", "-q",
			"--authfile", authFile,
			ALPINE, testTarget,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// only `server + /podmantest` and `server` have the correct login data
		err := os.WriteFile(authFile, fmt.Appendf(nil, `{"auths": {
			"%s/podmantest/test-alpine": { "auth": "cG9kbWFudGVzdDp3cm9uZw==" },
			"%s/podmantest": { "auth": "cG9kbWFudGVzdDp0ZXN0" },
			"%s": { "auth": "cG9kbWFudGVzdDp0ZXN0" }
		}}`, server, server, server), 0o644)
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{
			"pull", "-q",
			"--authfile", authFile,
			server + "/podmantest/test-alpine",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "/test-alpine: authentication required"))
	})
})
