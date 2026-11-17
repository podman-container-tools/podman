//go:build linux || freebsd

package integration

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman unshare", func() {
	BeforeEach(func() {
		if _, err := os.Stat("/proc/self/uid_map"); err != nil {
			Skip("User namespaces not supported.")
		}

		if !isRootless() {
			Skip("Use unshare in rootless only")
		}
	})

	It("podman unshare", func() {
		SkipIfRemote("podman-remote unshare is not supported")
		userNS, _ := os.Readlink("/proc/self/ns/user")
		session := podmanTest.Podman([]string{"unshare", "readlink", "/proc/self/ns/user"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).ToNot(ContainSubstring(userNS))
	})

	It("podman unshare exit codes", func() {
		SkipIfRemote("podman-remote unshare is not supported")
		session := podmanTest.Podman([]string{"unshare", "false"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
		Expect(session.OutputToString()).Should(Equal(""))

		session = podmanTest.Podman([]string{"unshare", "/usr/bin/bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(127, "no such file or directory"))
		Expect(session.OutputToString()).Should(Equal(""))

		session = podmanTest.Podman([]string{"unshare", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(127, "executable file not found in $PATH"))
		Expect(session.OutputToString()).Should(Equal(""))

		session = podmanTest.Podman([]string{"unshare", "/usr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(126, "permission denied"))
		Expect(session.OutputToString()).Should(Equal(""))

		session = podmanTest.Podman([]string{"unshare", "--bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "unknown flag: --bogus"))
		Expect(session.OutputToString()).Should(Equal(""))
	})

	It("podman unshare check remote error", func() {
		SkipIfNotRemote("check for podman-remote unshare error")
		session := podmanTest.Podman([]string{"unshare"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `Error: cannot use command "podman-remote unshare" with the remote podman client`))
	})
})
