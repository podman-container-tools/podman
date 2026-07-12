//go:build linux || freebsd

package integration

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman generate spec", func() {
	BeforeEach(func() {
		SkipIfRemote("podman generate spec is not supported on the remote client")
	})

	It("podman generate spec bogus should fail", func() {
		session := podmanTest.Podman([]string{"generate", "spec", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "could not find a pod or container with the id foobar"))
	})

	It("podman generate spec basic usage", func() {
		session := podmanTest.Podman([]string{"create", "--cpus", "5", "--name", "specgen", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"generate", "spec", "--compact", "specgen"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman generate spec file", func() {
		session := podmanTest.Podman([]string{"create", "--cpus", "5", "--name", "specgen", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"generate", "spec", "--filename", filepath.Join(tempdir, "out.json"), "specgen"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		path := filepath.Join(tempdir, "out.json")

		exec := SystemExec("cat", []string{path})
		exec.WaitWithDefaultTimeout()
		Expect(exec.OutputToString()).Should(ContainSubstring("specgen-clone"))
		Expect(exec.OutputToString()).Should(ContainSubstring("500000"))
	})

	It("generate spec pod", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--cpus", "5", "--name", "podspecgen"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"generate", "spec", "--compact", "podspecgen"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("generate spec for pod with no infra container", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "none", "--name", "podspecgen2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"generate", "spec", "--compact", "podspecgen2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})
})
