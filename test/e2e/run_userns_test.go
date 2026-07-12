//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage"
)

func createContainersConfFileWithCustomUserns(pTest *PodmanTestIntegration, userns string) {
	configPath := filepath.Join(pTest.TempDir, "containers.conf")
	containersConf := fmt.Appendf(nil, "[containers]\nuserns = \"%s\"\n", userns)
	err := os.WriteFile(configPath, containersConf, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	// Set custom containers.conf file
	os.Setenv("CONTAINERS_CONF", configPath)
	if IsRemote() {
		pTest.RestartRemoteService()
	}
}

var _ = Describe("Podman UserNS support", func() {
	BeforeEach(func() {
		if os.Getenv("SKIP_USERNS") != "" {
			Skip("Skip userns tests.")
		}
		if _, err := os.Stat("/proc/self/uid_map"); err != nil {
			Skip("User namespaces not supported.")
		}
	})

	// Note: Lot of tests for build with --userns=auto are already there in buildah
	// but they are skipped in podman CI because bud tests are executed in rootfull
	// environment ( where mappings for the `containers` user is not present in /etc/subuid )
	// causing them to skip hence this is a redundant test for sanity to make sure
	// we don't break this feature for podman-remote.
	It("podman build with --userns=auto", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		name := u.Username
		if name == "root" {
			name = "containers"
		}
		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}
		session := podmanTest.Podman([]string{"build", "-f", "build/Containerfile.userns-auto", "-t", "test", "--userns=auto"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// `1024` is the default size or length of the range of user IDs
		// that is mapped between the two user namespaces by --userns=auto.
		Expect(session.OutputToString()).To(ContainSubstring(strconv.Itoa(storage.AutoUserNsMinSize)))
	})

	It("podman uidmapping and gidmapping", func() {
		session := podmanTest.Podman([]string{"run", "--uidmap=0:100:5000", "--gidmap=0:200:5000", "alpine", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
	})

	// It essentially repeats the test above but with the `-it` short option
	// that broke execution at:
	//     https://github.com/containers/podman/pull/1066#issuecomment-403562116
	// To avoid a potential future regression, use this as a test.
	It("podman uidmapping and gidmapping with short-opts", func() {
		session := podmanTest.Podman([]string{"run", "--uidmap=0:1:5000", "--gidmap=0:200:5000", "alpine", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman uidmapping and gidmapping with a volume", func() {
		session := podmanTest.Podman([]string{"run", "--uidmap=0:1:500", "--gidmap=0:200:5000", "-v", "my-foo-volume:/foo:Z", "alpine", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman uidmapping and gidmapping with an idmapped volume", func() {
		SkipIfRunc(podmanTest, "Test not supported yet with runc (issue 17433, wontfix)")
		session := podmanTest.Podman([]string{"run", "--uidmap=0:1:500", "--gidmap=0:200:5000", "-v", "my-foo-volume:/foo:Z,idmap", "alpine", "stat", "-c", "#%u:%g#", "/foo"})
		session.WaitWithDefaultTimeout()
		if strings.Contains(session.ErrorToString(), "Operation not permitted") {
			Skip("not sufficiently privileged")
		}
		if strings.Contains(session.ErrorToString(), "Invalid argument") {
			Skip("the file system doesn't support idmapped mounts")
		}
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("#0:0#"))
	})

	It("podman uidmapping and gidmapping with an idmapped volume on existing directory", func() {
		SkipIfRunc(podmanTest, "Test not supported yet with runc (issue 17433, wontfix)")
		// The directory /mnt already exists in the image
		session := podmanTest.Podman([]string{"run", "--uidmap=0:1:500", "--gidmap=0:200:5000", "-v", "my-foo-volume:/mnt:Z,idmap", "alpine", "stat", "-c", "#%u:%g#", "/mnt"})
		session.WaitWithDefaultTimeout()
		if strings.Contains(session.ErrorToString(), "Operation not permitted") {
			Skip("not sufficiently privileged")
		}
		if strings.Contains(session.ErrorToString(), "Invalid argument") {
			Skip("the file system doesn't support idmapped mounts")
		}
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("#0:0#"))
	})

	It("podman uidmapping and gidmapping --net=host", func() {
		session := podmanTest.Podman([]string{"run", "--net=host", "--uidmap=0:1:5000", "--gidmap=0:200:5000", "alpine", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman --userns=keep-id", func() {
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "alpine", "id", "-u"})
		session.WaitWithDefaultTimeout()

		Expect(session).Should(ExitCleanly())
		uid := strconv.Itoa(os.Geteuid())
		Expect(session.OutputToString()).To(ContainSubstring(uid))

		session = podmanTest.Podman([]string{"run", "--userns=keep-id:uid=10,gid=12", "alpine", "sh", "-c", "echo $(id -u):$(id -g)"})
		session.WaitWithDefaultTimeout()

		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("10:12"))
	})

	It("podman --userns=keep-id check passwd", func() {
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "alpine", "id", "-un"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		Expect(session.OutputToString()).To(Equal(u.Username))
	})

	It("podman --userns=keep-id root owns /usr", func() {
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "alpine", "stat", "-c%u", "/usr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("0"))
	})

	It("podman --userns=keep-id:size", func() {
		session := podmanTest.Podman([]string{"run", "--userns=keep-id:size=10", ALPINE, "sh", "-c", "(awk 'BEGIN{SUM=0} {SUM += $3} END{print SUM}' < /proc/self/uid_map)"})
		session.WaitWithDefaultTimeout()

		if isRootless() {
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("10"))
		} else {
			Expect(session).Should(ExitWithError(125, "cannot set max size for user namespace when not running rootless"))
		}
	})

	It("podman --userns=keep-id --user root:root", func() {
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "--user", "root:root", "alpine", "id", "-u"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("0"))
	})

	It("podman run --userns=keep-id can add users", func() {
		userName := os.Getenv("USER")
		if userName == "" {
			Skip("Can't complete test if no username available")
		}

		ctrName := "ctr-name"
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "--user", "root:root", "-d", "--stop-signal", "9", "--name", ctrName, CITEST_IMAGE, "sleep", "600"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		exec1 := podmanTest.Podman([]string{"exec", ctrName, "cat", "/etc/passwd"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())
		Expect(exec1.OutputToString()).To(ContainSubstring(userName))

		exec2 := podmanTest.Podman([]string{"exec", ctrName, "adduser", "-D", "testuser"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())
	})

	It("podman --userns=auto", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		name := u.Username
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		m := make(map[string]string)
		for range 5 {
			session := podmanTest.Podman([]string{"run", "--userns=auto", "alpine", "cat", "/proc/self/uid_map"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			l := session.OutputToString()
			// `1024` is the default size or length of the range of user IDs
			// that is mapped between the two user namespaces by --userns=auto.
			Expect(l).To(ContainSubstring("1024"))
			m[l] = l
		}
		// check for no duplicates
		Expect(m).To(HaveLen(5))

		createContainersConfFileWithCustomUserns(podmanTest, "auto:size=1019")
		session := podmanTest.Podman([]string{"run", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("1019"))
	})

	It("podman --userns=auto:size=%d", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		name := u.Username
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		session := podmanTest.Podman([]string{"run", "--userns=auto:size=500", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("500"))

		session = podmanTest.Podman([]string{"run", "--userns=auto:size=3000", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("3000"))

		session = podmanTest.Podman([]string{"run", "--userns=auto", "--user=2000:3000", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("3001"))

		session = podmanTest.Podman([]string{"run", "--userns=auto", "--user=4000:1000", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("4001"))
	})

	It("podman --userns=auto:uidmapping=", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		name := u.Username
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		session := podmanTest.Podman([]string{"run", "--userns=auto:uidmapping=0:0:1", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(MatchRegexp(`(^|\s)0\s+0\s+1(\s|$)`))

		session = podmanTest.Podman([]string{"run", "--userns=auto:size=8192,uidmapping=0:0:1", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("8191"))
	})

	It("podman --userns=auto:gidmapping=", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		name := u.Username
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		session := podmanTest.Podman([]string{"run", "--userns=auto:gidmapping=0:0:1", "alpine", "cat", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(MatchRegexp(`(^|\s)0\s+0\s+1(\s|$)`))

		session = podmanTest.Podman([]string{"run", "--userns=auto:size=8192,gidmapping=0:0:1", "alpine", "cat", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("8191"))
	})

	It("podman --userns=container:CTR", func() {
		ctrName := "userns-ctr"
		session := podmanTest.Podman([]string{"run", "-d", "--uidmap=0:0:1", "--uidmap=1:2:4998", "--name", ctrName, "alpine", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// runc has an issue and we also need to join the IPC namespace.
		session = podmanTest.Podman([]string{"run", "--rm", "--userns=container:" + ctrName, "--ipc=container:" + ctrName, "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(session.OutputToString()).To(ContainSubstring("4998"))

		session = podmanTest.Podman([]string{"run", "--rm", "--userns=container:" + ctrName, "--net=container:" + ctrName, "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(session.OutputToString()).To(ContainSubstring("4998"))
	})

	It("podman --user with volume", func() {
		tests := []struct {
			uid, gid, arg, vol string
		}{
			{"0", "0", "0:0", "vol-0"},
			{"1000", "0", "1000", "vol-1"},
			{"1000", "1000", "1000:1000", "vol-2"},
		}

		for _, tt := range tests {
			session := podmanTest.Podman([]string{"run", "-d", "--user", tt.arg, "--mount", "type=volume,src=" + tt.vol + ",dst=/home/user", "alpine", "top"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			inspectUID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .UID }}", tt.vol})
			inspectUID.WaitWithDefaultTimeout()
			Expect(inspectUID).Should(ExitCleanly())
			Expect(inspectUID.OutputToString()).To(Equal(tt.uid))

			// Make sure we're defaulting to 0.
			inspectGID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .GID }}", tt.vol})
			inspectGID.WaitWithDefaultTimeout()
			Expect(inspectGID).Should(ExitCleanly())
			Expect(inspectGID.OutputToString()).To(Equal(tt.gid))
		}
	})

	It("podman --userns= conflicts with ui[dg]map and sub[ug]idname", func() {
		session := podmanTest.Podman([]string{"run", "--userns=host", "--uidmap=0:1:500", "alpine", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "--userns and --uidmap/--gidmap/--subuidname/--subgidname are mutually exclusive"))

		session = podmanTest.Podman([]string{"run", "--userns=host", "--gidmap=0:200:5000", "alpine", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "--userns and --uidmap/--gidmap/--subuidname/--subgidname are mutually exclusive"))

		// with sub[ug]idname we don't check for the error output since the error message could be different, depending on the
		// system configuration since the specified user could not be defined and cause a different earlier error.
		// In any case, make sure the command doesn't succeed.
		session = podmanTest.Podman([]string{"run", "--userns=private", "--subuidname=containers", "alpine", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Not(ExitCleanly()))

		session = podmanTest.Podman([]string{"run", "--userns=private", "--subgidname=containers", "alpine", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Not(ExitCleanly()))
	})

	It("podman PODMAN_USERNS", func() {
		podmanUserns, podmanUserusSet := os.LookupEnv("PODMAN_USERNS")
		os.Setenv("PODMAN_USERNS", "keep-id")
		defer func() {
			if podmanUserusSet {
				os.Setenv("PODMAN_USERNS", podmanUserns)
			} else {
				os.Unsetenv("PODMAN_USERNS")
			}
		}()
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		result := podmanTest.Podman([]string{"create", ALPINE, "true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.IDMappings }}", result.OutputToString()})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Not(Equal("<nil>")))

		// --pod should work.
		result = podmanTest.Podman([]string{"create", "--pod=new:new-pod", ALPINE, "true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
	})

	It("podman pod userns inherited for containers", func() {
		podName := "testPod"
		podIDFile := filepath.Join(podmanTest.TempDir, "podid")
		podCreate := podmanTest.Podman([]string{"pod", "create", "--pod-id-file", podIDFile, "--uidmap", "0:0:1000", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		// The containers should not use PODMAN_USERNS as they must inherited the userns from the pod.
		os.Setenv("PODMAN_USERNS", "keep-id")
		defer os.Unsetenv("PODMAN_USERNS")

		expectedMapping := `         0          0       1000
         0          0       1000
`
		// rootless mapping is split in two ranges
		if isRootless() {
			expectedMapping = `         0          0          1
         1          1        999
         0          0          1
         1          1        999
`
		}

		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := string(session.Out.Contents())
		Expect(output).To(Equal(expectedMapping))

		// https://github.com/containers/podman/issues/22931
		session = podmanTest.Podman([]string{"run", "--pod-id-file", podIDFile, ALPINE, "cat", "/proc/self/uid_map", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = string(session.Out.Contents())
		Expect(output).To(Equal(expectedMapping))
	})
})
