//go:build linux || freebsd

package integration

import (
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

// TODO: we need to check the output. Currently, we only check the exit codes
// which is not enough.
var _ = Describe("Podman stats", func() {
	It("podman stats with bogus container", func() {
		session := podmanTest.Podman([]string{"stats", "--no-stream", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `unable to get list of containers: unable to look up container 123: no container with name or ID "123" found: no such container`))
	})

	It("podman stats on a running container", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"stats", "--no-stream", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman stats on all containers", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman stats on all running containers", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"stats", "--no-stream"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman stats only output cids", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"stats", "--all", "--no-trunc", "--no-stream", "--format", "\"{{.ID}}\""})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(len(session.OutputToStringArray()[0])).Should(BeEquivalentTo(66))
	})

	It("podman stats with GO template", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--format", "table {{.ID}} {{.AVGCPU}} {{.MemUsage}} {{.CPU}} {{.NetIO}} {{.BlockIO}} {{.PIDS}}"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitCleanly())
	})

	It("podman stats with invalid GO template", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--format", "\"table {{.ID}} {{.NoSuchField}} \""})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitWithError(125, `template: stats:1:28: executing "stats" at <.NoSuchField>: can't evaluate field NoSuchField in type containers.containerStats`))
	})

	It("podman stats with negative interval", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--interval=-1"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitWithError(125, "invalid interval, must be a positive number greater zero"))
	})

	It("podman stats with zero interval", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--interval=0"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitWithError(125, "invalid interval, must be a positive number greater zero"))
	})

	It("podman stats with interval", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--interval=5"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
	})

	It("podman stats with json output", func() {
		var found bool
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		for range 5 {
			ps := podmanTest.Podman([]string{"ps", "-q"})
			ps.WaitWithDefaultTimeout()
			if len(ps.OutputToStringArray()) == 1 {
				found = true
				break
			}
			time.Sleep(time.Second)
		}
		Expect(found).To(BeTrue(), "container has started")
		stats := podmanTest.Podman([]string{"stats", "--all", "--no-stream", "--format", "json"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
		Expect(stats.OutputToString()).To(BeValidJSON())
	})

	It("podman stats on a container with no net ns", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--net", "none", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman stats on a container that joined another's net ns", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "-d", "--net", fmt.Sprintf("container:%s", cid), ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	// Regression test for #8265
	It("podman stats with custom memory limits", func() {
		// Run three containers. One with a memory limit.  Make sure
		// that the limits are different and the limited one has a
		// lower limit.
		ctrNoLimit0 := "no-limit-0"
		ctrNoLimit1 := "no-limit-1"
		ctrWithLimit := "with-limit"

		session := podmanTest.Podman([]string{"run", "-d", "--name", ctrNoLimit0, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--name", ctrNoLimit1, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--name", ctrWithLimit, "--memory", "50m", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"stats", "--no-stream", "--format", "{{.MemLimit}}", ctrNoLimit0, ctrNoLimit1, ctrWithLimit})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// We have three containers.  The unlimited ones need to have
		// the same limit, the limited one a lower one.
		limits := session.OutputToStringArray()
		Expect(limits).To(HaveLen(3))
		Expect(limits[0]).To(Equal(limits[1]))
		Expect(limits[0]).ToNot(Equal(limits[2]))

		defaultLimit, err := strconv.Atoi(limits[0])
		Expect(err).ToNot(HaveOccurred())
		customLimit, err := strconv.Atoi(limits[2])
		Expect(err).ToNot(HaveOccurred())

		Expect(customLimit).To(BeNumerically("<", defaultLimit))
	})

	It("podman stats with a container that is not running", func() {
		ctr := "created_container"
		session := podmanTest.Podman([]string{"create", "--name", ctr, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"stats", "--no-stream", ctr})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman stats show cgroup memory limit", func() {
		ctrWithLimit := "with-limit"

		session := podmanTest.Podman([]string{"run", "-d", "--name", ctrWithLimit, "--memory", "50m", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"stats", "--no-stream", "--format", "{{.MemLimit}}", ctrWithLimit})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		limit, err := strconv.Atoi(session.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		Expect(limit).To(BeNumerically("==", 50*1024*1024))

		session = podmanTest.Podman([]string{"container", "update", ctrWithLimit, "--memory", "100m"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"stats", "--no-stream", "--format", "{{.MemLimit}}", ctrWithLimit})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		limit, err = strconv.Atoi(session.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		Expect(limit).To(BeNumerically("==", 100*1024*1024))
	})

	It("podman stats --all", func() {
		runningContainersession := podmanTest.RunTopContainer("")
		runningContainersession.WaitWithDefaultTimeout()
		Expect(runningContainersession).Should(ExitCleanly())
		runningCtrID := runningContainersession.OutputToString()[0:12]

		createdContainerSession, _, _ := podmanTest.CreatePod(map[string][]string{
			"--infra": {"true"},
		})

		createdContainerSession.WaitWithDefaultTimeout()
		Expect(createdContainerSession).Should(ExitCleanly())

		sessionAll := podmanTest.Podman([]string{"stats", "--no-stream", "--format", "{{.ID}}"})
		sessionAll.WaitWithDefaultTimeout()
		Expect(sessionAll).Should(ExitCleanly())
		Expect(sessionAll.OutputToString()).Should(Equal(runningCtrID))

		sessionAll = podmanTest.Podman([]string{"stats", "--no-stream", "--all=false", "--format", "{{.ID}}"})
		sessionAll.WaitWithDefaultTimeout()
		Expect(sessionAll).Should(ExitCleanly())
		Expect(sessionAll.OutputToString()).Should(Equal(runningCtrID))

		sessionAll = podmanTest.Podman([]string{"stats", "--all=true", "--no-stream", "--format", "{{.ID}}"})
		sessionAll.WaitWithDefaultTimeout()
		Expect(sessionAll).Should(ExitCleanly())
		Expect(sessionAll.OutputToStringArray()).Should(HaveLen(2))

		sessionAll = podmanTest.Podman([]string{"stats", "--all", "--no-stream", "--format", "{{.ID}}"})
		sessionAll.WaitWithDefaultTimeout()
		Expect(sessionAll).Should(ExitCleanly())
		Expect(sessionAll.OutputToStringArray()).Should(HaveLen(2))
	})
})
