//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

const cgroupRoot = "/sys/fs/cgroup"

var _ = Describe("Podman run with --cgroup-parent", func() {
	Specify("valid --cgroup-parent using cgroupfs", func() {
		if !Containerized() {
			Skip("Must be containerized to run this test.")
		}
		cgroup := "/zzz"
		run := podmanTest.Podman([]string{"run", "--cgroupns=host", "--cgroup-parent", cgroup, FEDORA_MINIMAL, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(cgroup))
	})

	Specify("no --cgroup-parent", func() {
		cgroup := "/libpod_parent"
		if !Containerized() && podmanTest.CgroupManager != "cgroupfs" {
			if isRootless() {
				cgroup = "/containers.slice"
			} else {
				cgroup = "/machine.slice"
			}
		}
		run := podmanTest.Podman([]string{"run", "--cgroupns=host", FEDORA_MINIMAL, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(cgroup))
	})

	Specify("always honor --cgroup-parent", func() {
		if Containerized() || podmanTest.CgroupManager == "cgroupfs" {
			Skip("Requires Systemd cgroup manager support")
		}
		if IsRemote() {
			Skip("Not supported for remote")
		}

		run := podmanTest.Podman([]string{"run", "-d", "--cgroupns=host", FEDORA_MINIMAL, "sleep", "100"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		cid := run.OutputToString()

		exec := podmanTest.Podman([]string{"exec", cid, "cat", "/proc/1/cgroup"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())

		containerCgroup := strings.TrimRight(strings.ReplaceAll(exec.OutputToString(), "0::", ""), "\n")

		// Move the container process to a sub cgroup
		content, err := os.ReadFile(filepath.Join(cgroupRoot, containerCgroup, "cgroup.procs"))
		Expect(err).ToNot(HaveOccurred())
		oldSubCgroupPath := filepath.Join(cgroupRoot, containerCgroup, "old-container")
		err = os.MkdirAll(oldSubCgroupPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(filepath.Join(oldSubCgroupPath, "cgroup.procs"), content, 0o644)
		Expect(err).ToNot(HaveOccurred())

		newCgroup := fmt.Sprintf("%s/new-container", containerCgroup)
		err = os.MkdirAll(filepath.Join(cgroupRoot, newCgroup), 0o755)
		Expect(err).ToNot(HaveOccurred())

		run = podmanTest.Podman([]string{"--cgroup-manager=cgroupfs", "run", "--rm", "--cgroupns=host", fmt.Sprintf("--cgroup-parent=%s", newCgroup), FEDORA_MINIMAL, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		cgroupEffective := strings.TrimRight(strings.ReplaceAll(run.OutputToString(), "0::", ""), "\n")

		Expect(newCgroup).To(Equal(filepath.Dir(cgroupEffective)))
	})

	Specify("valid --cgroup-parent using slice", func() {
		if Containerized() || podmanTest.CgroupManager == "cgroupfs" {
			Skip("Requires Systemd cgroup manager support")
		}
		cgroup := "aaaa.slice"
		run := podmanTest.Podman([]string{"run", "--cgroupns=host", "--cgroup-parent", cgroup, FEDORA_MINIMAL, "cat", "/proc/1/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(cgroup))
	})
})
