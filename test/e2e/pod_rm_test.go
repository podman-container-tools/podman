//go:build linux || freebsd

package integration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod rm", func() {
	It("podman pod rm empty pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "rm", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// Also check that we don't leak cgroups
		err := filepath.WalkDir("/sys/fs/cgroup", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// A cgroup directory could have been deleted in the meanwhile filepath.WalkDir was
				// accessing it.  If that happens, we just ignore the error.
				if d.IsDir() && errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return err
			}
			if strings.Contains(d.Name(), podid) {
				return fmt.Errorf("leaking cgroup path %s", path)
			}
			return nil
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman pod rm latest pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec2, podid2 := podmanTest.CreatePod(map[string][]string{"--name": {"pod2"}})
		Expect(ec2).To(Equal(0))

		latest := "--latest"
		if IsRemote() {
			latest = "pod2"
		}
		result := podmanTest.Podman([]string{"pod", "rm", latest})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(podid))
		Expect(result.OutputToString()).To(Not(ContainSubstring(podid2)))
	})

	It("podman pod rm removes a pod with a container", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.RunLsContainerInPod("", podid)
		Expect(ec2).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "rm", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"ps", "-qa"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToStringArray()).To(BeEmpty())
	})

	It("podman pod rm -f does remove a running container", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(BeEmpty())
	})

	It("podman pod rm -a doesn't remove a running container", func() {
		GinkgoWriter.Printf("To start, there are %d pods\n", podmanTest.NumberOfPods())
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))
		GinkgoWriter.Printf("Started %d pods\n", podmanTest.NumberOfPods())

		session := podmanTest.RunTopContainerInPod("", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		GinkgoWriter.Printf("Started container running in one pod")

		numPods := podmanTest.NumberOfPods()
		Expect(numPods).To(Equal(2))
		ps := podmanTest.Podman([]string{"pod", "ps"})
		ps.WaitWithDefaultTimeout()
		GinkgoWriter.Printf("Current %d pod(s):\n%s\n", numPods, ps.OutputToString())

		GinkgoWriter.Printf("Removing all empty pods\n")
		result := podmanTest.Podman([]string{"pod", "rm", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, "it is running - running or paused containers cannot be removed without force: container state improper"))
		Expect(result.ErrorToString()).To(ContainSubstring("not all containers could be removed from pod"))

		numPods = podmanTest.NumberOfPods()
		ps = podmanTest.Podman([]string{"pod", "ps"})
		ps.WaitWithDefaultTimeout()
		GinkgoWriter.Printf("Final %d pod(s):\n%s\n", numPods, ps.OutputToString())
		Expect(numPods).To(Equal(1))
		// Confirm top container still running inside remaining pod
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod rm -fa removes everything", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec, podid2 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", podid1, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.RunLsContainerInPod("", podid2)
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid2)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(BeEmpty())

		// one pod should have been deleted
		result = podmanTest.Podman([]string{"pod", "ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(BeEmpty())
	})

	It("podman rm bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "rm", "bogus"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID bogus found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "bogus": no such pod`
		}
		Expect(session).Should(ExitWithError(1, expect))
	})

	It("podman rm bogus pod and a running pod", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "rm", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID bogus found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "bogus": no such pod`
		}
		Expect(session).Should(ExitWithError(1, expect))

		session = podmanTest.Podman([]string{"pod", "rm", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		// FIXME-someday: consolidate different error messages
		expect = "no pod with name or ID test1 found"
		if IsRemote() {
			expect = `unable to find pod "test1"`
		}
		Expect(session).Should(ExitWithError(1, expect+": no such pod"))
	})

	It("podman rm --ignore bogus pod and a running pod", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "rm", "-t", "0", "--force", "--ignore", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "rm", "--ignore", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman pod start/remove single pod via --pod-id-file", func() {
		podIDFile := filepath.Join(tempdir, "podID")

		podName := "rudolph"

		// Create a pod with --pod-id-file.
		session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", podIDFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Create container inside the pod.
		session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", "--pod-id-file", podIDFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2)) // infra+top

		session = podmanTest.Podman([]string{"pod", "rm", "-t", "0", "--pod-id-file", podIDFile, "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod start/remove multiple pods via --pod-id-file", func() {
		podIDFiles := []string{}
		for _, i := range "0123456789" {
			cidFile := filepath.Join(tempdir, "cid"+string(i))
			podName := "rudolph" + string(i)
			// Create a pod with --pod-id-file.
			session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", cidFile})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// Create container inside the pod.
			session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// Append the id files along with the command.
			podIDFiles = append(podIDFiles, "--pod-id-file")
			podIDFiles = append(podIDFiles, cidFile)
		}

		cmd := []string{"pod", "start"}
		cmd = append(cmd, podIDFiles...)
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(20)) // 10*(infra+top)

		cmd = []string{"pod", "rm", "--time=0", "--force"}
		cmd = append(cmd, podIDFiles...)
		session = podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod rm with exited containers", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "--pod", podid, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podid, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "rm", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman pod rm pod with infra container and running container", func() {
		podName := "testPod"
		ctrName := "testCtr"

		ctrAndPod := podmanTest.Podman([]string{"run", "-d", "--pod", fmt.Sprintf("new:%s", podName), "--name", ctrName, ALPINE, "top"})
		ctrAndPod.WaitWithDefaultTimeout()
		Expect(ctrAndPod).Should(ExitCleanly())

		removePod := podmanTest.Podman([]string{"pod", "rm", "-a"})
		removePod.WaitWithDefaultTimeout()
		Expect(removePod).Should(Not(ExitCleanly()))

		ps := podmanTest.Podman([]string{"pod", "ps"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring(podName))

		removePodForce := podmanTest.Podman([]string{"pod", "rm", "-af"})
		removePodForce.WaitWithDefaultTimeout()
		Expect(removePodForce).Should(ExitCleanly())

		ps2 := podmanTest.Podman([]string{"pod", "ps"})
		ps2.WaitWithDefaultTimeout()
		Expect(ps2).Should(ExitCleanly())
		Expect(ps2.OutputToString()).To(Not(ContainSubstring(podName)))
	})
})
