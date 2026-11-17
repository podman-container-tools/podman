//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman volume exists", func() {
	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman volume exists", func() {
		vol := "vol" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"volume", "create", vol})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "exists", vol})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "exists", stringid.GenerateRandomID()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
	})
})
