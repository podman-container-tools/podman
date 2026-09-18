//go:build linux || freebsd

package integration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman wait", func() {
	It("podman wait on bogus container", func() {
		session := podmanTest.Podman([]string{"wait", "1234"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "1234" found: no such container`))
	})

	It("podman wait on a stopped container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "ls"})
		session.Wait(10)
		cid := session.OutputToString()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"wait", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman wait on a sleeping container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		cid := session.OutputToString()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"wait", cid})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
	})

	It("podman wait on latest container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		if IsRemote() {
			session = podmanTest.Podman([]string{"wait", session.OutputToString()})
		} else {
			session = podmanTest.Podman([]string{"wait", "-l"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman container wait on latest container", func() {
		session := podmanTest.Podman([]string{"container", "run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		if IsRemote() {
			session = podmanTest.Podman([]string{"container", "wait", session.OutputToString()})
		} else {
			session = podmanTest.Podman([]string{"container", "wait", "-l"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman container wait on latest container with --interval flag", func() {
		session := podmanTest.Podman([]string{"container", "run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "wait", "-i", "5000", session.OutputToString()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman container wait on latest container with --interval flag", func() {
		session := podmanTest.Podman([]string{"container", "run", "-d", ALPINE, "sleep", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "wait", "--interval", "1s", session.OutputToString()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman container wait on container with bogus --interval", func() {
		session := podmanTest.Podman([]string{"container", "run", "-d", ALPINE, "sleep", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "wait", "--interval", "100days", session.OutputToString()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `time: unknown unit "days" in duration "100days"`))
	})

	It("podman wait on three containers", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		cid1 := session.OutputToString()
		session = podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		cid2 := session.OutputToString()
		session = podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		cid3 := session.OutputToString()
		session = podmanTest.Podman([]string{"wait", cid1, cid2, cid3})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(Equal([]string{"0", "0", "0"}))
	})

	It("podman wait on multiple conditions", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "100"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		// condition should return once nay of the condition is met not all of them,
		// as the container is running this should return immediately
		// https://github.com/containers/podman-py/issues/425
		session = podmanTest.Podman([]string{"wait", "--condition", "running,exited", cid})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("-1"))
	})

	It("podman wait for first return container", func() {
		session1 := podmanTest.PodmanExitCleanly("run", "-d", ALPINE, "sh", "-c", "sleep 100; exit 1")
		cid1 := session1.OutputToString()

		session2 := podmanTest.PodmanExitCleanly("run", "-d", ALPINE, "sh", "-c", "sleep 3; exit 2")
		cid2 := session2.OutputToString()

		waitSession := podmanTest.PodmanExitCleanly("wait", "--exit-first-match", "--condition", "exited", cid1, cid2)
		waitSession.Wait(10)
		Expect(waitSession.OutputToString()).To(Equal("2"))
	})

	It("podman wait --condition=exited on never-started container returns immediately", func() {
		// Documents the long-standing semantic: a created-but-never-started
		// container is "not running" right now, so --condition=exited and
		// --condition=stopped return immediately with 0. Users that want to
		// block until the container actually runs and exits must use
		// --condition=next-exit.
		podmanTest.PodmanExitCleanly("create", "--name", "never_started", ALPINE, "sh", "-c", "exit 7")

		session := podmanTest.PodmanExitCleanly("wait", "--condition=exited", "never_started")
		Expect(session.OutputToString()).To(Equal("0"))

		session = podmanTest.PodmanExitCleanly("wait", "--condition=stopped", "never_started")
		Expect(session.OutputToString()).To(Equal("0"))
	})

	It("podman wait --condition=next-exit waits for an actual exit", func() {
		podmanTest.PodmanExitCleanly("create", "--name", "next_exit_ctr", ALPINE, "sh", "-c", "exit 7")

		// Start the wait command — it should block until the container actually exits.
		waitSession := podmanTest.Podman([]string{"wait", "--condition=next-exit", "next_exit_ctr"})

		// Give wait a moment to subscribe before starting the container.
		time.Sleep(500 * time.Millisecond)
		podmanTest.PodmanExitCleanly("start", "next_exit_ctr")

		waitSession.WaitWithDefaultTimeout()
		Expect(waitSession).Should(ExitCleanly())
		Expect(waitSession.OutputToString()).To(Equal("7"))
	})

	It("podman wait --condition=next-exit ignores current state for a running container", func() {
		runSession := podmanTest.PodmanExitCleanly("run", "-d", "--name", "sleeper", ALPINE, "sleep", "60")
		Expect(runSession.OutputToString()).ToNot(BeEmpty())

		waitSession := podmanTest.Podman([]string{"wait", "--condition=next-exit", "sleeper"})
		time.Sleep(500 * time.Millisecond)

		// Stop the container — wait should return after this, not immediately.
		podmanTest.PodmanExitCleanly("stop", "-t", "0", "sleeper")

		waitSession.WaitWithDefaultTimeout()
		Expect(waitSession).Should(ExitCleanly())
		// Container was killed by SIGKILL on stop -t 0 → exit code 137.
		Expect(waitSession.OutputToString()).ToNot(Equal("137"))
	})

	It("podman wait --condition=not-running on never-started container returns 0", func() {
		// Docker semantic: a container that has never run is "not running"
		// right now, so wait returns immediately with exit code 0.
		podmanTest.PodmanExitCleanly("create", "--name", "not_running_ctr", ALPINE, "ls")

		session := podmanTest.PodmanExitCleanly("wait", "--condition=not-running", "not_running_ctr")
		Expect(session.OutputToString()).To(Equal("0"))
	})

	It("podman wait --condition=not-running on stopped container returns the real exit code", func() {
		runSession := podmanTest.Podman([]string{"run", "--name", "not_running_exited_ctr", ALPINE, "sh", "-c", "exit 5"})
		runSession.WaitWithDefaultTimeout()
		Expect(runSession).Should(ExitWithError(5, ""))

		session := podmanTest.PodmanExitCleanly("wait", "--condition=not-running", "not_running_exited_ctr")
		Expect(session.OutputToString()).To(Equal("5"))
	})

	It("podman wait --condition=removed waits for container removal", func() {
		runSession := podmanTest.PodmanExitCleanly("run", "-d", "--name", "removed_ctr", ALPINE, "sleep", "60")
		Expect(runSession.OutputToString()).ToNot(BeEmpty())

		waitSession := podmanTest.Podman([]string{"wait", "--condition=removed", "removed_ctr"})
		time.Sleep(500 * time.Millisecond)

		// -t 0 skips SIGTERM (sleep here ignores it, which would produce a
		// "resorting to SIGKILL" warning on stderr and trip PodmanExitCleanly).
		podmanTest.PodmanExitCleanly("rm", "-f", "-t", "0", "removed_ctr")

		waitSession.WaitWithDefaultTimeout()
		Expect(waitSession).Should(ExitCleanly())
	})

	It("podman wait --condition=created matches both internal states", func() {
		// Configured state.
		podmanTest.PodmanExitCleanly("create", "--name", "created_ctr_1", ALPINE, "sleep", "60")
		session := podmanTest.PodmanExitCleanly("wait", "--condition=created", "created_ctr_1")
		Expect(session.OutputToString()).To(Equal("-1"))

		// init transitions to ContainerStateCreated (libpod "initialized");
		// the CLI should still match because users see "created" for both.
		podmanTest.PodmanExitCleanly("create", "--name", "created_ctr_2", ALPINE, "sleep", "60")
		podmanTest.PodmanExitCleanly("init", "created_ctr_2")
		session = podmanTest.PodmanExitCleanly("wait", "--condition=created", "created_ctr_2")
		Expect(session.OutputToString()).To(Equal("-1"))
	})
})
