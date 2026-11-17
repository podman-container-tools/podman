//go:build linux || freebsd

package integration

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman generate kube", func() {
	It("podman empty security labels", func() {
		test1 := podmanTest.Podman([]string{"create", "--label", "io.containers.capabilities=", "--name", "test1", "alpine", "echo", "test1"})
		test1.WaitWithDefaultTimeout()
		Expect(test1).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		ctr := inspect.InspectContainerToJSON()
		Expect(ctr[0].EffectiveCaps).To(BeNil())

		test2 := podmanTest.Podman([]string{"run", "--label", "io.containers.capabilities=", "alpine", "grep", "^CapEff", "/proc/self/status"})
		test2.WaitWithDefaultTimeout()
		Expect(test2.OutputToString()).To(ContainSubstring("0000000000000000"))
	})

	It("podman security labels", func() {
		test1 := podmanTest.Podman([]string{"create", "--label", "io.containers.capabilities=setuid,setgid", "--name", "test1", "alpine", "echo", "test1"})
		test1.WaitWithDefaultTimeout()
		Expect(test1).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		ctr := inspect.InspectContainerToJSON()
		caps := strings.Join(ctr[0].EffectiveCaps, ",")
		Expect(caps).To(Equal("CAP_SETGID,CAP_SETUID"))
	})

	It("podman bad security labels", func() {
		test1 := podmanTest.Podman([]string{"create", "--label", "io.containers.capabilities=sys_admin", "--name", "test1", "alpine", "echo", "test1"})
		test1.WaitWithDefaultTimeout()
		Expect(test1).Should(Exit(0))
		stderr := test1.ErrorToString()
		if IsRemote() {
			Expect(stderr).To(BeEmpty())
		} else {
			Expect(stderr).To(ContainSubstring("Capabilities requested by user or image are not allowed by default: \\\"CAP_SYS_ADMIN\\\""))
		}

		inspect := podmanTest.Podman([]string{"inspect", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		ctr := inspect.InspectContainerToJSON()
		caps := strings.Join(ctr[0].EffectiveCaps, ",")
		Expect(caps).To(Not(Equal("CAP_SYS_ADMIN")))
	})

	It("podman --cap-add sys_admin security labels", func() {
		test1 := podmanTest.Podman([]string{"create", "--cap-add", "SYS_ADMIN", "--label", "io.containers.capabilities=sys_admin", "--name", "test1", "alpine", "echo", "test1"})
		test1.WaitWithDefaultTimeout()
		Expect(test1).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		ctr := inspect.InspectContainerToJSON()
		caps := strings.Join(ctr[0].EffectiveCaps, ",")
		Expect(caps).To(Equal("CAP_SYS_ADMIN"))
	})

	It("podman --cap-drop all sys_admin security labels", func() {
		test1 := podmanTest.Podman([]string{"create", "--cap-drop", "all", "--label", "io.containers.capabilities=sys_admin", "--name", "test1", "alpine", "echo", "test1"})
		test1.WaitWithDefaultTimeout()
		Expect(test1).Should(Exit(0))
		stderr := test1.ErrorToString()
		if IsRemote() {
			Expect(stderr).To(BeEmpty())
		} else {
			Expect(stderr).To(ContainSubstring("Capabilities requested by user or image are not allowed by default: \\\"CAP_SYS_ADMIN\\\""))
		}

		inspect := podmanTest.Podman([]string{"inspect", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		ctr := inspect.InspectContainerToJSON()
		caps := strings.Join(ctr[0].EffectiveCaps, ",")
		Expect(caps).To(Equal(""))
	})

	It("podman security labels from image", func() {
		test1 := podmanTest.Podman([]string{"create", "--name", "test1", "alpine", "echo", "test1"})
		test1.WaitWithDefaultTimeout()
		Expect(test1).Should(ExitCleanly())

		commit := podmanTest.Podman([]string{"commit", "-q", "-c", "label=io.containers.capabilities=setgid,setuid", "test1", "image1"})
		commit.WaitWithDefaultTimeout()
		Expect(commit).Should(ExitCleanly())

		image1 := podmanTest.Podman([]string{"create", "--name", "test2", "image1", "echo", "test1"})
		image1.WaitWithDefaultTimeout()
		Expect(image1).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "test2"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		ctr := inspect.InspectContainerToJSON()
		caps := strings.Join(ctr[0].EffectiveCaps, ",")
		Expect(caps).To(Equal("CAP_SETGID,CAP_SETUID"))
	})

	It("podman --privileged security labels", func() {
		pull := podmanTest.Podman([]string{"create", "--privileged", "--label", "io.containers.capabilities=setuid,setgid", "--name", "test1", "alpine", "echo", "test"})
		pull.WaitWithDefaultTimeout()
		Expect(pull).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		ctr := inspect.InspectContainerToJSON()
		caps := strings.Join(ctr[0].EffectiveCaps, ",")
		Expect(caps).To(Not(Equal("CAP_SETUID,CAP_SETGID")))
	})

	It("podman container runlabel (podman --version)", func() {
		SkipIfRemote("runlabel not supported on podman-remote")
		PodmanDockerfile := fmt.Sprintf(`
FROM  %s
LABEL io.containers.capabilities=chown,kill`, ALPINE)

		image := "podman-caps:podman"
		podmanTest.BuildImage(PodmanDockerfile, image, "false")

		test1 := podmanTest.Podman([]string{"create", "--name", "test1", image, "echo", "test1"})
		test1.WaitWithDefaultTimeout()
		Expect(test1).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		ctr := inspect.InspectContainerToJSON()
		caps := strings.Join(ctr[0].EffectiveCaps, ",")
		Expect(caps).To(Equal("CAP_CHOWN,CAP_KILL"))
	})
})
