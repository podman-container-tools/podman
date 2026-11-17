//go:build linux || freebsd

package integration

import (
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman restart", func() {
	It("podman restart bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "123" found: no such container`))
	})

	It("podman restart stopped container by name", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("podman restart stopped container by ID", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", cid})
		startTime.WaitWithDefaultTimeout()

		startSession := podmanTest.Podman([]string{"start", "--attach", cid})
		startSession.WaitWithDefaultTimeout()
		Expect(startSession).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"restart", cid})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", cid})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("podman restart running container", func() {
		_ = podmanTest.RunTopContainer("test1")
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("podman container restart running container", func() {
		_ = podmanTest.RunTopContainer("test1")
		startTime := podmanTest.Podman([]string{"container", "inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"container", "restart", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		restartTime := podmanTest.Podman([]string{"container", "inspect", "--format='{{.State.StartedAt}}'", "test1"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("podman restart multiple containers", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		_, exitCode, _ = podmanTest.RunLsContainer("test2")
		Expect(exitCode).To(Equal(0))
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "test1", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Not(Equal(startTime.OutputToStringArray()[0])))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("podman restart the latest container", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		_, exitCode, _ = podmanTest.RunLsContainer("test2")
		Expect(exitCode).To(Equal(0))

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		cid := "-l"
		if IsRemote() {
			cid = "test2"
		}
		session := podmanTest.Podman([]string{"restart", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Equal(startTime.OutputToStringArray()[0]))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("podman restart non-stop container with short timeout", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", "--env", "STOPSIGNAL=SIGKILL", ALPINE, "sleep", "999"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		startTime := time.Now()
		session = podmanTest.Podman([]string{"restart", "-t", "2", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		timeSince := time.Since(startTime)
		Expect(timeSince).To(BeNumerically("<", 10*time.Second))
		Expect(timeSince).To(BeNumerically(">", 2*time.Second))
		stderr := session.ErrorToString()
		if IsRemote() {
			Expect(stderr).To(BeEmpty())
		} else {
			Expect(stderr).To(ContainSubstring("StopSignal SIGTERM failed to stop container test1 in 2 seconds, resorting to SIGKILL"))
		}
	})

	It("podman restart --all", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		test2 := podmanTest.RunTopContainer("test2")
		test2.WaitWithDefaultTimeout()
		Expect(test2).Should(ExitCleanly())

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Not(Equal(startTime.OutputToStringArray()[0])))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("podman restart --all --running", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		test2 := podmanTest.RunTopContainer("test2")
		test2.WaitWithDefaultTimeout()
		Expect(test2).Should(ExitCleanly())

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "-a", "--running"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Equal(startTime.OutputToStringArray()[0]))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("podman restart a container in a pod and hosts should not duplicated", func() {
		// Fixes: https://github.com/containers/podman/issues/8921

		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("host-restart-test", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		testCmd := []string{"exec", "host-restart-test", "cat", "/etc/hosts"}

		// before restart
		beforeRestart := podmanTest.Podman(testCmd)
		beforeRestart.WaitWithDefaultTimeout()
		Expect(beforeRestart).Should(ExitCleanly())
		nHostLines := len(beforeRestart.OutputToStringArray())

		session = podmanTest.Podman([]string{"restart", "host-restart-test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		afterRestart := podmanTest.Podman(testCmd)
		afterRestart.WaitWithDefaultTimeout()
		Expect(afterRestart).Should(ExitCleanly())

		// line count should be equal
		Expect(afterRestart.OutputToStringArray()).To(HaveLen(nHostLines),
			"number of host lines post-restart == number of lines pre-restart")
	})

	It("podman restart all stopped containers with --all", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		session = podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))

		podmanTest.StopContainer("--all")
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		session = podmanTest.Podman([]string{"restart", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("podman restart --cidfile", func() {
		cidFile := filepath.Join(tempdir, "cid")

		session := podmanTest.Podman([]string{"create", "--cidfile", cidFile, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToStringArray()[0]

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"restart", "--cidfile", cidFile})
		result.WaitWithDefaultTimeout()
		// FIXME - #20196: Cannot use ExitCleanly()
		Expect(result).Should(Exit(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
	})

	It("podman restart multiple --cidfile", func() {
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

		result := podmanTest.Podman([]string{"restart", "--cidfile", cidFile1, "--cidfile", cidFile2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))
	})

	It("podman restart invalid --latest and --cidfile and --all", func() {
		SkipIfRemote("--latest flag n/a")
		result := podmanTest.Podman([]string{"restart", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"restart", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"restart", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"restart", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--all and --latest cannot be used together"))
	})

	It("podman restart --filter", func() {
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

		session1 = podmanTest.Podman([]string{"restart", cid1, "-f", "status=test"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitWithError(125, "--filter takes no arguments"))

		session1 = podmanTest.Podman([]string{"restart", "-a", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEmpty())

		session1 = podmanTest.Podman([]string{"restart", "-a", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"restart", "-a", "--filter", "label=test=with,comma"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"restart", "-f", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))
	})
})
