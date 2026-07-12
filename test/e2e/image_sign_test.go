//go:build linux || freebsd

package integration

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

// Each of these tests runs with a different GNUPGHOME; gpg-agent blows up
// if these run in parallel. We use Serial, not Ordered, because tests in
// trust_test.go also rely on gpg and can't coexist with us.
var _ = Describe("Podman image sign", Serial, func() {
	var origGNUPGHOME string

	BeforeEach(func() {
		SkipIfRemote("podman-remote image sign is not supported")
		tempGNUPGHOME := filepath.Join(podmanTest.TempDir, "tmpGPG")
		err := os.Mkdir(tempGNUPGHOME, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		origGNUPGHOME = os.Getenv("GNUPGHOME")
		err = os.Setenv("GNUPGHOME", tempGNUPGHOME)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		// There's no way to run gpg without an agent, so, clean up
		// after every test. No need to check error status.
		cmd := exec.Command("gpgconf", "--kill", "gpg-agent")
		cmd.Stdout = GinkgoWriter
		cmd.Stderr = GinkgoWriter
		_ = cmd.Run()

		os.Setenv("GNUPGHOME", origGNUPGHOME)
	})

	It("podman sign image", func() {
		cmd := exec.Command("gpg", "--import", "sign/secret-key.asc")
		cmd.Stdout = GinkgoWriter
		cmd.Stderr = GinkgoWriter
		err := cmd.Run()
		Expect(err).ToNot(HaveOccurred())
		sigDir := filepath.Join(podmanTest.TempDir, "test-sign")
		err = os.MkdirAll(sigDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"image", "sign", "--directory", sigDir, "--sign-by", "foo@bar.com", "docker://library/alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		_, err = os.Stat(filepath.Join(sigDir, "library"))
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman sign --all multi-arch image", func() {
		cmd := exec.Command("gpg", "--import", "sign/secret-key.asc")
		cmd.Stdout = GinkgoWriter
		cmd.Stderr = GinkgoWriter
		err := cmd.Run()
		Expect(err).ToNot(HaveOccurred())
		sigDir := filepath.Join(podmanTest.TempDir, "test-sign-multi")
		err = os.MkdirAll(sigDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"image", "sign", "--all", "--directory", sigDir, "--sign-by", "foo@bar.com", "docker://library/alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		fInfos, err := os.ReadDir(filepath.Join(sigDir, "library"))
		Expect(err).ToNot(HaveOccurred())
		Expect(len(fInfos)).To(BeNumerically(">", 1), "len(fInfos)")
	})
})
