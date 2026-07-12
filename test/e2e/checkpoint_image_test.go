//go:build linux || freebsd

package integration

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/pkg/criu"
	. "go.podman.io/podman/v6/test/utils"
)

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
	})

	It("podman checkpoint --create-image with bogus container", func() {
		checkpointImage := "foobar-checkpoint"
		session := podmanTest.Podman([]string{"container", "checkpoint", "--create-image", checkpointImage, "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `no container with name or ID "foobar" found: no such container`))
	})

	It("podman checkpoint --create-image with running container", func() {
		// Container image must be lowercase
		checkpointImage := "alpine-checkpoint-" + strings.ToLower(RandomString(6))
		containerName := "alpine-container-" + RandomString(6)

		localRunString := []string{
			"run",
			"-d",
			"--ip", GetSafeIPAddress(),
			"--name", containerName,
			ALPINE,
			"top",
		}
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		containerID := session.OutputToString()

		// Checkpoint image should not exist
		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.LineInOutputContainsTag("localhost/"+checkpointImage, "latest")).To(BeFalse())

		// Check if none of the checkpoint/restore specific information is displayed
		// for newly started containers.
		inspect := podmanTest.Podman([]string{"inspect", containerID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		inspectOut := inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Checkpointed).To(BeFalse(), ".State.Checkpointed")
		Expect(inspectOut[0].State.Restored).To(BeFalse(), ".State.Restored")
		Expect(inspectOut[0].State).To(HaveField("CheckpointPath", ""))
		Expect(inspectOut[0].State).To(HaveField("CheckpointLog", ""))
		Expect(inspectOut[0].State).To(HaveField("RestoreLog", ""))

		result := podmanTest.Podman([]string{"container", "checkpoint", "--create-image", checkpointImage, "--keep", containerID})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		inspect = podmanTest.Podman([]string{"inspect", containerID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		inspectOut = inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Checkpointed).To(BeTrue(), ".State.Checkpointed")
		Expect(inspectOut[0].State.CheckpointPath).To(ContainSubstring("userdata/checkpoint"))
		Expect(inspectOut[0].State.CheckpointLog).To(ContainSubstring("userdata/dump.log"))

		// Check if checkpoint image has been created
		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.LineInOutputContainsTag("localhost/"+checkpointImage, "latest")).To(BeTrue())

		// Check if the checkpoint image contains annotations
		inspect = podmanTest.Podman([]string{"inspect", checkpointImage})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		inspectImageOut := inspect.InspectImageJSON()
		Expect(inspectImageOut[0].Annotations["io.podman.annotations.checkpoint.name"]).To(
			BeEquivalentTo(containerName),
			"io.podman.annotations.checkpoint.name",
		)

		ociRuntimeName := ""
		if strings.Contains(podmanTest.OCIRuntime, "runc") {
			ociRuntimeName = "runc"
		} else if strings.Contains(podmanTest.OCIRuntime, "crun") {
			ociRuntimeName = "crun"
		}
		if ociRuntimeName != "" {
			Expect(inspectImageOut[0].Annotations["io.podman.annotations.checkpoint.runtime.name"]).To(
				BeEquivalentTo(ociRuntimeName),
				"io.podman.annotations.checkpoint.runtime.name",
			)
		}

		// Remove existing container
		result = podmanTest.Podman([]string{"rm", "-t", "1", "-f", containerID})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// Restore container from checkpoint image
		result = podmanTest.Podman([]string{"container", "restore", checkpointImage})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Clean-up
		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{"rmi", checkpointImage})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman restore multiple containers from single checkpoint image", func() {
		// Container image must be lowercase
		checkpointImage := "alpine-checkpoint-" + strings.ToLower(RandomString(6))
		containerName := "alpine-container-" + RandomString(6)

		localRunString := []string{"run", "-d", "--name", containerName, ALPINE, "top"}
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		containerID := session.OutputToString()

		// Checkpoint image should not exist
		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.LineInOutputContainsTag("localhost/"+checkpointImage, "latest")).To(BeFalse())

		result := podmanTest.Podman([]string{"container", "checkpoint", "--create-image", checkpointImage, "--keep", containerID})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Check if checkpoint image has been created
		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.LineInOutputContainsTag("localhost/"+checkpointImage, "latest")).To(BeTrue())

		// Remove existing container
		result = podmanTest.Podman([]string{"rm", "-t", "1", "-f", containerID})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		for i := 1; i < 5; i++ {
			// Restore container from checkpoint image
			name := containerName + strconv.Itoa(i)
			result = podmanTest.Podman([]string{"container", "restore", "--name", name, checkpointImage})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(ExitCleanly())
			Expect(podmanTest.NumberOfContainersRunning()).To(Equal(i))

			// Check that the container is running
			status := podmanTest.Podman([]string{"inspect", name, "--format={{.State.Status}}"})
			status.WaitWithDefaultTimeout()
			Expect(status).Should(ExitCleanly())
			Expect(status.OutputToString()).To(Equal("running"))
		}

		// Clean-up
		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{"rmi", checkpointImage})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman restore multiple containers from multiple checkpoint images", func() {
		// Container image must be lowercase
		checkpointImage1 := "alpine-checkpoint-" + strings.ToLower(RandomString(6))
		checkpointImage2 := "alpine-checkpoint-" + strings.ToLower(RandomString(6))
		containerName1 := "alpine-container-" + RandomString(6)
		containerName2 := "alpine-container-" + RandomString(6)

		// Create first container
		localRunString := []string{"run", "-d", "--name", containerName1, ALPINE, "top"}
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		containerID1 := session.OutputToString()

		// Create second container
		localRunString = []string{"run", "-d", "--name", containerName2, ALPINE, "top"}
		session = podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		containerID2 := session.OutputToString()

		// Checkpoint first container
		result := podmanTest.Podman([]string{"container", "checkpoint", "--create-image", checkpointImage1, "--keep", containerID1})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		// Checkpoint second container
		result = podmanTest.Podman([]string{"container", "checkpoint", "--create-image", checkpointImage2, "--keep", containerID2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		// Remove existing containers
		result = podmanTest.Podman([]string{"rm", "-t", "1", "-f", containerName1, containerName2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// Restore both containers from images
		result = podmanTest.Podman([]string{"container", "restore", checkpointImage1, checkpointImage2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))

		// Check if first container is running
		status := podmanTest.Podman([]string{"inspect", containerName1, "--format={{.State.Status}}"})
		status.WaitWithDefaultTimeout()
		Expect(status).Should(ExitCleanly())
		Expect(status.OutputToString()).To(Equal("running"))

		// Check if second container is running
		status = podmanTest.Podman([]string{"inspect", containerName2, "--format={{.State.Status}}"})
		status.WaitWithDefaultTimeout()
		Expect(status).Should(ExitCleanly())
		Expect(status.OutputToString()).To(Equal("running"))

		// Clean-up
		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{"rmi", checkpointImage1, checkpointImage2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman run with checkpoint image", func() {
		// Container image must be lowercase
		checkpointImage := "alpine-checkpoint-" + strings.ToLower(RandomString(6))
		containerName := "alpine-container-" + RandomString(6)

		// Create container
		localRunString := []string{"run", "-d", "--name", containerName, ALPINE, "top"}
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		containerID1 := session.OutputToString()

		// Checkpoint container, create checkpoint image
		result := podmanTest.Podman([]string{"container", "checkpoint", "--create-image", checkpointImage, "--keep", containerID1})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		// Remove existing container
		result = podmanTest.Podman([]string{"rm", "-t", "1", "-f", containerName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// Restore containers from image using `podman run`
		result = podmanTest.Podman([]string{"run", checkpointImage})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		// Check if the container is running
		status := podmanTest.Podman([]string{"inspect", containerName, "--format={{.State.Status}}"})
		status.WaitWithDefaultTimeout()
		Expect(status).Should(ExitCleanly())
		Expect(status.OutputToString()).To(Equal("running"))

		// Clean-up
		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{"rmi", checkpointImage})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})
})
