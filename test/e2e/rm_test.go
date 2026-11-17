//go:build linux || freebsd

package integration

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman rm", func() {
	It("podman rm stopped container", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman rm refuse to remove a running container", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(2, "containers cannot be removed without force"))
	})

	It("podman rm created container", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman container rm created container", func() {
		session := podmanTest.Podman([]string{"container", "create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman rm running container with -f", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"rm", "-t", "0", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman rm all containers", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"rm", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman rm all containers with one running and short options", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"rm", "-af"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman rm the latest container", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, cid := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		latest := "-l"
		if IsRemote() {
			latest = cid
		}
		result := podmanTest.Podman([]string{"rm", latest})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman rm --cidfile", func() {
		cidFile := filepath.Join(tempdir, "cid")

		session := podmanTest.Podman([]string{"create", "--cidfile", cidFile, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"rm", "--cidfile", cidFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm multiple --cidfile", func() {
		cidFile1 := filepath.Join(tempdir, "cid-1")
		cidFile2 := filepath.Join(tempdir, "cid-2")

		session := podmanTest.Podman([]string{"create", "--cidfile", cidFile1, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid1 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"create", "--cidfile", cidFile2, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid2 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		result := podmanTest.Podman([]string{"rm", "--cidfile", cidFile1, "--cidfile", cidFile2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm invalid --latest and --cidfile and --all", func() {
		SkipIfRemote("Verifying --latest flag")

		result := podmanTest.Podman([]string{"rm", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"rm", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"rm", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"rm", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all and --latest cannot be used together"))
	})

	It("podman rm --all", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		session = podmanTest.Podman([]string{"rm", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm --ignore", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"rm", "bogus", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, `no container with ID or name "bogus" found: no such container`))
		if IsRemote() {
			Expect(session.OutputToString()).To(BeEquivalentTo(cid))
		}

		session = podmanTest.Podman([]string{"rm", "--ignore", "bogus", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if !IsRemote() {
			Expect(session.OutputToString()).To(BeEquivalentTo(cid))
		}
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm bogus container", func() {
		session := podmanTest.Podman([]string{"rm", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, `no container with ID or name "bogus" found: no such container`))
	})

	It("podman rm bogus container and a running container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rm", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, `no container with ID or name "bogus" found: no such container`))

		session = podmanTest.Podman([]string{"rm", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, `no container with ID or name "bogus" found: no such container`))
	})

	It("podman rm --ignore bogus container and a running container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rm", "--ignore", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(2, "containers cannot be removed without force"))

		session = podmanTest.Podman([]string{"rm", "-t", "0", "--force", "--ignore", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeEquivalentTo("test1"))
	})

	It("podman rm --filter", func() {
		session1 := podmanTest.RunTopContainer("test1")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid1 := session1.OutputToString()

		session1 = podmanTest.RunTopContainer("test2")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid2 := session1.OutputToString()

		session1 = podmanTest.RunTopContainer("test3")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid3 := session1.OutputToString()
		shortCid3 := cid3[0:5]

		session1 = podmanTest.RunTopContainerWithArgs("test4", []string{"--label", "test=with,comma"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid4 := session1.OutputToString()

		session1 = podmanTest.Podman([]string{"rm", cid1, "-f", "--filter", "status=running"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitWithError(125, "--filter takes no arguments"))

		session1 = podmanTest.Podman([]string{"rm", "-a", "-f", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEmpty())

		session1 = podmanTest.Podman([]string{"rm", "-a", "-f", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"rm", "-f", "--filter", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))

		session1 = podmanTest.Podman([]string{"rm", "-f", "--filter", "label=test=with,comma"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid4))
	})

	It("podman rm -fa with dependencies", func() {
		ctr1Name := "ctr1"
		ctr1 := podmanTest.RunTopContainer(ctr1Name)
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())
		cid1 := ctr1.OutputToString()

		ctr2 := podmanTest.Podman([]string{"run", "-d", "--network", fmt.Sprintf("container:%s", ctr1Name), ALPINE, "top"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())
		cid2 := ctr2.OutputToString()

		rm := podmanTest.Podman([]string{"rm", "-t0", "-fa"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())
		Expect(rm.ErrorToString()).To(BeEmpty(), "rm -fa error logged")
		Expect(rm.OutputToStringArray()).Should(ConsistOf(cid1, cid2))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})
})
