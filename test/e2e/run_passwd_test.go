//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman run passwd", func() {
	It("podman run no user specified ", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("passwd")))
	})
	It("podman run user specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "bin", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("passwd")))
	})

	It("podman run UID specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "2:1", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("passwd")))
	})

	It("podman run UID not specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "20001:1", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("passwd"))
	})

	It("podman can run container without /etc/passwd", func() {
		dockerfile := fmt.Sprintf(`FROM %s
RUN rm -f /etc/passwd /etc/shadow /etc/group
USER 1000`, ALPINE)
		imgName := "testimg"
		podmanTest.BuildImage(dockerfile, imgName, "false")
		session := podmanTest.Podman([]string{"run", "--passwd=false", "--rm", imgName, "ls", "/etc/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("passwd")))

		// test that the /etc/passwd file is created
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "0:0", imgName, "ls", "/etc/passwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run with no user specified does not change --group specified", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("/etc/group")))
	})

	It("podman run group specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "root:bin", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("/etc/group")))
	})

	It("podman run non-numeric group not specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "root:doesnotexist", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(126, "unable to find group doesnotexist: no matching entries in group file"))
	})

	It("podman run numeric group specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "root:11", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("/etc/group")))
	})

	It("podman run numeric group not specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "20001:20001", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/etc/group"))
	})

	It("podman run numeric group from image and no group file", func() {
		dockerfile := fmt.Sprintf(`FROM %s
RUN rm -f /etc/passwd /etc/shadow /etc/group
USER 1000`, ALPINE)
		imgName := "testimg"
		podmanTest.BuildImage(dockerfile, imgName, "false")
		session := podmanTest.Podman([]string{"run", "--rm", imgName, "ls", "/etc/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("/etc/group")))
	})

	It("podman run --no-manage-passwd flag", func() {
		run := podmanTest.Podman([]string{"run", "--user", "1234:1234", ALPINE, "cat", "/etc/passwd"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring("1234:1234"))

		run = podmanTest.Podman([]string{"run", "--passwd=false", "--user", "1234:1234", ALPINE, "cat", "/etc/passwd"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).NotTo(ContainSubstring("1234:1234"))
	})

	It("podman run --passwd-entry flag", func() {
		// Test that the line we add doesn't contain anything else than what is specified
		run := podmanTest.Podman([]string{"run", "--user", "1234:1234", "--passwd-entry=FOO", ALPINE, "grep", "^FOO$", "/etc/passwd"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		run = podmanTest.Podman([]string{"run", "--user", "12345:12346", "-w", "/etc", "--passwd-entry=$UID-$GID-$NAME-$HOME-$USERNAME", ALPINE, "cat", "/etc/passwd"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring("12345-12346-container user-/etc-12345"))
	})

	It("podman run --group-entry flag", func() {
		// Test that the line we add doesn't contain anything else than what is specified
		run := podmanTest.Podman([]string{"run", "--user", "1234:1234", "--group-entry=FOO", ALPINE, "grep", "^FOO$", "/etc/group"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		run = podmanTest.Podman([]string{"run", "--user", "12345:12346", "--group-entry=$GID", ALPINE, "tail", "/etc/group"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring("12346"))
	})
})
