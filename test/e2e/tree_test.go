//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman image tree", func() {
	BeforeEach(func() {
		podmanTest.AddImageToRWStore(BB)
	})

	It("podman image tree", func() {
		SkipIfRemote("podman-image-tree is not supported for remote clients")
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		dockerfile := `FROM quay.io/libpod/cirros:latest
RUN mkdir hello
RUN touch test.txt
ENV foo=bar
`
		podmanTest.BuildImage(dockerfile, "test:latest", "true")

		session := podmanTest.Podman([]string{"image", "tree", "test:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"image", "tree", "--whatrequires", "quay.io/libpod/cirros:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rmi", "test:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"rmi", "quay.io/libpod/cirros:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})
})
