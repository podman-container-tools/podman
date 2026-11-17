//go:build linux || freebsd

package integration

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/libpod/define"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman init containers", func() {
	It("podman create init container without --pod should fail", func() {
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "must specify pod value with init-ctr"))
	})

	It("podman create init container with bad init type should fail", func() {
		session := podmanTest.Podman([]string{"create", "--init-ctr", "unknown", "--pod", "new:foobar", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "init-ctr value must be 'always' or 'once'"))
	})

	It("podman init containers should not degrade pod status", func() {
		// create a pod
		topPod := podmanTest.Podman([]string{"create", "-t", "--pod", "new:foobar", ALPINE, "top"})
		topPod.WaitWithDefaultTimeout()
		Expect(topPod).Should(ExitCleanly())
		// add an init container
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "foobar", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// start a pod
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"pod", "inspect", "foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		data := inspect.InspectPodToJSON()
		Expect(data).To(HaveField("State", define.PodStateRunning))
	})

	It("podman create init container should fail in running pod", func() {
		// create a running pod
		topPod := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:foobar", ALPINE, "top"})
		topPod.WaitWithDefaultTimeout()
		Expect(topPod).Should(ExitCleanly())
		// adding init-ctr to running pod should fail
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "foobar", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "cannot add init-ctr to a running pod"))
	})

	It("podman make sure init container runs before pod containers", func() {
		filename := filepath.Join("/dev/shm", RandomString(12))
		content := RandomString(16)
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "new:foobar", ALPINE, "bin/sh", "-c", fmt.Sprintf("echo %s > %s", content, filename)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		verify := podmanTest.Podman([]string{"create", "--pod", "foobar", "-t", ALPINE, "top"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(ExitCleanly())
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())
		checkLog := podmanTest.Podman([]string{"exec", verify.OutputToString(), "cat", filename})
		checkLog.WaitWithDefaultTimeout()
		Expect(checkLog).Should(ExitCleanly())
		Expect(checkLog.OutputToString()).To(Equal(content))
	})

	It("podman make sure once container is removed", func() {
		filename := filepath.Join("/dev/shm", RandomString(12))
		content := RandomString(16)
		session := podmanTest.Podman([]string{"create", "--init-ctr", "once", "--pod", "new:foobar", ALPINE, "bin/sh", "-c", fmt.Sprintf("echo %s > %s", content, filename)})
		session.WaitWithDefaultTimeout()
		initContainerID := session.OutputToString()
		Expect(session).Should(ExitCleanly())
		verify := podmanTest.Podman([]string{"create", "--pod", "foobar", "-t", ALPINE, "top"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(ExitCleanly())
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())
		check := podmanTest.Podman([]string{"container", "exists", initContainerID})
		check.WaitWithDefaultTimeout()
		// Container was rm'd
		Expect(check).To(ExitWithError(1, ""))

		// Let's double check with a stop and start
		podmanTest.StopPod("foobar")
		startPod := podmanTest.Podman([]string{"pod", "start", "foobar"})
		startPod.WaitWithDefaultTimeout()
		Expect(startPod).Should(ExitCleanly())

		// Because no init was run, the file should not even exist
		doubleCheck := podmanTest.Podman([]string{"exec", verify.OutputToString(), "cat", filename})
		doubleCheck.WaitWithDefaultTimeout()
		Expect(doubleCheck).Should(ExitWithError(1, fmt.Sprintf("cat: can't open '%s': No such file or directory", filename)))
	})

	It("podman ensure always init containers always run", func() {
		filename := filepath.Join("/dev/shm", RandomString(12))

		// Write the date to a file
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "new:foobar", FEDORA_MINIMAL, "/bin/sh", "-c", "date +%T.%N > " + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		verify := podmanTest.Podman([]string{"create", "--pod", "foobar", "-t", ALPINE, "top"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(ExitCleanly())
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())

		// capture the date written
		checkLog := podmanTest.Podman([]string{"exec", verify.OutputToString(), "cat", filename})
		checkLog.WaitWithDefaultTimeout()
		firstResult := checkLog.OutputToString()
		Expect(checkLog).Should(ExitCleanly())

		// Stop and start the pod
		podmanTest.StopPod("foobar")
		startPod := podmanTest.Podman([]string{"pod", "start", "foobar"})
		startPod.WaitWithDefaultTimeout()
		Expect(startPod).Should(ExitCleanly())

		// Check the file again with exec
		secondCheckLog := podmanTest.Podman([]string{"exec", verify.OutputToString(), "cat", filename})
		secondCheckLog.WaitWithDefaultTimeout()
		Expect(secondCheckLog).Should(ExitCleanly())

		// Dates should not match
		Expect(firstResult).ToNot(Equal(secondCheckLog.OutputToString()))
	})
})
