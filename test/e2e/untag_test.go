//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman untag", func() {
	It("podman untag all", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		tags := []string{CIRROS_IMAGE, "registry.com/foo:bar", "localhost/foo:bar"}

		cmd := []string{"tag"}
		cmd = append(cmd, tags...)
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Make sure that all tags exists.
		for _, t := range tags {
			session = podmanTest.Podman([]string{"image", "exists", t})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		}

		// No arguments -> remove all tags.
		session = podmanTest.Podman([]string{"untag", CIRROS_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Make sure that none of tags exists anymore.
		for _, t := range tags {
			session = podmanTest.Podman([]string{"image", "exists", t})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(1, ""))
		}
	})

	It("podman tag/untag - tag normalization", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)

		tests := []struct {
			tag, normalized string
		}{
			{"registry.com/image:latest", "registry.com/image:latest"},
			{"registry.com/image", "registry.com/image:latest"},
			{"image:latest", "localhost/image:latest"},
			{"image", "localhost/image:latest"},
		}

		// Make sure that the user input is normalized correctly for
		// `podman tag` and `podman untag`.
		for _, tt := range tests {
			session := podmanTest.Podman([]string{"tag", CIRROS_IMAGE, tt.tag})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			session = podmanTest.Podman([]string{"image", "exists", tt.normalized})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			session = podmanTest.Podman([]string{"untag", CIRROS_IMAGE, tt.tag})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			session = podmanTest.Podman([]string{"image", "exists", tt.normalized})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(1, ""))
		}
	})
})
