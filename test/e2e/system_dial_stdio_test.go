//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("podman system dial-stdio", func() {
	It("podman system dial-stdio help", func() {
		session := podmanTest.Podman([]string{"system", "dial-stdio", "--help"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Examples: podman system dial-stdio"))
	})
})
