//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("podman rename", func() {
	It("podman rename on non-existent container", func() {
		session := podmanTest.Podman([]string{"rename", "doesNotExist", "aNewName"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `no container with name or ID "doesNotExist" found: no such container`))
	})

	It("Podman rename on existing container with bad name", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		newName := "invalid<>:char"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).To(ExitWithError(125, "names must match [a-zA-Z0-9][a-zA-Z0-9_.-]*: invalid argument"))

		ps := podmanTest.Podman([]string{"ps", "-aq", "--filter", fmt.Sprintf("name=%s", ctrName), "--format", "{{ .Names }}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring(ctrName))
	})

	It("Successfully rename a created container", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		newName := "aNewName"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(ExitCleanly())

		ps := podmanTest.Podman([]string{"ps", "-aq", "--filter", fmt.Sprintf("name=%s", newName), "--format", "{{ .Names }}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring(newName))
	})

	It("Successfully rename a created container and test event generated", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		newName := "aNewName"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "container=aNewName"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("rename"))
	})

	It("Successfully rename a running container", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		newName := "aNewName"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(ExitCleanly())

		ps := podmanTest.Podman([]string{"ps", "-aq", "--filter", fmt.Sprintf("name=%s", newName), "--format", "{{ .Names }}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring(newName))
	})

	It("Rename a running container with exec sessions", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec := podmanTest.Podman([]string{"exec", "-d", ctrName, "top"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())

		newName := "aNewName"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(ExitCleanly())

		ps := podmanTest.Podman([]string{"ps", "-aq", "--filter", fmt.Sprintf("name=%s", newName), "--format", "{{ .Names }}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring(newName))
	})

	It("Rename a container that is part of a pod", func() {
		podName := "testPod"
		infraName := "infra1"
		pod := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--infra-name", infraName})
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(ExitCleanly())

		infraName2 := "infra2"
		rename := podmanTest.Podman([]string{"rename", infraName, infraName2})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(ExitCleanly())

		remove := podmanTest.Podman([]string{"pod", "rm", "-f", podName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(ExitCleanly())

		create := podmanTest.Podman([]string{"create", "--name", infraName2, ALPINE, "top"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		create2 := podmanTest.Podman([]string{"create", "--name", infraName, ALPINE, "top"})
		create2.WaitWithDefaultTimeout()
		Expect(create2).Should(ExitCleanly())
	})
})
