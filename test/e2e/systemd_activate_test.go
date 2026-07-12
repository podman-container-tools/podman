//go:build linux || freebsd

package integration

import (
	"errors"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	testUtils "go.podman.io/podman/v6/test/utils"
	podmanUtils "go.podman.io/podman/v6/utils"
)

var _ = Describe("Systemd activate", func() {
	var activate string

	BeforeEach(func() {
		SkipIfRemote("Testing stopped service requires both podman and podman-remote binaries")

		activate, err = exec.LookPath("systemd-socket-activate")
		if err != nil {
			activate = "/usr/bin/systemd-socket-activate"
		}
		stat, err := os.Stat(activate)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			Skip(activate + " required for systemd activation tests")
		case stat.Mode()&0o111 == 0:
			Skip("Unable to execute " + activate)
		case err != nil:
			Skip(err.Error())
		}
	})

	It("stop podman.service", func() {
		// systemd-socket-activate does not support DNS lookups
		host := "127.0.0.1"
		port, err := podmanUtils.GetRandomPort()
		Expect(err).ToNot(HaveOccurred())
		addr := net.JoinHostPort(host, strconv.Itoa(port))

		podmanOptions := podmanTest.makeOptions(nil, testUtils.PodmanExecOptions{})

		systemdArgs := []string{
			"-E", "http_proxy", "-E", "https_proxy", "-E", "no_proxy",
			"-E", "HTTP_PROXY", "-E", "HTTPS_PROXY", "-E", "NO_PROXY",
			"-E", "XDG_RUNTIME_DIR",
			"--listen", addr,
			podmanTest.PodmanBinary,
		}
		systemdArgs = append(systemdArgs, podmanOptions...)
		systemdArgs = append(systemdArgs, "system", "service", "--time=0")

		activateSession := testUtils.StartSystemExec(activate, systemdArgs)
		Expect(activateSession.Exited).ShouldNot(Receive(), "Failed to start podman service")
		WaitForService(url.URL{Scheme: "tcp", Host: addr})
		defer activateSession.Signal(syscall.SIGTERM)

		// Create custom functions for running podman and
		// podman-remote.  This test is a rare exception where both
		// binaries need to be run in parallel.  Usually, the remote
		// and non-remote details are hidden.  Yet we use the
		// `podmanOptions` above to make sure all settings (root,
		// runroot, events, tmpdir, etc.) are used as in other e2e
		// tests.
		podmanRemote := func(args ...string) *testUtils.PodmanSession {
			args = append([]string{"--url", "tcp://" + addr}, args...)
			return testUtils.SystemExec(podmanTest.RemotePodmanBinary, args)
		}

		podman := func(args ...string) *testUtils.PodmanSession {
			args = append(podmanOptions, args...)
			return testUtils.SystemExec(podmanTest.PodmanBinary, args)
		}

		// regression check for https://github.com/containers/podman/issues/24152
		session := podmanRemote("info", "--format", "{{.Host.RemoteSocket.Path}}--{{.Host.RemoteSocket.Exists}}")
		Expect(session).Should(testUtils.ExitCleanly())
		Expect(session.OutputToString()).To(Equal("tcp://" + addr + "--true"))

		containerName := "top_" + testUtils.RandomString(8)
		apiSession := podmanRemote(
			"create", "--tty", "--name", containerName, "--entrypoint", "top",
			ALPINE,
		)
		Expect(apiSession).Should(testUtils.ExitCleanly())
		defer podman("rm", "-f", containerName)

		apiSession = podmanRemote("start", containerName)
		Expect(apiSession).Should(testUtils.ExitCleanly())

		apiSession = podmanRemote("inspect", "--format={{.State.Running}}", containerName)
		Expect(apiSession).Should(testUtils.ExitCleanly())
		Expect(apiSession.OutputToString()).To(Equal("true"))

		// Emulate 'systemd stop podman.service'
		activateSession.Signal(syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		Eventually(activateSession).Should(Exit(0))

		abiSession := podman("inspect", "--format={{.State.Running}}", containerName)
		Expect(abiSession).To(testUtils.ExitCleanly())
		Expect(abiSession.OutputToString()).To(Equal("true"))
	})

	It("invalid systemd file descriptor", func() {
		host := "127.0.0.1"
		port, err := podmanUtils.GetRandomPort()
		Expect(err).ToNot(HaveOccurred())

		addr := net.JoinHostPort(host, strconv.Itoa(port))

		// start systemd activation with datagram socket
		activateSession := testUtils.StartSystemExec(activate, []string{
			"--datagram", "--listen", addr,
			podmanTest.PodmanBinary,
			"--root=" + filepath.Join(tempdir, "server_root"),
			"system", "service",
			"--time=0",
		})
		Expect(activateSession.Exited).ShouldNot(Receive(), "Failed to start podman service")

		// we have to wait for systemd-socket-activate to become ready
		time.Sleep(1 * time.Second)

		// now dial the socket to start podman
		conn, err := net.Dial("udp", addr)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		_, err = conn.Write([]byte("test"))
		Expect(err).ToNot(HaveOccurred())

		// wait for podman to exit
		activateSession.Wait(10)
		Expect(activateSession).To(Exit(125))
		Expect(activateSession.ErrorToString()).To(ContainSubstring("Error: unexpected fd received from systemd: cannot listen on it"))
	})
})
