//go:build linux || freebsd

package integration

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman run cpu", func() {
	BeforeEach(func() {
		if err := os.WriteFile("/sys/fs/cgroup/cgroup.subtree_control", []byte("+cpuset"), 0o644); err != nil {
			Skip("cpuset controller not available on the current kernel")
		}
	})

	It("podman run cpu-period", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-period=5000", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpu.max"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("5000"))
	})

	It("podman run cpu-quota", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-quota=5000", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpu.max"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("5000"))
	})

	It("podman run cpus", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-quota=5000", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpu.max"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("5000 100000"))
	})

	It("podman run cpu-shares", func() {
		// [2-262144] is mapped to [1-10000]
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-shares=262144", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpu.weight"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("10000"))
	})

	It("podman run cpuset-cpus", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpuset-cpus=0", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpuset.cpus.effective"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("0"))
	})

	It("podman run cpuset-mems", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpuset-mems=0", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpuset.mems.effective"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("0"))
	})

	It("podman run cpus and cpu-period", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-period=5000", "--cpus=0.5", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, "--cpu-period and --cpus cannot be set together"))
	})

	It("podman run cpus and cpu-quota", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-quota=5000", "--cpus=0.5", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, "--cpu-quota and --cpus cannot be set together"))
	})

	It("podman run invalid cpu-rt-period with cgroupsv2", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-rt-period=5000", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.ErrorToString()).To(ContainSubstring("Realtime period not supported on cgroups V2 systems"))
	})

	It("podman run invalid cpu-rt-runtime with cgroupsv2", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-rt-runtime=5000", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.ErrorToString()).To(ContainSubstring("Realtime runtime not supported on cgroups V2 systems"))
	})
})
