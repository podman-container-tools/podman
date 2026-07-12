//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman start", func() {
	It("podman start bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "123" found: no such container`))
	})

	It("podman start single container by id", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman start --rm removed on failure", func() {
		session := podmanTest.Podman([]string{"create", "--name=test", "--rm", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "not found in $PATH"))
		session = podmanTest.Podman([]string{"container", "exists", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(1, ""))
	})

	It("podman start --rm --attach removed on failure", func() {
		session := podmanTest.Podman([]string{"create", "--rm", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"start", "--attach", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "not found in $PATH"))
		session = podmanTest.Podman([]string{"container", "exists", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(1, ""))
	})

	It("podman container start single container by id", func() {
		session := podmanTest.Podman([]string{"container", "create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"container", "start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(cid))
	})

	It("podman container start single container by short id", func() {
		session := podmanTest.Podman([]string{"container", "create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		shortID := cid[0:10]
		session = podmanTest.Podman([]string{"container", "start", shortID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(shortID))
	})

	It("podman start single container by name", func() {
		name := "foobar99"
		session := podmanTest.Podman([]string{"create", "--name", name, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"start", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(name))
	})

	It("podman start single container with attach and test the signal", func() {
		session := podmanTest.Podman([]string{"create", "--entrypoint", "sh", ALPINE, "-c", "exit 1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"start", "--attach", cid})
		session.WaitWithDefaultTimeout()
		// It should forward the signal
		Expect(session).Should(ExitWithError(1, ""))
	})

	It("podman start multiple containers", func() {
		session := podmanTest.Podman([]string{"create", "--name", "foobar99", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		cid1 := session.OutputToString()
		session2 := podmanTest.Podman([]string{"create", "--name", "foobar100", ALPINE, "ls"})
		session2.WaitWithDefaultTimeout()
		cid2 := session2.OutputToString()
		session = podmanTest.Podman([]string{"start", cid1, cid2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman start multiple containers with bogus", func() {
		session := podmanTest.Podman([]string{"create", "--name", "foobar99", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		cid1 := session.OutputToString()
		session = podmanTest.Podman([]string{"start", cid1, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "doesnotexist" found: no such container`))
	})

	It("podman multiple containers -- attach should fail", func() {
		session := podmanTest.Podman([]string{"create", "--name", "foobar1", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"create", "--name", "foobar2", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"start", "-a", "foobar1", "foobar2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "you cannot start and attach multiple containers at once"))
	})

	It("podman failed to start with --rm should delete the container", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test1", "--rm", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		start := podmanTest.Podman([]string{"start", "test1"})
		start.WaitWithDefaultTimeout()

		wait := podmanTest.Podman([]string{"wait", "test1"})
		wait.WaitWithDefaultTimeout()
		Expect(wait).To(ExitWithError(125, `no container with name or ID "test1" found: no such container`))

		Eventually(podmanTest.NumberOfContainers, defaultWaitTimeout, 3.0).Should(BeZero())
	})

	It("podman failed to start without --rm should NOT delete the container", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		start := podmanTest.Podman([]string{"start", session.OutputToString()})
		start.WaitWithDefaultTimeout()
		Expect(start).To(ExitWithError(125, "not found in $PATH"))

		Eventually(podmanTest.NumberOfContainers, defaultWaitTimeout, 3.0).Should(Equal(1))
	})

	It("podman start --sig-proxy should not work without --attach", func() {
		session := podmanTest.Podman([]string{"create", "--name", "sigproxyneedsattach", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"start", "--sig-proxy", "sigproxyneedsattach"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "you cannot use sig-proxy without --attach: invalid argument"))
	})

	It("podman start container with special pidfile", func() {
		SkipIfRemote("pidfile not handled by remote")
		pidfile := filepath.Join(tempdir, "pidfile")
		session := podmanTest.Podman([]string{"create", "--pidfile", pidfile, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		readFirstLine := func(path string) string {
			content, err := os.ReadFile(path)
			Expect(err).ToNot(HaveOccurred())
			return strings.Split(string(content), "\n")[0]
		}
		containerPID := readFirstLine(pidfile)
		_, err = strconv.Atoi(containerPID) // Make sure it's a proper integer
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman start container --filter", func() {
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

		session1 = podmanTest.Podman([]string{"start", cid1, "-f", "status=running"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEmpty())

		session1 = podmanTest.Podman([]string{"start", "--all", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEmpty())

		session1 = podmanTest.Podman([]string{"start", "--all", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"start", "--all", "--filter", "label=test=with,comma"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid4))

		session1 = podmanTest.Podman([]string{"start", "-f", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))
	})

	It("podman start container does not set HOME to home of caller", func() {
		home, err := os.UserHomeDir()
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"create", "--userns", "keep-id", "--user", "bin:bin", "--volume", fmt.Sprintf("%s:%s:ro", home, home), ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", cid, "--format", "{{.Config.Env}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		env := session.OutputToString()
		Expect(env).To(ContainSubstring("HOME"))
		Expect(env).ToNot(ContainSubstring(fmt.Sprintf("HOME=%s", home)))

		session = podmanTest.Podman([]string{"restart", "-t", "-1", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", cid, "--format", "{{.Config.Env}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		env = session.OutputToString()
		Expect(env).To(ContainSubstring("HOME"))
		Expect(env).ToNot(ContainSubstring(fmt.Sprintf("HOME=%s", home)))
	})

	It("podman start container sets HOME to home of execUser", func() {
		session := podmanTest.Podman([]string{"create", "--userns", "keep-id", "--user", "bin:bin", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", cid, "--format", "{{.Config.Env}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		env := session.OutputToString()
		Expect(env).To(ContainSubstring("HOME=/bin"))

		session = podmanTest.Podman([]string{"restart", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", cid, "--format", "{{.Config.Env}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		env = session.OutputToString()
		Expect(env).To(ContainSubstring("HOME=/bin"))
	})

	It("podman start container retains the HOME env if present", func() {
		session := podmanTest.Podman([]string{"create", "--userns", "keep-id", "--user", "bin:bin", "--env=HOME=/env/is/respected", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", cid, "--format", "{{.Config.Env}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		env := session.OutputToString()
		Expect(env).To(ContainSubstring("HOME=/env/is/respected"))

		session = podmanTest.Podman([]string{"restart", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", cid, "--format", "{{.Config.Env}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		env = session.OutputToString()
		Expect(env).To(ContainSubstring("HOME=/env/is/respected"))
	})
})
