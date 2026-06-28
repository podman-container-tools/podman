//go:build linux || freebsd

package integration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
	"golang.org/x/sys/unix"
)

const (
	sigCatch  = "trap \"echo FOO >> /h/fifo \" 8; echo READY >> /h/fifo; while :; do sleep 0.25; done"
	sigCatch2 = "trap \"echo Received\" SIGFPE; echo READY; while :; do sleep 0.25; done"
)

var _ = Describe("Podman run with --sig-proxy", func() {
	Specify("signals are forwarded to container using sig-proxy", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("Doesn't work on ppc64le")
		}
		signal := syscall.SIGFPE
		// Set up a socket for communication
		udsDir := filepath.Join(tempdir, "socket")
		err := os.Mkdir(udsDir, 0o700)
		Expect(err).ToNot(HaveOccurred())
		udsPath := filepath.Join(udsDir, "fifo")
		err = syscall.Mkfifo(udsPath, 0o600)
		Expect(err).ToNot(HaveOccurred())
		if isRootless() {
			err = podmanTest.RestoreArtifact(FEDORA_MINIMAL)
			Expect(err).ToNot(HaveOccurred())
		}
		_, pid := podmanTest.PodmanPID([]string{"run", "-v", fmt.Sprintf("%s:/h:Z", udsDir), FEDORA_MINIMAL, "bash", "-c", sigCatch})

		uds, _ := os.OpenFile(udsPath, os.O_RDONLY|syscall.O_NONBLOCK, 0o600)
		defer uds.Close()

		// Wait for the script in the container to alert us that it is READY
		counter := 0
		for {
			buf := make([]byte, 1024)
			n, err := uds.Read(buf)
			if err != nil && err != io.EOF {
				GinkgoWriter.Println(err)
				return
			}
			data := string(buf[0:n])
			if strings.Contains(data, "READY") {
				break
			}
			time.Sleep(1 * time.Second)
			if counter == 15 {
				Fail("Timed out waiting for READY from container")
			}
			counter++
		}
		// Ok, container is up and running now and so is the script

		if err := unix.Kill(pid, signal); err != nil {
			Fail(fmt.Sprintf("error killing podman process %d: %v", pid, err))
		}

		// The sending of the signal above will send FOO to the socket; here we
		// listen to the socket for that.
		counter = 0
		for {
			buf := make([]byte, 1024)
			n, err := uds.Read(buf)
			if err != nil {
				GinkgoWriter.Println(err)
				return
			}
			data := string(buf[0:n])
			if strings.Contains(data, "FOO") {
				break
			}
			time.Sleep(1 * time.Second)
			if counter == 15 {
				Fail("timed out waiting for FOO from container")
			}
			counter++
		}
	})

	Specify("signals are not forwarded to container with sig-proxy false", func() {
		signal := syscall.SIGFPE
		session, pid := podmanTest.PodmanPID([]string{"run", "--name", "test2", "--sig-proxy=false", FEDORA_MINIMAL, "bash", "-c", sigCatch2})

		// need to ensure the process is ready to get signals
		Expect(podmanTest.WaitContainerReady("test2", "READY", 5, 1)).To(BeTrue(), "bash signal listener ready")

		// Kill with given signal
		// Should be no output, SIGPOLL is usually ignored
		if err := unix.Kill(pid, signal); err != nil {
			Fail(fmt.Sprintf("error killing podman process %d: %v", pid, err))
		}

		// Kill with -9 to guarantee the container dies
		killSession := podmanTest.Podman([]string{"kill", "-s", "9", "test2"})
		killSession.WaitWithDefaultTimeout()
		Expect(killSession).Should(ExitCleanly())

		session.WaitWithDefaultTimeout()
		// Exit code is normally 2, however with GOTRACEBACK=crash (default in
		// Fedora/RHEL rpm builds) it will be 134 thus allow both.
		// https://github.com/containers/podman/issues/24213
		errorMsg := "SIGFPE: floating-point exception"
		Expect(session).To(Or(ExitWithError(2, errorMsg), ExitWithError(134, errorMsg)))
		Expect(session.OutputToString()).To(Not(ContainSubstring("Received")))
	})
})
