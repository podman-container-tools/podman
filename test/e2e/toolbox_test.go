//go:build linux || freebsd

package integration

/*
	toolbox_test.go is under the care of the Toolbox Team.

	The tests are trying to stress parts of Podman that Toolbox[0] needs for
	its functionality.

	[0] https://github.com/containers/toolbox

	Info about test cases:
	- some tests rely on a certain configuration of a container that is done by
		executing several commands in the entry-point of a container. To make
		sure the initialization had enough time to be executed,
		WaitContainerReady() after the container is started.

	- in several places there's an invocation of 'podman logs' It is there mainly
		to ease debugging when a test goes wrong (during the initialization of a
		container) but sometimes it is also used in the test case itself.

	Maintainers (Toolbox Team):
	- Ondřej Míchal <harrymichal@fedoraproject.org>
	- Debarshi Ray <rishi@fedoraproject.org>

	Also available on Freenode IRC on #silverblue or #podman
*/

import (
	"fmt"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/libpod/define"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Toolbox-specific testing", func() {
	It("podman run --dns=none - allows self-management of /etc/resolv.conf", func() {
		session := podmanTest.Podman([]string{
			"run", "--dns", "none", ALPINE, "sh", "-c",
			"rm -f /etc/resolv.conf; touch -d '1970-01-01 00:02:03' /etc/resolv.conf; stat -c %s:%Y /etc/resolv.conf",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0:123"))
	})

	It("podman run --no-hosts - allows self-management of /etc/hosts", func() {
		session := podmanTest.Podman([]string{
			"run", "--no-hosts", ALPINE, "sh", "-c",
			"rm -f /etc/hosts; touch -d '1970-01-01 00:02:03' /etc/hosts; stat -c %s:%Y /etc/hosts",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0:123"))
	})

	It("podman create --ulimit host + podman exec - correctly mirrors hosts ulimits", func() {
		if podmanTest.RemoteTest {
			Skip("Ulimit check does not work with a remote client")
		}
		info := GetHostDistributionInfo()
		if info.Distribution == "debian" {
			// "expected 1048576 to be >= 1073741816"
			Skip("FIXME 2024-05-28 fails on debian, maybe because of systemd 256?")
		}
		var session *PodmanSessionIntegration
		var containerHardLimit int
		var rlimit syscall.Rlimit
		var err error

		err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit)
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Expected value: %d", rlimit.Max)

		session = podmanTest.Podman([]string{
			"create", "--name", "test", "--ulimit", "host", ALPINE,
			"sleep", "1000",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{
			"exec", "test", "sh", "-c",
			"ulimit -H -n",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		containerHardLimit, err = strconv.Atoi(strings.Trim(session.OutputToString(), "\n"))
		Expect(err).ToNot(HaveOccurred())
		Expect(containerHardLimit).To(BeNumerically(">=", rlimit.Max))
	})

	It("podman create --ipc=host --pid=host + podman exec - correct shared memory limit size", func() {
		// Comparison of the size of /dev/shm on the host being equal to the one in
		// a container
		if podmanTest.RemoteTest {
			Skip("Shm size check does not work with a remote client")
		}
		var session *PodmanSessionIntegration
		var cmd *exec.Cmd
		var hostShmSize, containerShmSize int
		var err error

		// Because Alpine uses busybox, most commands don't offer advanced options
		// like "--output" in df. Therefore the value of the field 'Size' (or
		// ('1K-blocks') needs to be extracted manually.
		cmd = exec.Command("df", "/dev/shm")
		res, err := cmd.Output()
		Expect(err).ToNot(HaveOccurred())
		lines := strings.SplitN(string(res), "\n", 2)
		fields := strings.Fields(lines[len(lines)-1])
		hostShmSize, err = strconv.Atoi(fields[1])
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{
			"create", "--name", "test", "--ipc=host", "--pid=host", ALPINE,
			"sleep", "1000",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{
			"exec", "test",
			"df", "/dev/shm",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		lines = session.OutputToStringArray()
		fields = strings.Fields(lines[len(lines)-1])
		containerShmSize, err = strconv.Atoi(fields[1])
		Expect(err).ToNot(HaveOccurred())

		// In some cases it may happen that the size of /dev/shm is not exactly
		// equal. Therefore it's fine if there's a slight tolerance between the
		// compared values.
		Expect(hostShmSize).To(BeNumerically("~", containerShmSize, 100))
	})

	It("podman create --userns=keep-id --user root:root - entrypoint - entrypoint is executed as root", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		session := podmanTest.Podman([]string{
			"run", "--userns=keep-id", "--user", "root:root", ALPINE,
			"id",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("uid=0(root) gid=0(root)"))
	})

	It("podman create --userns=keep-id + podman exec - correct names of user and group", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration
		var err error

		currentUser, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		currentGroup, err := user.LookupGroupId(currentUser.Gid)
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{
			"create", "--name", "test", "--userns=keep-id", ALPINE,
			"sleep", "1000",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		expectedOutput := fmt.Sprintf("uid=%s(%s) gid=%s(%s)",
			currentUser.Uid, currentUser.Username,
			currentGroup.Gid, currentGroup.Name)

		session = podmanTest.Podman([]string{
			"exec", "test",
			"id",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(expectedOutput))
	})

	It("podman run --userns=keep-id - modify /etc/passwd and /etc/group", func() {
		passwdLine := "testuser:x:1001:1001::/home/testuser:/bin/sh"
		groupLine := "testuser:x:1001:"

		// ensure that the container can edit passwd and group files
		session := podmanTest.Podman([]string{
			"run", "--log-driver", "k8s-file", "--name", "test", "--userns=keep-id",
			"--user", "root:root", ALPINE, "sh", "-c",
			fmt.Sprintf("echo %s > /etc/passwd && echo %s > /etc/group && cat /etc/passwd && cat /etc/group", passwdLine, groupLine),
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring(passwdLine))
		Expect(session.OutputToString()).Should(ContainSubstring(groupLine))
	})

	It("podman run --privileged --userns=keep-id --user root:root - entrypoint - (bind)mounting", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration

		session = podmanTest.Podman([]string{
			"run", "--privileged", "--userns=keep-id", "--user", "root:root", ALPINE,
			"mount", "-t", define.TypeTmpfs, define.TypeTmpfs, "/tmp",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{
			"run", "--privileged", "--userns=keep-id", "--user", "root:root", ALPINE,
			"mount", "--rbind", "/tmp", "/var/tmp",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman create + start - with all needed switches for create", func() {
		SkipIfNotRootless("only meaningful when run rootless")

		// These should be most of the switches that Toolbox uses to create a "toolbox" container
		// https://github.com/containers/toolbox/blob/main/src/cmd/create.go
		session := podmanTest.Podman([]string{
			"create",
			"--log-driver", "k8s-file",
			"--dns", "none",
			"--hostname", "toolbox",
			"--ipc", "host",
			"--label", "com.github.containers.toolbox=true",
			"--name", "test",
			"--network", "host",
			"--no-hosts",
			"--pid", "host",
			"--privileged",
			"--security-opt", "label=disable",
			"--ulimit", "host",
			"--userns=keep-id",
			"--user", "root:root",
			ALPINE, "sh", "-c", "echo READY",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"start", "-a", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("READY"))
	})

	It("podman run --userns=keep-id check $HOME", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration
		currentUser, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:%s", currentUser.HomeDir, currentUser.HomeDir), "--userns=keep-id", ALPINE, "sh", "-c", "echo $HOME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(currentUser.HomeDir))

		location := path.Dir(currentUser.HomeDir)
		volumeArg := fmt.Sprintf("%s:%s", location, location)
		session = podmanTest.Podman([]string{
			"run",
			"--userns=keep-id",
			"--volume", volumeArg,
			ALPINE, "sh", "-c", "echo $HOME",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(currentUser.HomeDir))
	})
})
