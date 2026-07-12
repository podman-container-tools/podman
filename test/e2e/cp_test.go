//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

// NOTE: Only smoke tests.  The system tests (i.e., "./test/system/*") take
// care of function and regression tests.  Please consider adding system tests
// rather than e2e tests.  System tests are used in RHEL gating.

var _ = Describe("Podman cp", func() {
	// Copy a file to the container, then back to the host and make sure
	// that the contents match.
	It("podman cp file", func() {
		srcFile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		originalContent := []byte("podman cp file test")
		err = os.WriteFile(srcFile.Name(), originalContent, 0o644)
		Expect(err).ToNot(HaveOccurred())

		// Create a container. NOTE that container mustn't be running for copying.
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		name := session.OutputToString()

		// Copy TO the container.

		// Cannot copy to a nonexistent path (note the trailing "/").
		session = podmanTest.Podman([]string{"cp", srcFile.Name(), name + ":foo/"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `"foo/" could not be found on container`))

		// The file will now be created (and written to).
		session = podmanTest.Podman([]string{"cp", srcFile.Name(), name + ":foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Copy FROM the container.

		destFile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer destFile.Close()
		defer os.Remove(destFile.Name())

		session = podmanTest.Podman([]string{"cp", name + ":foo", destFile.Name()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"start", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Now make sure the content matches.
		roundtripContent, err := os.ReadFile(destFile.Name())
		Expect(err).ToNot(HaveOccurred())
		Expect(roundtripContent).To(Equal(originalContent))
	})

	// Copy a file to the container, then back to the host in --pid=host
	It("podman cp --pid=host file", func() {
		srcFile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		originalContent := []byte("podman cp file test")
		err = os.WriteFile(srcFile.Name(), originalContent, 0o644)
		Expect(err).ToNot(HaveOccurred())

		// Create a container. NOTE that container mustn't be running for copying.
		session := podmanTest.Podman([]string{"create", "--pid=host", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"start", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// The file will now be created (and written to).
		session = podmanTest.Podman([]string{"cp", srcFile.Name(), name + ":foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Copy FROM the container.

		destFile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer destFile.Close()
		defer os.Remove(destFile.Name())

		session = podmanTest.Podman([]string{"cp", name + ":foo", destFile.Name()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Now make sure the content matches.
		roundtripContent, err := os.ReadFile(destFile.Name())
		Expect(err).ToNot(HaveOccurred())
		Expect(roundtripContent).To(Equal(originalContent))
	})

	// Create a symlink in the container, use it as a copy destination and
	// make sure that the link and the resolved path are accessible and
	// give the right content.
	It("podman cp symlink", func() {
		srcFile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		originalContent := []byte("podman cp symlink test")
		err = os.WriteFile(srcFile.Name(), originalContent, 0o644)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"exec", name, "ln", "-s", "/tmp", "/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"cp", "--pause=false", srcFile.Name(), name + ":/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", name, "cat", "/tmp/" + filepath.Base(srcFile.Name())})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(string(originalContent)))

		session = podmanTest.Podman([]string{"exec", name, "cat", "/test/" + filepath.Base(srcFile.Name())})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(string(originalContent)))
	})

	// Copy a file to a volume in the container.  The tricky part is that
	// containers mustn't be running for copying, so Podman has to do some
	// intense Yoga and 1) detect volume paths on the container, 2) resolve
	// the path to the volume's mount point on the host, and 3) copy the
	// data to the volume and not the container.
	It("podman cp volume", func() {
		srcFile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		originalContent := []byte("podman cp volume")
		err = os.WriteFile(srcFile.Name(), originalContent, 0o644)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"volume", "create", "data"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "-v", "data:/data", "--name", "container1", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"cp", srcFile.Name(), "container1" + ":/data/file.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Now get the volume's mount point, read the file and make
		// sure the contents match.
		session = podmanTest.Podman([]string{"volume", "inspect", "data", "--format", "{{.Mountpoint}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		volumeMountPoint := session.OutputToString()
		copiedContent, err := os.ReadFile(filepath.Join(volumeMountPoint, "file.txt"))
		Expect(err).ToNot(HaveOccurred())
		Expect(copiedContent).To(Equal(originalContent))
	})

	// Create another user in the container, let them create a file, copy
	// it to the host and back to the container and make sure that we can
	// access it, and (roughly) the right users own it.
	It("podman cp from ctr chown ", func() {
		srcFile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		setup := podmanTest.RunTopContainer("testctr")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "testctr", "adduser", "-S", "testuser"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "-u", "testuser", "testctr", "touch", "/tmp/testfile"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"cp", "--pause=false", "testctr:/tmp/testfile", srcFile.Name()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// owner of the file copied to local machine is not testuser
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		cmd := exec.Command("ls", "-l", srcFile.Name())
		cmdRet, err := cmd.Output()
		Expect(err).ToNot(HaveOccurred())
		Expect(string(cmdRet)).To(ContainSubstring(u.Username))

		session = podmanTest.Podman([]string{"cp", "--pause=false", srcFile.Name(), "testctr:testfile2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// owner of the file copied to a container is the root user
		session = podmanTest.Podman([]string{"exec", "testctr", "ls", "-l", "testfile2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("root"))
	})

	// Copy the root dir "/" of a container to the host.
	It("podman cp the root directory from the ctr to an existing directory on the host ", func() {
		container := "copyroottohost"
		session := podmanTest.RunTopContainer(container)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", container, "touch", "/dummy.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		tmpDir := GinkgoT().TempDir()

		session = podmanTest.Podman([]string{"cp", container + ":/", tmpDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cmd := exec.Command("ls", "-la", tmpDir)
		output, err := cmd.Output()
		lsOutput := string(output)
		Expect(err).ToNot(HaveOccurred())
		Expect(lsOutput).To(ContainSubstring("dummy.txt"))
		Expect(lsOutput).To(ContainSubstring("tmp"))
		Expect(lsOutput).To(ContainSubstring("etc"))
		Expect(lsOutput).To(ContainSubstring("var"))
		Expect(lsOutput).To(ContainSubstring("bin"))
		Expect(lsOutput).To(ContainSubstring("usr"))
	})

	It("podman cp to volume in container that is not running", func() {
		volName := "testvol"
		ctrName := "testctr"
		imgName := "testimg"
		ctrVolPath := "/test/"

		dockerfile := fmt.Sprintf(`FROM %s
RUN mkdir %s
RUN chown 9999:9999 %s`, ALPINE, ctrVolPath, ctrVolPath)

		podmanTest.BuildImage(dockerfile, imgName, "false")

		srcFile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		_ = podmanTest.PodmanExitCleanly("volume", "create", volName)
		_ = podmanTest.PodmanExitCleanly("create", "--name", ctrName, "-v", fmt.Sprintf("%s:%s", volName, ctrVolPath), imgName, "sh")

		_ = podmanTest.PodmanExitCleanly("cp", srcFile.Name(), fmt.Sprintf("%s:%s", ctrName, ctrVolPath))

		ls := podmanTest.PodmanExitCleanly("run", "-v", fmt.Sprintf("%s:%s", volName, ctrVolPath), ALPINE, "ls", "-al", ctrVolPath)
		Expect(ls.OutputToString()).To(ContainSubstring("9999 9999"))
		Expect(ls.OutputToString()).To(ContainSubstring(filepath.Base(srcFile.Name())))

		// Test for #25585
		_ = podmanTest.PodmanExitCleanly("rm", ctrName)
		_ = podmanTest.PodmanExitCleanly("create", "--name", ctrName, "-v", fmt.Sprintf("%s:%s", volName, ctrVolPath), imgName, "sh")
		_ = podmanTest.PodmanExitCleanly("cp", srcFile.Name(), fmt.Sprintf("%s:%sfile2", ctrName, ctrVolPath))
	})
})
