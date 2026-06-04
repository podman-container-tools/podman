//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/checkpoint-restore/go-criu/v8/stats"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/checkpoint/crutils"
	"go.podman.io/podman/v6/pkg/criu"
	"go.podman.io/podman/v6/pkg/domain/entities"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/podman/v6/utils"
)

var netname string

func getRunString(input []string) []string {
	runString := []string{"run", "-d", "--network", netname}
	return append(runString, input...)
}

var _ = Describe("Podman checkpoint", func() {
	BeforeEach(func() {
		SkipIfRootless("checkpoint not supported in rootless mode")

		// Check if the runtime implements checkpointing. Currently only
		// runc's checkpoint/restore implementation is supported.
		cmd := exec.Command(podmanTest.OCIRuntime, "checkpoint", "--help")
		if err := cmd.Start(); err != nil {
			Skip("OCI runtime does not support checkpoint/restore")
		}
		if err := cmd.Wait(); err != nil {
			Skip("OCI runtime does not support checkpoint/restore")
		}

		if err := criu.CheckForCriu(criu.MinCriuVersion); err != nil {
			Skip(fmt.Sprintf("check CRIU version error: %v", err))
		}

		session := podmanTest.PodmanExitCleanly("network", "create")
		netname = session.OutputToString()
	})

	AfterEach(func() {
		if netname != "" {
			podmanTest.PodmanExitCleanly("network", "rm", "-f", netname)
		}
	})

	It("podman checkpoint bogus container", func() {
		session := podmanTest.Podman([]string{"container", "checkpoint", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "no such container"))
	})

	It("podman restore bogus container", func() {
		session := podmanTest.Podman([]string{"container", "restore", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "no such container or image"))
	})

	It("podman checkpoint a running container by id", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.PodmanExitCleanly(localRunString...)
		cid := session.OutputToString()

		// Check if none of the checkpoint/restore specific information is displayed
		// for newly started containers.
		inspect := podmanTest.PodmanExitCleanly("inspect", cid)
		inspectOut := inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Checkpointed).To(BeFalse(), ".State.Checkpointed")
		Expect(inspectOut[0].State.Restored).To(BeFalse(), ".State.Restored")
		Expect(inspectOut[0].State).To(HaveField("CheckpointPath", ""))
		Expect(inspectOut[0].State).To(HaveField("CheckpointLog", ""))
		Expect(inspectOut[0].State).To(HaveField("RestoreLog", ""))

		result := podmanTest.PodmanExitCleanly(
			"container",
			"checkpoint",
			"--keep",
			cid,
		)
		Expect(result.OutputToString()).To(Equal(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// For a checkpointed container we expect the checkpoint related information
		// to be populated.
		inspect = podmanTest.PodmanExitCleanly("inspect", cid)
		inspectOut = inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Checkpointed).To(BeTrue(), ".State.Checkpointed")
		Expect(inspectOut[0].State.Restored).To(BeFalse(), ".State.Restored")
		Expect(inspectOut[0].State.CheckpointPath).To(ContainSubstring("userdata/checkpoint"))
		Expect(inspectOut[0].State.CheckpointLog).To(ContainSubstring("userdata/dump.log"))
		Expect(inspectOut[0].State).To(HaveField("RestoreLog", ""))

		// Restoring with --publish should fail without --import (#28031)
		result = podmanTest.Podman([]string{"container", "restore", "-p", "8080:8080", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "--publish can only be used with image or --import"))

		result = podmanTest.PodmanExitCleanly(
			"container",
			"restore",
			"--keep",
			cid,
		)
		Expect(result.OutputToString()).To(Equal(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		inspect = podmanTest.PodmanExitCleanly("inspect", cid)
		inspectOut = inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Restored).To(BeTrue(), ".State.Restored")
		Expect(inspectOut[0].State.Checkpointed).To(BeFalse(), ".State.Checkpointed")
		Expect(inspectOut[0].State.CheckpointPath).To(ContainSubstring("userdata/checkpoint"))
		Expect(inspectOut[0].State.CheckpointLog).To(ContainSubstring("userdata/dump.log"))
		Expect(inspectOut[0].State.RestoreLog).To(ContainSubstring("userdata/restore.log"))

		podmanTest.StopContainer(cid)
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		podmanTest.PodmanExitCleanly(
			"container",
			"start",
			cid,
		)

		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		// Stopping and starting the container should remove all checkpoint
		// related information from inspect again.
		inspect = podmanTest.PodmanExitCleanly("inspect", cid)
		inspectOut = inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Checkpointed).To(BeFalse(), ".State.Checkpointed")
		Expect(inspectOut[0].State.Restored).To(BeFalse(), ".State.Restored")
		Expect(inspectOut[0].State).To(HaveField("CheckpointPath", ""))
		Expect(inspectOut[0].State).To(HaveField("CheckpointLog", ""))
		Expect(inspectOut[0].State).To(HaveField("RestoreLog", ""))
	})

	It("podman checkpoint a running container by name", func() {
		localRunString := getRunString([]string{"--name", "test_name", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"container", "checkpoint", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("test_name"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("test_name"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Restore a container which name is equal to an image name (#15055)
		localRunString = getRunString([]string{"--name", "alpine", "quay.io/libpod/alpine:latest", "top"})
		session = podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"container", "checkpoint", "alpine"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "alpine"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
	})

	It("podman pause a checkpointed container by id", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitWithError(125, `"exited" is not running, can't pause: container state improper`))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(2, " as it is running - running or paused containers cannot be removed without force: container state improper"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", "-t", "1", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint latest running container", func() {
		localRunString := getRunString([]string{"--name", "first", ALPINE, "top"})
		session1 := podmanTest.Podman(localRunString)
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		localRunString = getRunString([]string{"--name", "second", ALPINE, "top"})
		session2 := podmanTest.Podman(localRunString)
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"container", "checkpoint", "second"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("second"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		ps := podmanTest.Podman([]string{"ps", "-q", "--no-trunc"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring(session1.OutputToString()))
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session2.OutputToString())))

		result = podmanTest.Podman([]string{"container", "restore", "second"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("second"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
		Expect(podmanTest.GetContainerStatus()).To(Not(ContainSubstring("Exited")))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint all running container", func() {
		localRunString := getRunString([]string{"--name", "first", ALPINE, "top"})
		session1 := podmanTest.Podman(localRunString)
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())
		cid1 := session1.OutputToString()

		localRunString = getRunString([]string{"--name", "second", ALPINE, "top"})
		session2 := podmanTest.Podman(localRunString)
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		cid2 := session2.OutputToString()

		result := podmanTest.Podman([]string{"container", "checkpoint", "-a"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid1))
		Expect(result.OutputToString()).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		ps := podmanTest.Podman([]string{"ps", "-q", "--no-trunc"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session1.OutputToString())))
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session2.OutputToString())))

		result = podmanTest.Podman([]string{"container", "restore", "-a"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid1))
		Expect(result.OutputToString()).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
		Expect(podmanTest.GetContainerStatus()).To(Not(ContainSubstring("Exited")))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint container with established tcp connections", func() {
		localRunString := getRunString([]string{REDIS_IMAGE})
		session := podmanTest.PodmanExitCleanly(localRunString...)
		cid := session.OutputToString()
		if !WaitContainerReady(podmanTest, cid, "Ready to accept connections", 20, 1) {
			Fail("Container failed to get ready")
		}

		IP := podmanTest.PodmanExitCleanly("inspect", cid, fmt.Sprintf("--format={{(index .NetworkSettings.Networks \"%s\").IPAddress}}", netname))

		// Open a network connection to the redis server
		conn, err := net.DialTimeout("tcp4", IP.OutputToString()+":6379", time.Duration(3)*time.Second)
		Expect(err).ToNot(HaveOccurred())

		// This should fail as the container has established TCP connections
		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()

		// FIXME: criu emits an error message, but podman never sees it:
		//   "CRIU checkpointing failed -52.  Please check CRIU logfile /...."
		Expect(result).Should(ExitWithError(125, "failed: exit status 1"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Now it should work thanks to "--tcp-established"
		podmanTest.PodmanExitCleanly("container", "checkpoint", cid, "--tcp-established")

		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Restore should fail as the checkpoint image contains established TCP connections
		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		// Some older versions print "CRIU restoring failed: -52" while others
		// "Error: crun: (00.054135) Error (criu/cgroup.c:1998): cg: cgroupd: recv req error: No such file or directory: OCI runtime attempted to invoke a command that was not found"
		expectStderr := "cg: cgroupd: recv req error|CRIU restoring failed: -52"
		if podmanTest.OCIRuntime == "runc" {
			expectStderr = "runc: criu failed: type NOTIFY errno 0"
		}
		Expect(result).Should(ExitWithErrorRegex(125, expectStderr))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Now it should work thanks to "--tcp-established"
		podmanTest.PodmanExitCleanly("container", "restore", cid, "--tcp-established")
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		conn.Close()
	})

	It("podman restore container with tcp-close", func() {
		// Start a container with redis (which listens on tcp port)
		localRunString := getRunString([]string{REDIS_IMAGE})
		session := podmanTest.PodmanExitCleanly(localRunString...)
		cid := session.OutputToString()
		if !WaitContainerReady(podmanTest, cid, "Ready to accept connections", 20, 1) {
			Fail("Container failed to get ready")
		}

		// Get container IP
		IP := podmanTest.PodmanExitCleanly("inspect", cid, fmt.Sprintf("--format={{(index .NetworkSettings.Networks \"%s\").IPAddress}}", netname))

		// Open a network connection to the redis server
		conn, err := net.DialTimeout("tcp4", IP.OutputToString()+":6379", time.Duration(3)*time.Second)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()

		// Checkpoint with --tcp-established since we have an open connection
		podmanTest.PodmanExitCleanly("container", "checkpoint", cid, "--tcp-established")
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Restore should fail as the checkpoint image contains established TCP connections
		result := podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		// Some older versions print "CRIU restoring failed: -52" while others
		// "Error: crun: (00.054135) Error (criu/cgroup.c:1998): cg: cgroupd: recv req error: No such file or directory: OCI runtime attempted to invoke a command that was not found"
		expectStderr := "cg: cgroupd: recv req error|CRIU restoring failed: -52"
		if podmanTest.OCIRuntime == "runc" {
			expectStderr = "runc: criu failed: type NOTIFY errno 0"
		}
		Expect(result).Should(ExitWithErrorRegex(125, expectStderr))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Now it should work thanks to "--tcp-close"
		podmanTest.PodmanExitCleanly("container", "restore", cid, "--tcp-close")
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
	})

	It("podman checkpoint with --leave-running", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		// Checkpoint container, but leave it running
		result := podmanTest.Podman([]string{"container", "checkpoint", "--leave-running", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		// Make sure it is still running
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Stop the container
		podmanTest.StopContainer(cid)
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Restore the stopped container from the previous checkpoint
		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint and restore container with same IP", func() {
		localRunString := getRunString([]string{"--name", "test_name", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		IPBefore := podmanTest.Podman([]string{"inspect", "test_name", fmt.Sprintf("--format={{(index .NetworkSettings.Networks \"%s\").IPAddress}}", netname)})
		IPBefore.WaitWithDefaultTimeout()
		Expect(IPBefore).Should(ExitCleanly())
		Expect(IPBefore.OutputToString()).To(MatchRegexp("^[0-9]+(\\.[0-9]+){3}$"))

		MACBefore := podmanTest.Podman([]string{"inspect", "test_name", fmt.Sprintf("--format={{(index .NetworkSettings.Networks \"%s\").MacAddress}}", netname)})
		MACBefore.WaitWithDefaultTimeout()
		Expect(MACBefore).Should(ExitCleanly())
		Expect(MACBefore.OutputToString()).To(MatchRegexp("^[0-9a-f]{2}(:[0-9a-f]{2}){5}$"))

		result := podmanTest.Podman([]string{"container", "checkpoint", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "test_name"})
		result.WaitWithDefaultTimeout()

		IPAfter := podmanTest.Podman([]string{"inspect", "test_name", fmt.Sprintf("--format={{(index .NetworkSettings.Networks \"%s\").IPAddress}}", netname)})
		IPAfter.WaitWithDefaultTimeout()
		Expect(IPAfter).Should(ExitCleanly())

		MACAfter := podmanTest.Podman([]string{"inspect", "test_name", fmt.Sprintf("--format={{(index .NetworkSettings.Networks \"%s\").MacAddress}}", netname)})
		MACAfter.WaitWithDefaultTimeout()
		Expect(MACAfter).Should(ExitCleanly())

		// Check that IP address did not change between checkpointing and restoring
		Expect(IPAfter.OutputToString()).To(Equal(IPBefore.OutputToString()))

		// Check that MAC address did not change between checkpointing and restoring
		Expect(MACAfter.OutputToString()).To(Equal(MACBefore.OutputToString()))

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	// This test does the same steps which are necessary for migrating
	// a container from one host to another
	It("podman checkpoint container with export (migration)", func() {
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container the first time with different name.
		// Using '--ignore-static-ip' as for parallel test runs
		// each containers gets a random IP address via '--ip'.
		// '--ignore-static-ip' tells the restore to use the next
		// available IP address.
		// First restore the container with a new name/ID to make
		// sure nothing in the restored container depends on the
		// original container.
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName, "-n", "restore_again", "--ignore-static-ip"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})
	// This test does the same steps which are necessary for migrating
	// a container from one host to another
	It("podman checkpoint container with export and different compression algorithms", func() {
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		// Checkpoint with the default algorithm
		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Checkpoint with the zstd algorithm
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName, "--compress", "zstd"})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Checkpoint with the none algorithm
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName, "-c", "none"})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Checkpoint with the gzip algorithm
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName, "-c", "gzip"})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Checkpoint with the non-existing algorithm
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName, "-c", "non-existing"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitWithError(125, `selected compression algorithm ("non-existing") not supported. Please select one from`))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", "--time", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint and restore container with root file-system changes", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		// Change the container's root file-system
		result := podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo test" + cid + "test > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "rm /etc/motd"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"diff", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("C /etc"))
		Expect(result.OutputToString()).To(ContainSubstring("A /test.output"))
		Expect(result.OutputToString()).To(ContainSubstring("D /etc/motd"))
		Expect(result.OutputToStringArray()).To(HaveLen(3))

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the changes to the container's root file-system
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("test" + cid + "test"))

		result = podmanTest.Podman([]string{"diff", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("C /etc"))
		Expect(result.OutputToString()).To(ContainSubstring("A /test.output"))
		Expect(result.OutputToString()).To(ContainSubstring("D /etc/motd"))
		Expect(result.OutputToStringArray()).To(HaveLen(3))

		// Remove exported checkpoint
		os.Remove(fileName)
	})
	It("podman checkpoint and restore container with root file-system changes using --ignore-rootfs during restore", func() {
		// Start the container
		// test that restore works without network namespace (https://github.com/containers/podman/issues/14389)
		session := podmanTest.Podman([]string{"run", "--network=none", "-d", "--rm", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1), "# of running containers at start")
		cid := session.OutputToString()
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		// Change the container's root file-system
		signalFile := "/test.output"
		result := podmanTest.Podman([]string{"exec", cid, "touch", signalFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal(cid), "checkpoint output")
		// Allow a few seconds for --rm to take effect
		ncontainers := podmanTest.NumberOfContainers()
		for range 4 {
			if ncontainers == 0 {
				break
			}
			time.Sleep(time.Second)
			ncontainers = podmanTest.NumberOfContainers()
		}
		Expect(ncontainers).To(Equal(0), "# of containers (total) after checkpoint")

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "--ignore-rootfs", "-i", fileName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		runCheck := podmanTest.Podman([]string{"ps", "-a", "--noheading", "--no-trunc", "--format", "{{.ID}} {{.State}}"})
		runCheck.WaitWithDefaultTimeout()
		Expect(runCheck).Should(ExitCleanly())
		Expect(runCheck.OutputToString()).To(Equal(cid+" running"), "podman ps, after restore")

		// Verify the changes to the container's root file-system
		result = podmanTest.Podman([]string{"exec", cid, "cat", signalFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(1, "cat: can't open '"+signalFile+"': No such file or directory"))

		// Remove exported checkpoint
		os.Remove(fileName)
	})
	It("podman checkpoint and restore container with root file-system changes using --ignore-rootfs during checkpoint", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		// Change the container's root file-system
		result := podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo test" + cid + "test > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", "--ignore-rootfs", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the changes to the container's root file-system
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(1, "cat: can't open '/test.output': No such file or directory"))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint and run exec in restored container", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		// Checkpoint the container
		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Exec in the container
		result = podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo " + cid + " > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"exec", cid, "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint a container started with --rm", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		cid := session.OutputToString()
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		// Checkpoint the container - this should fail as it was started with --rm
		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, "cannot checkpoint containers that have been started with '--rm'"))

		// Checkpointing with --export should still work
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint a container with volumes", func() {
		session := podmanTest.Podman([]string{
			"build", "-f", "build/basicalpine/Containerfile.volume", "-t", "test-cr-volume",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Start the container
		localRunString := getRunString([]string{
			"--rm",
			"-v", "/volume1",
			"-v", "my-test-vol:/volume2",
			"test-cr-volume",
			"top",
		})
		session = podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		cid := session.OutputToString()

		// Add file in volume0
		result := podmanTest.Podman([]string{
			"exec", cid, "/bin/sh", "-c", "echo " + cid + " > /volume0/test.output",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// Add file in volume1
		result = podmanTest.Podman([]string{
			"exec", cid, "/bin/sh", "-c", "echo " + cid + " > /volume1/test.output",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// Add file in volume2
		result = podmanTest.Podman([]string{
			"exec", cid, "/bin/sh", "-c", "echo " + cid + " > /volume2/test.output",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		checkpointFileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", checkpointFileName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container should fail because named volume still exists
		result = podmanTest.Podman([]string{"container", "restore", "-i", checkpointFileName})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, "volume with name my-test-vol already exists. Use --ignore-volumes to not restore content of volumes"))

		// Remove named volume
		session = podmanTest.Podman([]string{"volume", "rm", "my-test-vol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Restoring container
		result = podmanTest.Podman([]string{"container", "restore", "-i", checkpointFileName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Validate volume0 content
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/volume0/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))

		// Validate volume1 content
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/volume1/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))

		// Validate volume2 content
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/volume2/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))

		// Remove exported checkpoint
		os.Remove(checkpointFileName)
	})

	It("podman checkpoint container with --pre-checkpoint", func() {
		if podmanTest.Host.Arch == "arm64" {
			Skip("skip on arm64/aarch64, https://github.com/checkpoint-restore/criu/issues/2676")
		}
		SkipIfContainerized("FIXME: #24230 - no longer works in container testing")
		if !criu.MemTrack() {
			Skip("system (architecture/kernel/CRIU) does not support memory tracking")
		}
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "checkpoint", "-P", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"container", "checkpoint", "--with-previous", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
	})

	It("podman checkpoint container with --pre-checkpoint and export (migration)", func() {
		if podmanTest.Host.Arch == "arm64" {
			Skip("skip on arm64/aarch64, https://github.com/checkpoint-restore/criu/issues/2676")
		}
		SkipIfContainerized("FIXME: #24230 - no longer works in container testing")
		SkipIfRemote("--import-previous is not yet supported on the remote client")
		if !criu.MemTrack() {
			Skip("system (architecture/kernel/CRIU) does not support memory tracking")
		}
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		preCheckpointFileName := filepath.Join(podmanTest.TempDir, "/pre-checkpoint-"+cid+".tar.gz")
		checkpointFileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		result := podmanTest.Podman([]string{"container", "checkpoint", "-P", "-e", preCheckpointFileName, cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"container", "checkpoint", "--with-previous", "-e", checkpointFileName, cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{"container", "restore", "-i", checkpointFileName, "--import-previous", preCheckpointFileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		os.Remove(checkpointFileName)
		os.Remove(preCheckpointFileName)
	})

	It("podman checkpoint and restore container with different port mappings", func() {
		randomPort, err := utils.GetRandomPort()
		Expect(err).ShouldNot(HaveOccurred())
		localRunString := getRunString([]string{"-p", fmt.Sprintf("%d:6379", randomPort), "--rm", REDIS_IMAGE})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		if !WaitContainerReady(podmanTest, cid, "Ready to accept connections", 20, 1) {
			Fail("Container failed to get ready")
		}

		GinkgoWriter.Printf("Trying to connect to redis server at localhost:%d\n", randomPort)
		// Open a network connection to the redis server via initial port mapping
		conn, err := net.DialTimeout("tcp4", fmt.Sprintf("localhost:%d", randomPort), time.Duration(3)*time.Second)
		Expect(err).ShouldNot(HaveOccurred())
		conn.Close()

		// Checkpoint the container
		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container with different port mapping
		newRandomPort, err := utils.GetRandomPort()
		Expect(err).ShouldNot(HaveOccurred())
		result = podmanTest.Podman([]string{"container", "restore", "-p", fmt.Sprintf("%d:6379", newRandomPort), "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Open a network connection to the redis server via initial port mapping
		// This should fail
		_, err = net.DialTimeout("tcp4", fmt.Sprintf("localhost:%d", randomPort), time.Duration(3)*time.Second)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connection refused"))
		// Open a network connection to the redis server via new port mapping
		GinkgoWriter.Printf("Trying to reconnect to redis server at localhost:%d\n", newRandomPort)
		conn, err = net.DialTimeout("tcp4", fmt.Sprintf("localhost:%d", newRandomPort), time.Duration(3)*time.Second)
		Expect(err).ShouldNot(HaveOccurred())
		conn.Close()

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	namespaceCombination := []string{
		"ipc,net,uts,pid",
		"ipc,net,uts",
		"ipc,net",
		"net,uts,pid",
		"net,uts",
		"uts,pid",
	}
	for index, share := range namespaceCombination {
		testName := fmt.Sprintf(
			"podman checkpoint and restore container out of and into pod (%s)",
			share,
		)

		It(testName, func() {
			Skip("FIXME: #24571 - not working an super flaky, don't waste CI time on it")
			podName := "test_pod"

			if err := criu.CheckForCriu(criu.PodCriuVersion); err != nil {
				Skip(fmt.Sprintf("check CRIU pod version error: %v", err))
			}

			if !crutils.CRRuntimeSupportsPodCheckpointRestore(podmanTest.OCIRuntime) {
				Skip("runtime does not support pod restore: " + podmanTest.OCIRuntime)
			}

			// Create a pod
			session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", share})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitCleanly())
			podID := session.OutputToString()

			session = podmanTest.Podman([]string{
				"run",
				"-d",
				"--rm",
				"--pod",
				podID,
				ALPINE,
				"top",
			})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitCleanly())
			cid := session.OutputToString()

			fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

			// Checkpoint the container
			result := podmanTest.Podman([]string{
				"container",
				"checkpoint",
				"-e",
				fileName,
				cid,
			})
			result.WaitWithDefaultTimeout()

			// As the container has been started with '--rm' it will be completely
			// cleaned up after checkpointing.
			// #11784 (closed wontfix): runc warns "lstat /sys/.../machine.slice/...: ENOENT"
			// so we can't use ExitCleanly()
			if podmanTest.OCIRuntime == "runc" {
				Expect(result).To(Exit(0))
			} else {
				Expect(result).To(ExitCleanly())
			}
			Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
			Expect(podmanTest.NumberOfContainers()).To(Equal(1))

			// Remove the pod and create a new pod
			result = podmanTest.Podman([]string{
				"pod",
				"rm",
				podID,
			})
			result.WaitWithDefaultTimeout()
			Expect(result).To(ExitCleanly())

			// First create a pod with different shared namespaces.
			// Restore should fail

			wrongShare := share[:strings.LastIndex(share, ",")]

			session = podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", wrongShare})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitCleanly())
			podID = session.OutputToString()

			// Restore container with different port mapping
			result = podmanTest.Podman([]string{
				"container",
				"restore",
				"--pod",
				podID,
				"-i",
				fileName,
			})
			result.WaitWithDefaultTimeout()
			Expect(result).To(ExitWithError(125, "does not share the "))

			// Remove the pod and create a new pod
			result = podmanTest.Podman([]string{
				"pod",
				"rm",
				podID,
			})
			result.WaitWithDefaultTimeout()
			Expect(result).To(ExitCleanly())

			session = podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", share})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitCleanly())
			podID = session.OutputToString()

			// Restore container into Pod.
			// Verify that restore works with both Pod name and ID.
			podArg := podName
			if index%2 == 1 {
				podArg = podID
			}
			result = podmanTest.Podman([]string{"container", "restore", "--pod", podArg, "-i", fileName})
			result.WaitWithDefaultTimeout()

			Expect(result).To(ExitCleanly())
			Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
			Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

			result = podmanTest.Podman([]string{
				"rm",
				"-f",
				result.OutputToString(),
			})
			result.WaitWithDefaultTimeout()
			// #11784 (closed wontfix): runc warns "lstat /sys/.../machine.slice/...: ENOENT"
			// so we can't use ExitCleanly()
			if podmanTest.OCIRuntime == "runc" {
				Expect(result).To(Exit(0))
			} else {
				Expect(result).To(ExitCleanly())
			}
			Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
			Expect(podmanTest.NumberOfContainers()).To(Equal(1))

			result = podmanTest.Podman([]string{
				"pod",
				"rm",
				"-fa",
			})
			result.WaitWithDefaultTimeout()
			Expect(result).To(ExitCleanly())

			// Remove exported checkpoint
			os.Remove(fileName)
		})
	}

	It("podman checkpoint container with export (migration) and --ipc host", func() {
		localRunString := getRunString([]string{"--rm", "--ipc", "host", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		// Cannot use ExitCleanly() because "skipping [ssh-agent-path] since it is a socket"
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint container with export and statistics", func() {
		localRunString := getRunString([]string{
			"--rm",
			ALPINE,
			"top",
		})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")
		defer os.Remove(fileName)

		result := podmanTest.Podman([]string{
			"container",
			"checkpoint",
			cid, "-e",
			fileName,
		})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Extract checkpoint archive
		destinationDirectory := filepath.Join(podmanTest.TempDir, "dest")
		err = os.MkdirAll(destinationDirectory, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		tarsession := SystemExec(
			"tar",
			[]string{
				"xf",
				fileName,
				"-C",
				destinationDirectory,
			},
		)
		Expect(tarsession).Should(ExitCleanly())

		_, err = os.Stat(filepath.Join(destinationDirectory, stats.StatsDump))
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("podman checkpoint and restore containers with --print-stats", func() {
		session1 := podmanTest.Podman(getRunString([]string{REDIS_IMAGE}))
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		session2 := podmanTest.Podman(getRunString([]string{REDIS_IMAGE, "top"}))
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())

		result := podmanTest.Podman([]string{
			"container",
			"checkpoint",
			"-a",
			"--print-stats",
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		type checkpointStatistics struct {
			PodmanDuration      int64                        `json:"podman_checkpoint_duration"`
			ContainerStatistics []*entities.CheckpointReport `json:"container_statistics"`
		}

		cS := new(checkpointStatistics)
		err := json.Unmarshal([]byte(result.OutputToString()), cS)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(cS.ContainerStatistics).To(HaveLen(2))
		Expect(cS.PodmanDuration).To(BeNumerically(">", cS.ContainerStatistics[0].RuntimeDuration))
		Expect(cS.PodmanDuration).To(BeNumerically(">", cS.ContainerStatistics[1].RuntimeDuration))
		Expect(cS.ContainerStatistics[0].RuntimeDuration).To(
			BeNumerically(">", cS.ContainerStatistics[0].CRIUStatistics.FrozenTime),
		)
		Expect(cS.ContainerStatistics[1].RuntimeDuration).To(
			BeNumerically(">", cS.ContainerStatistics[1].CRIUStatistics.FrozenTime),
		)

		ps := podmanTest.Podman([]string{
			"ps",
			"-q",
			"--no-trunc",
		})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session1.OutputToString())))
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session2.OutputToString())))

		result = podmanTest.Podman([]string{
			"container",
			"restore",
			"-a",
			"--print-stats",
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
		Expect(podmanTest.GetContainerStatus()).To(Not(ContainSubstring("Exited")))

		type restoreStatistics struct {
			PodmanDuration      int64                     `json:"podman_restore_duration"`
			ContainerStatistics []*entities.RestoreReport `json:"container_statistics"`
		}

		rS := new(restoreStatistics)
		err = json.Unmarshal([]byte(result.OutputToString()), rS)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(cS.ContainerStatistics).To(HaveLen(2))
		Expect(cS.PodmanDuration).To(BeNumerically(">", cS.ContainerStatistics[0].RuntimeDuration))
		Expect(cS.PodmanDuration).To(BeNumerically(">", cS.ContainerStatistics[1].RuntimeDuration))
		Expect(cS.ContainerStatistics[0].RuntimeDuration).To(
			BeNumerically(">", cS.ContainerStatistics[0].CRIUStatistics.RestoreTime),
		)
		Expect(cS.ContainerStatistics[1].RuntimeDuration).To(
			BeNumerically(">", cS.ContainerStatistics[1].CRIUStatistics.RestoreTime),
		)

		result = podmanTest.Podman([]string{
			"rm",
			"-t",
			"0",
			"-fa",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint and restore container with --file-locks", func() {
		localRunString := getRunString([]string{"--name", "test_name", ALPINE, "flock", "test.lock", "sh", "-c", "echo READY;sleep 100"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(WaitContainerReady(podmanTest, "test_name", "READY", 5, 1)).To(BeTrue(), "Timed out waiting for READY")

		// Checkpoint is expected to fail without --file-locks
		result := podmanTest.Podman([]string{"container", "checkpoint", "test_name"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "failed: exit status 1"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		// Checkpoint is expected to succeed with --file-locks
		result = podmanTest.Podman([]string{"container", "checkpoint", "--file-locks", "test_name"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "--file-locks", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-f", "test_name"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint container with export and verify runtime", func() {
		SkipIfRemote("podman-remote does not support --runtime flag")
		localRunString := getRunString([]string{
			"--rm",
			ALPINE,
			"top",
		})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{
			"inspect",
			"--format",
			"{{.OCIRuntime}}",
			cid,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		runtime := session.OutputToString()

		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		result := podmanTest.Podman([]string{
			"container",
			"checkpoint",
			cid, "-e",
			fileName,
		})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		result = podmanTest.Podman([]string{
			"container",
			"restore",
			"-i",
			fileName,
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// The restored container should have the same runtime as the original container
		result = podmanTest.Podman([]string{
			"inspect",
			"--format",
			"{{.OCIRuntime}}",
			cid,
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(runtime))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint container with export and verify non-default runtime", func() {
		SkipIfRemote("podman-remote does not support --runtime flag")
		// This test triggers the edge case where:
		// 1. Default runtime is crun
		// 2. Container is created with runc
		// 3. Checkpoint without setting --runtime into archive
		// 4. Restore without setting --runtime from archive
		// It should be expected that podman identifies runtime
		// from the checkpoint archive.

		// Prevent --runtime arg from being set to force using default
		// runtime unless explicitly set through passed args.
		preservedMakeOptions := podmanTest.PodmanMakeOptions
		podmanTest.PodmanMakeOptions = func(args []string, options PodmanExecOptions) []string {
			defaultArgs := preservedMakeOptions(args, options)
			if slices.Contains(args, "--runtime") {
				// Runtime is set explicitly, so we should keep --runtime arg.
				return defaultArgs
			}
			updatedArgs := make([]string, 0)
			for i := 0; i < len(defaultArgs); i++ {
				// Remove --runtime arg, letting podman fall back to its default
				if defaultArgs[i] == "--runtime" {
					i++
				} else {
					updatedArgs = append(updatedArgs, defaultArgs[i])
				}
			}
			return updatedArgs
		}

		for _, runtime := range []string{"runc", "crun"} {
			if err := exec.Command(runtime, "--help").Run(); err != nil {
				Skip(fmt.Sprintf("%s not found in PATH; this test requires both runc and crun", runtime))
			}
		}

		// Detect default runtime
		session := podmanTest.Podman([]string{"info", "--format", "{{.Host.OCIRuntime.Name}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if defaultRuntime := session.OutputToString(); defaultRuntime != "crun" {
			Skip(fmt.Sprintf("Default runtime is %q; this test requires crun to be default", defaultRuntime))
		}

		// Force non-default runtime "runc"
		localRunString := getRunString([]string{"--runtime", "runc", "--rm", ALPINE, "top"})
		session = podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.OCIRuntime}}", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("runc"))

		checkpointExportPath := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		session = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", checkpointExportPath})
		session.WaitWithDefaultTimeout()
		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		session = podmanTest.Podman([]string{"container", "restore", "-i", checkpointExportPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// The restored container should have the same runtime as the original container
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.OCIRuntime}}", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("runc"))

		// Remove exported checkpoint
		os.Remove(checkpointExportPath)
	})

	It("podman checkpoint container with export and try to change the runtime", func() {
		SkipIfRemote("podman-remote does not support --runtime flag")
		// This test will only run if runc and crun both exist
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test requires crun and runc")
		}
		cmd := exec.Command("runc")
		if err := cmd.Start(); err != nil {
			Skip("Test requires crun and runc")
		}
		if err := cmd.Wait(); err != nil {
			Skip("Test requires crun and runc")
		}
		localRunString := getRunString([]string{
			"--rm",
			ALPINE,
			"top",
		})
		// Let's start a container with runc and try to restore it with crun (expected to fail)
		localRunString = append(
			[]string{
				"--runtime",
				"runc",
			},
			localRunString...,
		)
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{
			"inspect",
			"--format",
			"{{.OCIRuntime}}",
			cid,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		runtime := session.OutputToString()

		fileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")

		result := podmanTest.Podman([]string{
			"container",
			"checkpoint",
			cid, "-e",
			fileName,
		})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// This should fail as the container was checkpointed with runc
		result = podmanTest.Podman([]string{
			"--runtime",
			"crun",
			"container",
			"restore",
			"-i",
			fileName,
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitWithError(125, "and cannot be restored with runtime"))

		result = podmanTest.Podman([]string{
			"--runtime",
			"runc",
			"container",
			"restore",
			"-i",
			fileName,
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{
			"inspect",
			"--format",
			"{{.OCIRuntime}}",
			cid,
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal(runtime))

		result = podmanTest.Podman([]string{
			"--runtime",
			"runc",
			"rm",
			"-fa",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint and restore dev/shm content with --export and --import", func() {
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()

		// Add test file in dev/shm
		result := podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo test" + cid + "test > /dev/shm/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.OCIRuntime}}", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		runtime := session.OutputToString()

		checkpointFileName := filepath.Join(podmanTest.TempDir, "/checkpoint-"+cid+".tar.gz")
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", checkpointFileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		result = podmanTest.Podman([]string{"container", "restore", "-i", checkpointFileName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// The restored container should have the same runtime as the original container
		result = podmanTest.Podman([]string{"inspect", "--format", "{{.OCIRuntime}}", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(runtime))

		// Verify the test file content in dev/shm
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/dev/shm/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("test" + cid + "test"))

		// Remove exported checkpoint
		os.Remove(checkpointFileName)
	})

	It("podman checkpoint and restore dev/shm content", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()

		// Add test file in dev/shm
		result := podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo test" + cid + "test > /dev/shm/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the test file content in dev/shm
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/dev/shm/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("test" + cid + "test"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})
})
