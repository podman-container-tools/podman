//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/selinux/go-selinux"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman run", func() {
	BeforeEach(func() {
		if !selinux.GetEnabled() {
			Skip("SELinux not enabled")
		}
	})

	It("podman run selinux", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_t"))
	})

	It("podman run selinux grep test", func() {
		session := podmanTest.Podman([]string{"run", "--security-opt", "label=level:s0:c1,c2", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("s0:c1,c2"))
	})

	It("podman run selinux disable test", func() {
		session := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman run selinux type check test", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Or(ContainSubstring("container_t"), ContainSubstring("svirt_lxc_net_t")))
	})

	It("podman run selinux type setup test", func() {
		session := podmanTest.Podman([]string{"run", "--security-opt", "label=type:spc_t", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman privileged selinux", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman test selinux label resolv.conf", func() {
		session := podmanTest.Podman([]string{"run", FEDORA_MINIMAL, "ls", "-Z", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux label hosts", func() {
		session := podmanTest.Podman([]string{"run", FEDORA_MINIMAL, "ls", "-Z", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux label hostname", func() {
		session := podmanTest.Podman([]string{"run", FEDORA_MINIMAL, "ls", "-Z", "/etc/hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux label /run/secrets", func() {
		session := podmanTest.Podman([]string{"run", FEDORA_MINIMAL, "ls", "-dZ", "/run/secrets"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux --privileged label resolv.conf", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", FEDORA_MINIMAL, "ls", "-Z", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux --privileged label hosts", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", FEDORA_MINIMAL, "ls", "-Z", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux --privileged label hostname", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", FEDORA_MINIMAL, "ls", "-Z", "/etc/hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux --privileged label /run/secrets", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", FEDORA_MINIMAL, "ls", "-dZ", "/run/secrets"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman run selinux file type setup test", func() {
		session := podmanTest.Podman([]string{"run", "--security-opt", "label=type:spc_t", "--security-opt", "label=filetype:container_var_lib_t", FEDORA_MINIMAL, "ls", "-Z", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_var_lib_t"))

		session = podmanTest.Podman([]string{"run", "--security-opt", "label=type:spc_t", "--security-opt", "label=filetype:foobar", FEDORA_MINIMAL, "ls", "-Z", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(126, "invalid argument"))
	})

	It("podman exec selinux check", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/1/attr/current"})
		session.WaitWithDefaultTimeout()
		session1 := podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/self/attr/current"})
		session1.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Equal(session1.OutputToString()))
	})

	It("podman run --privileged and --security-opt SELinux options", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--security-opt", "label=type:spc_t", "--security-opt", "label=level:s0:c1,c2", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
		Expect(session.OutputToString()).To(ContainSubstring("s0:c1,c2"))
	})

	It("podman pod container share SELinux labels", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(label1))

		session = podmanTest.Podman([]string{"pod", "rm", "-t", "0", podID, "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman pod container --infra=false doesn't share SELinux labels", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--infra=false"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(Equal(label1)))

		session = podmanTest.Podman([]string{"pod", "rm", "-t", "0", podID, "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman shared IPC NS container share SELinux labels", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--ipc", "container:test1", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(label1))
	})

	It("podman shared PID NS container share SELinux labels", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pid", "container:test1", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(label1))
	})

	It("podman shared NET NS container doesn't share SELinux labels", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--net", "container:test1", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(Equal(label1)))
	})

	It("podman test --pid=host", func() {
		session := podmanTest.Podman([]string{"run", "--pid=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman test --ipc=host", func() {
		session := podmanTest.Podman([]string{"run", "--ipc=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman test --ipc=net", func() {
		session := podmanTest.Podman([]string{"run", "--net=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_t"))
	})

	It("podman test --ipc=net", func() {
		session := podmanTest.Podman([]string{"run", "--net=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_t"))
	})

	It("podman test --ipc=net", func() {
		session := podmanTest.Podman([]string{"run", "--net=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("container_t"))
	})

	It("podman test --runtime=/PATHTO/kata-runtime", func() {
		runtime := podmanTest.OCIRuntime
		podmanTest.OCIRuntime = filepath.Join(podmanTest.TempDir, "kata-runtime")
		err := os.Symlink("/bin/true", podmanTest.OCIRuntime)
		Expect(err).ToNot(HaveOccurred())
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		}
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"inspect", "--format", "{{ .ProcessLabel }}", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("container_kvm_t"))

		podmanTest.OCIRuntime = runtime
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		}
	})

	It("podman test init labels", func() {
		session := podmanTest.Podman([]string{"create", SYSTEMD_IMAGE, "/sbin/init"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"inspect", "--format", "{{ .ProcessLabel }}", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("container_init_t"))
	})

	It("podman relabels named volume with :Z", func() {
		session := podmanTest.Podman([]string{"run", "-v", "testvol:/test1/test:Z", FEDORA_MINIMAL, "ls", "-alZ", "/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(":s0:"))
	})
})
