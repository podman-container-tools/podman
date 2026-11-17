//go:build linux || freebsd

package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

func setupConnectionsConf() {
	// make sure connections are not written to real user config on host
	file := filepath.Join(podmanTest.TempDir, "containers.conf")
	f, err := os.Create(file)
	Expect(err).ToNot(HaveOccurred())
	f.Close()
	os.Setenv("CONTAINERS_CONF", file)

	file = filepath.Join(podmanTest.TempDir, "connections.conf")
	os.Setenv("PODMAN_CONNECTIONS_CONF", file)
}

var (
	systemConnectionListCmd                = []string{"system", "connection", "ls", "--format", "{{.Name}} {{.URI}} {{.Identity}} {{.Default}} {{.ReadWrite}}"}
	systemConnectionListTLSSpecialValueCmd = []string{"system", "connection", "ls", "--format", "tls"}
	systemConnectionListTLSCmd             = []string{"system", "connection", "ls", "--format", "{{.Name}} {{.URI}} {{.TLSCA}} {{.Default}} {{.ReadWrite}}"}
	systemConnectionListmTLSCmd            = []string{"system", "connection", "ls", "--format", "{{.Name}} {{.URI}} {{.TLSCA}} {{.TLSCert}} {{.TLSKey}} {{.Default}} {{.ReadWrite}}"}
	farmListCmd                            = []string{"farm", "ls", "--format", "{{.Name}} {{.Connections}} {{.Default}} {{.ReadWrite}}"}
)

var _ = Describe("podman system connection", func() {
	BeforeEach(setupConnectionsConf)

	Context("without running API service", func() {
		It("add ssh://", func() {
			cmd := []string{
				"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))

			cmd = []string{
				"system", "connection", "rename",
				"QA",
				"QE",
			}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QE ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))
		})

		It("add UDS", func() {
			cmd := []string{
				"system", "connection", "add",
				"QA-UDS",
				"unix:///run/podman/podman.sock",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())
			// stderr will probably warn (ENOENT or EACCESS) about socket
			// but it's too unreliable to test for.

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA-UDS unix:///run/podman/podman.sock true true"))

			cmd = []string{
				"system", "connection", "add",
				"QA-UDS1",
				"--socket-path", "/run/user/podman/podman.sock",
				"unix:///run/podman/podman.sock",
			}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`QA-UDS unix:///run/podman/podman.sock  true true
QA-UDS1 unix:///run/user/podman/podman.sock  false true
`))
		})

		It("add tcp", func() {
			cmd := []string{
				"system", "connection", "add",
				"QA-TCP",
				"tcp://localhost:8888",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA-TCP tcp://localhost:8888 true true"))
		})

		It("add tcp w/ TLS", func() {
			cmd := []string{
				"system", "connection", "add",
				"QA-TCP-TLS",
				"tcp://localhost:8888",
				"--tls-ca", "ca.pem",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListTLSCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA-TCP-TLS tcp://localhost:8888 ca.pem true true"))
		})

		It("add tcp w/ mTLS", func() {
			cmd := []string{
				"system", "connection", "add",
				"QA-TCP-MTLS",
				"tcp://localhost:8888",
				"--tls-ca", "ca.pem",
				"--tls-cert", "tls.crt",
				"--tls-key", "tls.key",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListmTLSCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA-TCP-MTLS tcp://localhost:8888 ca.pem tls.crt tls.key true true"))

			session = podmanTest.Podman(systemConnectionListTLSSpecialValueCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out).Should(Say("Name *URI *Identity *TLSCA *TLSCert *TLSKey *Default *ReadWrite\nQA-TCP-MTLS *tcp://localhost:8888 *ca.pem *tls.crt *tls.key *true *true"))
		})

		It("add tcp to reverse proxy path", func() {
			cmd := []string{
				"system", "connection", "add",
				"QA-TCP-RP",
				"tcp://localhost:8888/reverse/proxy/path/prefix",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA-TCP-RP tcp://localhost:8888/reverse/proxy/path/prefix true true"))
		})

		It("add to new farm", func() {
			cmd := []string{
				"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"--farm", "farm1",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))
			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("farm1 [QA] true true"))
		})

		It("add to existing farm", func() {
			// create empty farm
			cmd := []string{"farm", "create", "empty-farm"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"empty-farm\" created"))

			cmd = []string{
				"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"--farm", "empty-farm",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))
			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("empty-farm [QA] true true"))
		})

		It("removing connection should remove from farm also", func() {
			cmd := []string{
				"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"--farm", "farm1",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))
			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("farm1 [QA] true true"))

			// Remove the QA connection
			session = podmanTest.Podman([]string{"system", "connection", "remove", "QA"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal(""))
			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("farm1 [] true true"))
		})

		It("remove", func() {
			session := podmanTest.Podman([]string{
				"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// two passes to test that removing non-existent connection is not an error
			for range 2 {
				session = podmanTest.Podman([]string{"system", "connection", "remove", "QA"})
				session.WaitWithDefaultTimeout()
				Expect(session).Should(ExitCleanly())
				Expect(session.Out.Contents()).Should(BeEmpty())

				session = podmanTest.Podman(systemConnectionListCmd)
				session.WaitWithDefaultTimeout()
				Expect(session).Should(ExitCleanly())
				Expect(session.OutputToString()).To(Equal(""))
			}
		})

		It("remove --all", func() {
			session := podmanTest.Podman([]string{
				"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			session = podmanTest.Podman([]string{"system", "connection", "remove", "--all"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())
			Expect(session.Err.Contents()).Should(BeEmpty())

			session = podmanTest.Podman([]string{"system", "connection", "list"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToStringArray()).To(HaveLen(1))
		})

		It("default", func() {
			for _, name := range []string{"devl", "qe"} {
				cmd := []string{
					"system", "connection", "add",
					"--default",
					"--identity", "~/.ssh/id_rsa",
					name,
					"ssh://root@podman.test:2222/run/podman/podman.sock",
				}
				session := podmanTest.Podman(cmd)
				session.WaitWithDefaultTimeout()
				Expect(session).Should(ExitCleanly())
			}

			cmd := []string{"system", "connection", "default", "devl"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`devl ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true
qe ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa false true
`))

			cmd = []string{"system", "connection", "list"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out).Should(Say("Name *URI *Identity *Default"))
		})

		It("failed default", func() {
			cmd := []string{"system", "connection", "default", "devl"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).ShouldNot(ExitCleanly())
			Expect(session.Err).Should(Say("destination is not defined"))
		})

		It("failed rename", func() {
			cmd := []string{"system", "connection", "rename", "devl", "QE"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).ShouldNot(ExitCleanly())
			Expect(session.Err).Should(Say("destination is not defined"))
		})

		It("empty list", func() {
			cmd := []string{"system", "connection", "list"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToStringArray()).Should(HaveLen(1))
			Expect(session.Err.Contents()).Should(BeEmpty())
		})
	})

	Context("with running API service", func() {
		BeforeEach(func() {
			SkipIfNotRemote("requires podman API service")
		})

		It("add tcp:// connection with reverse proxy path", func() {
			// Create a reverse proxy to the podman socket using path prefix
			const pathPrefix = "/reverse/proxy/path/prefix"
			proxyGotUsed := false
			proxy := http.NewServeMux()
			proxy.Handle(pathPrefix+"/", &httputil.ReverseProxy{
				Rewrite: func(pr *httputil.ProxyRequest) {
					defer GinkgoRecover()
					proxyGotUsed = true
					pr.Out.URL.Path = strings.TrimPrefix(pr.Out.URL.Path, pathPrefix)
					pr.Out.URL.RawPath = strings.TrimPrefix(pr.Out.URL.RawPath, pathPrefix)
					scheme := "http"
					if podmanTest.RemoteTLSServerCAFile != "" {
						scheme = "https"
					}
					baseURL, err := url.Parse(scheme + "://localhost")
					Expect(err).ToNot(HaveOccurred())
					pr.SetURL(baseURL)
				},
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
						defer GinkgoRecover()
						By("Proxying to " + podmanTest.RemoteSocket)
						url, err := url.Parse(podmanTest.RemoteSocket)
						if err != nil {
							return nil, err
						}
						switch podmanTest.RemoteSocketScheme {
						case "unix":
							return (&net.Dialer{}).DialContext(ctx, podmanTest.RemoteSocketScheme, url.Path)
						case "tcp":
							return (&net.Dialer{}).DialContext(ctx, podmanTest.RemoteSocketScheme, url.Host)
						default:
							Fail("Unexpected remote socket scheme")
							panic("")
						}
					},
					TLSClientConfig: &tls.Config{
						RootCAs:      podmanTest.RemoteTLSServerCAPool,
						Certificates: podmanTest.RemoteTLSClientCerts,
					},
				},
			})
			srv := &http.Server{
				Handler:           proxy,
				ReadHeaderTimeout: time.Second,
			}

			// Serve the reverse proxy on a random port
			lis, err := net.Listen("tcp", "127.0.0.1:0")
			Expect(err).ToNot(HaveOccurred())
			defer lis.Close()

			defer srv.Close()
			go func() {
				defer GinkgoRecover()
				Expect(srv.Serve(lis)).To(MatchError(http.ErrServerClosed))
			}()

			connectionURL := "tcp://" + lis.Addr().String() + pathPrefix

			cmd := exec.Command(podmanTest.RemotePodmanBinary,
				"system", "connection", "add",
				"--default", "QA", connectionURL,
			)
			session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanTest.RemotePodmanBinary))
			Eventually(session, DefaultWaitTimeout).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())
			Expect(session.Err.Contents()).Should(BeEmpty())

			Expect(proxyGotUsed).To(BeFalse())
			cmd = exec.Command(podmanTest.RemotePodmanBinary,
				"--connection", "QA", "ps")
			session, err = Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanTest.RemotePodmanBinary))
			Eventually(session, DefaultWaitTimeout).Should(Exit(0))
			Expect(session.Out.Contents()).Should(Equal([]byte(`CONTAINER ID  IMAGE       COMMAND     CREATED     STATUS      PORTS       NAMES
`)))
			Expect(session.Err.Contents()).Should(BeEmpty())
			Expect(proxyGotUsed).To(BeTrue())

			// export the container_host env var and try again
			proxyGotUsed = false
			err = os.Setenv("CONTAINER_HOST", connectionURL)
			Expect(err).ToNot(HaveOccurred())
			defer os.Unsetenv("CONTAINER_HOST")

			cmd = exec.Command(podmanTest.RemotePodmanBinary, "ps")
			session, err = Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanTest.RemotePodmanBinary))
			Eventually(session, DefaultWaitTimeout).Should(Exit(0))
			Expect(session.Out.Contents()).Should(Equal([]byte(`CONTAINER ID  IMAGE       COMMAND     CREATED     STATUS      PORTS       NAMES
`)))
			Expect(session.Err.Contents()).Should(BeEmpty())
			Expect(proxyGotUsed).To(BeTrue())
		})
	})

	// Using "Ordered" or tests would concurrently access
	// the ~/.ssh/know_host file with unexpected results
	Context("sshd and API services required", Ordered, func() {
		var khCopyPath, khPath string
		var u *user.User

		BeforeAll(func() {
			Skip("FIXME: test assumes local ssh key is setup for the own user")
			// These tests are unique in as much as they require podman, podman-remote, systemd and sshd.
			// podman-remote commands will be executed by ginkgo directly.
			SkipIfContainerized("sshd is not available when running in a container")
			SkipIfRemote("connection heuristic requires both podman and podman-remote binaries")
			SkipIfNotRootless(fmt.Sprintf("FIXME: set up ssh keys when root. uid(%d) euid(%d)", os.Getuid(), os.Geteuid()))
			SkipIfSystemdNotRunning("cannot test connection heuristic if systemd is not running")
			SkipIfNotActive("sshd", "cannot test connection heuristic if sshd is not running")

			// If the file ~/.ssh/known_host exists, temporarily remove it so that the tests are deterministics
			var err error
			u, err = user.Current()
			Expect(err).ShouldNot(HaveOccurred())
			khPath = filepath.Join(u.HomeDir, ".ssh", "known_hosts")
			khCopyPath = khPath + ".copy"
			err = os.Rename(khPath, khCopyPath)
			Expect(err == nil || errors.Is(err, os.ErrNotExist)).To(BeTrue(), fmt.Sprintf("failed to rename %s to %s", khPath, khCopyPath))
		})

		AfterAll(func() { // codespell:ignore afterall
			err = os.Rename(khCopyPath, khPath)
			Expect(err == nil || errors.Is(err, os.ErrNotExist)).To(BeTrue(), fmt.Sprintf("failed to rename %s to %s", khCopyPath, khPath))
		})

		It("add ssh:// socket path using connection heuristic", func() {
			// Ensure that the remote end uses our built podman
			if os.Getenv("PODMAN_BINARY") == "" {
				err = os.Setenv("PODMAN_BINARY", podmanTest.PodmanBinary)
				Expect(err).ShouldNot(HaveOccurred())

				defer func() {
					os.Unsetenv("PODMAN_BINARY")
				}()
			}
			cmd := exec.Command(podmanTest.RemotePodmanBinary,
				"system", "connection", "add",
				"--default",
				"--identity", filepath.Join(u.HomeDir, ".ssh", "id_ed25519"),
				"QA",
				fmt.Sprintf("ssh://%s@localhost", u.Username))

			session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanTest.RemotePodmanBinary))
			Eventually(session, DefaultWaitTimeout).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())
			Expect(session.Err.Contents()).Should(BeEmpty())

			cmd = exec.Command(podmanTest.RemotePodmanBinary,
				"--connection", "QA", "ps")
			_, err = Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			// export the container_host env var and try again
			err = os.Setenv("CONTAINER_HOST", fmt.Sprintf("ssh://%s@localhost", u.Username))
			Expect(err).ToNot(HaveOccurred())
			defer os.Unsetenv("CONTAINER_HOST")

			cmd = exec.Command(podmanTest.RemotePodmanBinary, "ps")
			_, err = Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			uri := url.URL{
				Scheme: "ssh",
				User:   url.User(u.Username),
				Host:   "localhost:22",
				Path:   fmt.Sprintf("/run/user/%s/podman/podman.sock", u.Uid),
			}

			cmd = exec.Command(podmanTest.RemotePodmanBinary, systemConnectionListCmd...)
			lsSession, err := Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			lsSession.Wait(DefaultWaitTimeout)
			Expect(lsSession).Should(Exit(0))
			Expect(string(lsSession.Out.Contents())).To(Equal("QA " + uri.String() + " " + filepath.Join(u.HomeDir, ".ssh", "id_ed25519") + " true true\n"))
		})

		Describe("add ssh:// with known_hosts", func() {
			var (
				initialKnownHostFilesLines map[string][]string
				currentSSHServerHostname   string
			)

			BeforeAll(func() {
				currentSSHServerHostname = "localhost"

				// Retrieve current SSH server first two public keys
				// with the command `ssh-keyscan localhost`
				cmd := exec.Command("ssh-keyscan", currentSSHServerHostname)
				session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("`ssh-keyscan %s` failed to execute", currentSSHServerHostname))
				Eventually(session, DefaultWaitTimeout).Should(Exit(0))
				Expect(session.Out.Contents()).ShouldNot(BeEmpty(), fmt.Sprintf("`ssh-keyscan %s` output is empty", currentSSHServerHostname))
				serverKeys := bytes.Split(session.Out.Contents(), []byte("\n"))
				Expect(len(serverKeys)).Should(BeNumerically(">=", 2), fmt.Sprintf("`ssh-keyscan %s` returned less then 2 keys", currentSSHServerHostname))

				initialKnownHostFilesLines = map[string][]string{
					"serverFirstKey":   {string(serverKeys[0])},
					"serverSecondKey":  {string(serverKeys[1])},
					"fakeKey":          {currentSSHServerHostname + " ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGnBlHlwqleAtyzT01mLa+DXQFyxX8T0oa8odcEZ2/07"},
					"differentHostKey": {"differentserver ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGnBlHlwqleAtyzT01mLa+DXQFyxX8T0oa8odcEZ2/07"},
					"empty":            {},
				}
			})

			DescribeTable("->",
				func(label string, shouldFail bool, shouldAddKey bool) {
					initialKhLines, ok := initialKnownHostFilesLines[label]
					Expect(ok).To(BeTrue(), fmt.Sprintf("label %q not found in kh", label))
					// Create known_hosts file if needed
					if len(initialKhLines) > 0 {
						khFile, err := os.Create(khPath)
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to create %s", khPath))
						defer khFile.Close()
						err = os.WriteFile(khPath, []byte(strings.Join(initialKhLines, "\n")), 0o600)
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to write to %s", khPath))
					}
					// Ensure that the remote end uses our built podman
					if os.Getenv("PODMAN_BINARY") == "" {
						err = os.Setenv("PODMAN_BINARY", podmanTest.PodmanBinary)
						Expect(err).ShouldNot(HaveOccurred())

						defer func() {
							os.Unsetenv("PODMAN_BINARY")
						}()
					}
					// Run podman system connection add
					cmd := exec.Command(podmanTest.RemotePodmanBinary,
						"system", "connection", "add",
						"--default",
						"--identity", filepath.Join(u.HomeDir, ".ssh", "id_ed25519"),
						"QA",
						fmt.Sprintf("ssh://%s@%s", u.Username, currentSSHServerHostname))
					session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanTest.RemotePodmanBinary))
					Expect(session.Out.Contents()).Should(BeEmpty())
					if shouldFail {
						Eventually(session, DefaultWaitTimeout).Should(Exit(125))
						Expect(session.Err.Contents()).ShouldNot(BeEmpty())
					} else {
						Eventually(session, DefaultWaitTimeout).Should(Exit(0))
						Expect(session.Err.Contents()).Should(BeEmpty())
					}
					// If the known_hosts file didn't exist, it should
					// have been created
					if len(initialKhLines) == 0 {
						Expect(khPath).To(BeAnExistingFile())
						defer os.Remove(khPath)
					}
					// If the known_hosts file didn't contain the SSH server
					// public key it should have been added
					if shouldAddKey {
						khFileContent, err := os.ReadFile(khPath)
						Expect(err).ToNot(HaveOccurred())
						khLines := bytes.Split(khFileContent, []byte("\n"))
						Expect(len(khLines)).To(BeNumerically(">", len(initialKhLines)))
					}
				},
				Entry(
					"with a public key of the SSH server that matches the SSH server first key",
					"serverFirstKey",
					false,
					false,
				),
				Entry(
					"with a public key of the SSH server that matches SSH server second key",
					"serverSecondKey",
					false,
					false,
				),
				Entry(
					"with a fake public key of the SSH server that doesn't match any of the SSH server keys (should fail)",
					"fakeKey",
					true,
					false,
				),
				Entry(
					"with no public key for the SSH server (new key should be added)",
					"differentHostKey",
					false,
					true,
				),
				Entry(
					"not existing (should be created and a new key should be added)",
					"empty",
					false,
					true,
				),
			)
		})
	})
})
