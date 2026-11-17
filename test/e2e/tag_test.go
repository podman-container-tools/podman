//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman tag", func() {
	BeforeEach(func() {
		podmanTest.AddImageToRWStore(ALPINE)
	})

	It("podman tag shortname:latest", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"inspect", "foobar:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		inspectData := results.InspectImageJSON()
		Expect(inspectData[0].RepoTags).To(ContainElement("quay.io/libpod/alpine:latest"))
		Expect(inspectData[0].RepoTags).To(ContainElement("localhost/foobar:latest"))
	})

	It("podman tag shortname", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"inspect", "foobar:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		inspectData := results.InspectImageJSON()
		Expect(inspectData[0].RepoTags).To(ContainElement("quay.io/libpod/alpine:latest"))
		Expect(inspectData[0].RepoTags).To(ContainElement("localhost/foobar:latest"))
	})

	It("podman tag shortname:tag", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar:new"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"inspect", "foobar:new"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		inspectData := results.InspectImageJSON()
		Expect(inspectData[0].RepoTags).To(ContainElement("quay.io/libpod/alpine:latest"))
		Expect(inspectData[0].RepoTags).To(ContainElement("localhost/foobar:new"))
	})

	It("podman tag shortname image no tag", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"tag", "foobar", "barfoo"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())

		verify := podmanTest.Podman([]string{"inspect", "barfoo"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(ExitCleanly())
	})
})
