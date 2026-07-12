//go:build linux || freebsd

package integration

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman run", func() {
	It("podman run environment test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv", "HOME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/root"))

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "2", ALPINE, "printenv", "HOME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/sbin"))

		session = podmanTest.Podman([]string{"run", "--rm", "--env", "HOME=/foo", ALPINE, "printenv", "HOME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/foo"))

		session = podmanTest.Podman([]string{"run", "--rm", "--env", "FOO=BAR,BAZ", ALPINE, "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("BAR,BAZ"))

		session = podmanTest.Podman([]string{"run", "--rm", "--env", "PATH=/bin", ALPINE, "printenv", "PATH"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/bin"))

		// Verify environ keys with spaces do not blow up podman command
		os.Setenv("FOO BAR", "BAZ")
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		os.Unsetenv("FOO BAR")

		os.Setenv("FOO", "BAR")
		session = podmanTest.Podman([]string{"run", "--rm", "--env", "FOO", ALPINE, "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("BAR"))
		os.Unsetenv("FOO")

		session = podmanTest.Podman([]string{"run", "--rm", "--env", "FOO", ALPINE, "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(BeEmpty())
		Expect(session).Should(ExitWithError(1, ""))

		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// This currently does not work
		// Re-enable when hostname is an env variable
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "printenv"})
		session.Wait(10)
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("HOSTNAME"))
	})

	It("podman run with --env-merge", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENV hello=world
`
		podmanTest.BuildImage(dockerfile, "test", "false")
		session := podmanTest.Podman([]string{"run", "--rm", "--env-merge", "hello=${hello}-earth", "test", "env"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("world-earth"))

		session = podmanTest.Podman([]string{"run", "--rm", "--env-merge", "foo=${bar}-earth", "test", "printenv", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("-earth"))
	})

	It("podman run --env-host environment test", func() {
		env := append(os.Environ(), "FOO=BAR")
		session := podmanTest.PodmanExecBaseWithOptions([]string{"run", "--rm", "--env-host", ALPINE, "/bin/printenv", "FOO"}, PodmanExecOptions{Env: env})
		session.WaitWithDefaultTimeout()
		if IsRemote() {
			// podman-remote does not support --env-host
			Expect(session).Should(ExitWithError(125, "unknown flag: --env-host"))
			return
		}
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("BAR"))

		session = podmanTest.PodmanExecBaseWithOptions([]string{"run", "--rm", "--env", "FOO=BAR1", "--env-host", ALPINE, "/bin/printenv", "FOO"}, PodmanExecOptions{Env: env})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("BAR1"))
		os.Unsetenv("FOO")
	})

	It("podman run --http-proxy test", func() {
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
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("1.2.3.4"))

		session = podmanTest.Podman([]string{"run", "--http-proxy=false", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
		Expect(session.OutputToString()).To(Equal(""))

		session = podmanTest.Podman([]string{"run", "--env", "http_proxy=5.6.7.8", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("5.6.7.8"))

		session = podmanTest.Podman([]string{"run", "--http-proxy=false", "--env", "http_proxy=5.6.7.8", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("5.6.7.8"))
	})
})
