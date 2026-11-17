//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pause", func() {
	pausedState := "paused"
	createdState := "created"

	BeforeEach(func() {
		b, err := os.ReadFile("/proc/self/cgroup")
		if err != nil {
			Skip("cannot read self cgroup")
		}

		path := filepath.Join("/sys/fs/cgroup", strings.TrimSuffix(strings.Replace(string(b), "0::", "", 1), "\n"), "cgroup.freeze")
		_, err = os.Stat(path)
		if err != nil {
			Skip("freezer controller not available on the current kernel")
		}
	})

	It("podman pause bogus container", func() {
		session := podmanTest.Podman([]string{"pause", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `no container with name or ID "foobar" found: no such container`))
	})

	It("podman unpause bogus container", func() {
		session := podmanTest.Podman([]string{"unpause", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `no container with name or ID "foobar" found: no such container`))
	})

	It("podman pause a created container by id", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).To(ExitWithError(125, `"created" is not running, can't pause: container state improper`))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(createdState))

		// check we can read stats for a paused container
		result = podmanTest.Podman([]string{"stats", "--no-stream", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman pause a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"unpause", cid})
		result.WaitWithDefaultTimeout()
	})

	It("podman container pause a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"container", "unpause", cid})
		result.WaitWithDefaultTimeout()
	})

	It("podman unpause a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"unpause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitWithError(125, fmt.Sprintf(`"%s" is not paused, can't unpause: container state improper`, cid)))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman remove a paused container by id without force", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitWithError(2, fmt.Sprintf("cannot remove container %s as it is paused - running or paused containers cannot be removed without force: container state improper", cid)))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		// unpause so that the cleanup can stop the container,
		// otherwise it fails with container state improper
		session = podmanTest.Podman([]string{"unpause", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman remove a paused container by id with force", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "--force", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman stop a paused container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"stop", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitWithError(125, fmt.Sprintf("Error: container %s is running or paused, refusing to clean up: container state improper", cid)))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"unpause", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(2, fmt.Sprintf("cannot remove container %s as it is running - running or paused containers cannot be removed without force: container state improper", cid)))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pause a running container by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pause", "test1"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(Equal(pausedState))

		result = podmanTest.Podman([]string{"unpause", "test1"})
		result.WaitWithDefaultTimeout()
	})

	It("podman pause a running container by id and another by name", func() {
		session1 := podmanTest.RunTopContainer("test1")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		session2 := podmanTest.RunTopContainer("")
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		cid2 := session2.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid2})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"pause", "test1"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{"unpause", "test1"})
		result.WaitWithDefaultTimeout()
		result = podmanTest.Podman([]string{"unpause", cid2})
		result.WaitWithDefaultTimeout()
	})

	It("Pause all containers (no containers exist)", func() {
		result := podmanTest.Podman([]string{"pause", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("Unpause all containers (no paused containers exist)", func() {
		result := podmanTest.Podman([]string{"unpause", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("Pause a bunch of running containers", func() {
		for i := range 3 {
			name := fmt.Sprintf("test%d", i)
			run := podmanTest.Podman([]string{"run", "-dt", "--name", name, NGINX_IMAGE})
			run.WaitWithDefaultTimeout()
			Expect(run).Should(ExitCleanly())

		}
		running := podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running).Should(ExitCleanly())
		Expect(running.OutputToStringArray()).To(HaveLen(3))

		pause := podmanTest.Podman([]string{"pause", "--all"})
		pause.WaitWithDefaultTimeout()
		Expect(pause).Should(ExitCleanly())

		running = podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running).Should(ExitCleanly())
		Expect(running.OutputToStringArray()).To(BeEmpty())

		unpause := podmanTest.Podman([]string{"unpause", "--all"})
		unpause.WaitWithDefaultTimeout()
		Expect(unpause).Should(ExitCleanly())
	})

	It("Unpause a bunch of running containers", func() {
		for i := range 3 {
			name := fmt.Sprintf("test%d", i)
			run := podmanTest.Podman([]string{"run", "-dt", "--name", name, NGINX_IMAGE})
			run.WaitWithDefaultTimeout()
			Expect(run).Should(ExitCleanly())

		}
		pause := podmanTest.Podman([]string{"pause", "--all"})
		pause.WaitWithDefaultTimeout()
		Expect(pause).Should(ExitCleanly())

		unpause := podmanTest.Podman([]string{"unpause", "--all"})
		unpause.WaitWithDefaultTimeout()
		Expect(unpause).Should(ExitCleanly())

		running := podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running).Should(ExitCleanly())
		Expect(running.OutputToStringArray()).To(HaveLen(3))
	})

	It("podman pause --latest", func() {
		SkipIfRemote("--latest flag n/a")
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		result := podmanTest.Podman([]string{"pause", "-l"})
		result.WaitWithDefaultTimeout()

		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"unpause", "-l"})
		result.WaitWithDefaultTimeout()
	})

	It("podman pause --cidfile", func() {
		cidFile := filepath.Join(tempdir, "cid")

		session := podmanTest.Podman([]string{"create", "--cidfile", cidFile, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToStringArray()[0]

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pause", "--cidfile", cidFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))

		result = podmanTest.Podman([]string{"unpause", "--cidfile", cidFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output = result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
	})

	It("podman pause multiple --cidfile", func() {
		cidFile1 := filepath.Join(tempdir, "cid-1")
		cidFile2 := filepath.Join(tempdir, "cid-2")

		session := podmanTest.Podman([]string{"run", "--cidfile", cidFile1, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid1 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"run", "--cidfile", cidFile2, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid2 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		result := podmanTest.Podman([]string{"pause", "--cidfile", cidFile1, "--cidfile", cidFile2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		result = podmanTest.Podman([]string{"unpause", "--cidfile", cidFile1, "--cidfile", cidFile2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output = result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))
	})

	It("podman pause invalid --latest and --cidfile and --all", func() {
		SkipIfRemote("--latest flag n/a")
		result := podmanTest.Podman([]string{"pause", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"pause", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"pause", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"pause", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all and --latest cannot be used together"))
	})

	It("podman unpause invalid --latest and --cidfile and --all", func() {
		SkipIfRemote("--latest flag n/a")
		result := podmanTest.Podman([]string{"unpause", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"unpause", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"unpause", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"unpause", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all and --latest cannot be used together"))
	})

	It("podman pause --filter", func() {
		session1 := podmanTest.RunTopContainer("")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid1 := session1.OutputToString()

		session1 = podmanTest.RunTopContainer("")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid2 := session1.OutputToString()

		session1 = podmanTest.RunTopContainerWithArgs("", []string{"--label", "test=with,comma"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid3 := session1.OutputToString()
		shortCid3 := cid3[0:5]

		session1 = podmanTest.Podman([]string{"pause", cid1, "-f", "status=test"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitWithError(125, "--filter takes no arguments"))

		session1 = podmanTest.Podman([]string{"unpause", cid1, "-f", "status=paused"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitWithError(125, "--filter takes no arguments"))

		session1 = podmanTest.Podman([]string{"pause", "-a", "--filter", "label=test=with,comma"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"unpause", "-a", "--filter", "label=test=with,comma"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"pause", "-a", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEmpty())

		session1 = podmanTest.Podman([]string{"pause", "-a", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"unpause", "-a", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEmpty())

		session1 = podmanTest.Podman([]string{"unpause", "-a", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"pause", "-f", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))

		session1 = podmanTest.Podman([]string{"unpause", "-f", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))
	})
})
