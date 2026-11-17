//go:build linux || freebsd

package integration

import (
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman stop", func() {
	It("podman stop bogus container", func() {
		session := podmanTest.Podman([]string{"stop", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "foobar" found: no such container`))
	})

	It("podman stop --ignore bogus container", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "--ignore", "foobar", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring(cid))
	})

	It("podman stop container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"stop", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop single container by short id", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		shortID := cid[0:10]

		session = podmanTest.Podman([]string{"stop", shortID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(shortID))
	})

	It("podman stop container by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"stop", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman container stop by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "stop", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop stopped container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"stop", "test1"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())

		session3 := podmanTest.Podman([]string{"stop", "test1"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(ExitCleanly())

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))

		// make sure we only have one cleanup event for this container
		events := podmanTest.Podman([]string{"events", "--since=30s", "--stream=false"})
		events.WaitWithDefaultTimeout()
		Expect(events).Should(ExitCleanly())
		Expect(strings.Count(events.OutputToString(), "container cleanup")).To(Equal(1), "cleanup event should show up exactly once")
	})

	It("podman stop all containers -t", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid1 := session.OutputToString()

		session = podmanTest.RunTopContainer("test2")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid2 := session.OutputToString()

		session = podmanTest.RunTopContainer("test3")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid3 := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(output).To(ContainSubstring(cid3))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop container --time", func() {
		session := podmanTest.RunTopContainer("test4")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"stop", "--time", "0", "test4"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("test4"))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop container --timeout", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test5", ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"stop", "--timeout", "0", "test5"})
		// Without timeout container stops in 10 seconds
		// If not stopped in 5 seconds, then --timeout did not work
		session.Wait(5)
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("test5"))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop container --timeout Warning", func() {
		SkipIfRemote("warning will happen only on server side")
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test5", ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"stop", "--timeout", "1", "test5"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		warning := session.ErrorToString()
		Expect(warning).To(ContainSubstring("StopSignal SIGTERM failed to stop container test5 in 1 seconds, resorting to SIGKILL"))
	})

	It("podman stop latest containers", func() {
		SkipIfRemote("--latest flag n/a")
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "-l", "-t", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(cid))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop all containers with one stopped", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session2 := podmanTest.RunTopContainer("test2")
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		cid := "-l"
		if IsRemote() {
			cid = "test2"
		}
		session3 := podmanTest.Podman([]string{"stop", cid, "-t", "1"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(ExitCleanly())
		session4 := podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session4.WaitWithDefaultTimeout()
		Expect(session4).Should(ExitCleanly())
		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop all containers with one created", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session2 := podmanTest.Podman([]string{"create", ALPINE, "/bin/sh"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		session3 := podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(ExitCleanly())
		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(ExitCleanly())
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop should return silent success on stopping configured containers", func() {
		// following container is not created on OCI runtime
		// so we return success and assume that it is stopped
		session2 := podmanTest.Podman([]string{"create", "--name", "stopctr", ALPINE, "/bin/sh"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		session3 := podmanTest.Podman([]string{"stop", "stopctr"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(ExitCleanly())
	})

	It("podman stop --cidfile", func() {
		cidFile := filepath.Join(tempdir, "cid")

		session := podmanTest.Podman([]string{"create", "--cidfile", cidFile, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToStringArray()[0]

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"stop", "--cidfile", cidFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
	})

	It("podman stop multiple --cidfile", func() {
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

		result := podmanTest.Podman([]string{"stop", "--cidfile", cidFile1, "--cidfile", cidFile2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))
	})

	It("podman stop invalid --latest and --cidfile and --all", func() {
		SkipIfRemote("--latest flag n/a")

		result := podmanTest.Podman([]string{"stop", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"stop", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"stop", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"stop", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all and --latest cannot be used together"))
	})

	It("podman stop --all", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		session = podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))

		session = podmanTest.Podman([]string{"stop", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman stop --ignore", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		session = podmanTest.Podman([]string{"stop", "bogus", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "bogus" found: no such container`))

		session = podmanTest.Podman([]string{"stop", "--ignore", "bogus", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman stop --filter", func() {
		session1 := podmanTest.Podman([]string{"container", "create", ALPINE})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid1 := session1.OutputToString()

		session1 = podmanTest.Podman([]string{"container", "create", ALPINE})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid2 := session1.OutputToString()

		session1 = podmanTest.Podman([]string{"container", "create", ALPINE})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid3 := session1.OutputToString()
		shortCid3 := cid3[0:5]

		session1 = podmanTest.Podman([]string{"container", "create", "--label", "test=with,comma", ALPINE})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid4 := session1.OutputToString()

		session1 = podmanTest.Podman([]string{"start", "--all"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		session1 = podmanTest.Podman([]string{"stop", cid1, "-f", "status=running"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitWithError(125, "--filter takes no arguments"))

		session1 = podmanTest.Podman([]string{"stop", "-a", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEmpty())

		session1 = podmanTest.Podman([]string{"stop", "-a", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"stop", "-a", "--filter", "label=test=with,comma"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid4))

		session1 = podmanTest.Podman([]string{"stop", "-f", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))
	})

	It("podman stop --service sets StoppedByUser to false", func() {
		SkipIfRemote("--service flag is not supported on remote")
		containerName := "test-not-stopped-by-user"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", containerName, ALPINE, "top")

		podmanTest.PodmanExitCleanly("stop", "--service", containerName)

		data := podmanTest.InspectContainer(containerName)
		Expect(data).To(HaveLen(1))
		Expect(data[0].State.StoppedByUser).To(BeFalse())
	})

	It("podman stop without --service flag sets StoppedByUser to true", func() {
		containerName := "test-default-stop"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", containerName, ALPINE, "top")

		podmanTest.PodmanExitCleanly("stop", containerName)

		data := podmanTest.InspectContainer(containerName)
		Expect(data).To(HaveLen(1))
		Expect(data[0].State.StoppedByUser).To(BeTrue())
	})
})
