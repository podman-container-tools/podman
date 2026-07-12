//go:build windows

package integration

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/pkg/machine/windows"
	. "go.podman.io/podman/v6/test/utils"
)

type PodmanTestIntegration struct {
	PodmanTest
	TempDir string
}

var podmanTest *PodmanTestIntegration

type PodmanSessionIntegration struct {
	*PodmanSession
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite - Windows")
}

var _ = BeforeEach(func() {
	tempDir, err := os.MkdirTemp("", "podman_test")
	Expect(err).ToNot(HaveOccurred())
	podmanTest = PodmanTestCreate(tempDir)
})

var _ = AfterEach(func() {
	if podmanTest != nil && podmanTest.TempDir != "" {
		os.RemoveAll(podmanTest.TempDir)
	}
})

func PodmanTestCreate(tempDir string) *PodmanTestIntegration {
	cwd, _ := os.Getwd()
	podmanBinary := filepath.Join(cwd, "../../bin/windows/podman.exe")
	if envBinary := os.Getenv("PODMAN_BINARY"); envBinary != "" {
		podmanBinary = envBinary
	}

	p := &PodmanTestIntegration{
		PodmanTest: PodmanTest{
			PodmanBinary: podmanBinary,
		},
		TempDir: tempDir,
	}
	p.PodmanMakeOptions = p.makeOptions
	return p
}

func skipIfNotAdmin(reason string) {
	if !windows.HasAdminRights() {
		Skip(reason)
	}
}

func (p *PodmanTestIntegration) makeOptions(args []string, _ PodmanExecOptions) []string {
	return args
}

func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanExecBaseWithOptions(args, PodmanExecOptions{})
	return &PodmanSessionIntegration{podmanSession}
}

func (p *PodmanTestIntegration) PodmanExitCleanly(args ...string) *PodmanSessionIntegration {
	GinkgoHelper()
	session := p.Podman(args)
	session.WaitWithDefaultTimeout()
	Expect(session).Should(ExitCleanly())
	return session
}
