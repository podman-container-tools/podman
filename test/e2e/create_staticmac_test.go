//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman run with --mac-address flag", func() {
	It("Podman run --mac-address", func() {
		result := podmanTest.Podman([]string{"run", "--mac-address", "92:d0:c6:0a:29:34", ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		if isRootless() {
			Expect(result).Should(ExitWithError(125, "invalid config provided: networks and static ip/mac address can only be used with Bridge mode networking"))
		} else {
			Expect(result).Should(ExitCleanly())
			Expect(result.OutputToString()).To(ContainSubstring("92:d0:c6:0a:29:34"))
		}
	})

	It("Podman run --mac-address with custom network", func() {
		net := "n1" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"run", "--network", net, "--mac-address", "92:d0:c6:00:29:34", ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("92:d0:c6:00:29:34"))
	})
})
