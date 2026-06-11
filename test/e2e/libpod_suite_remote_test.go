//go:build remote_testing && (linux || freebsd)

package integration

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var RemoteTestingTarget = PodmanTestCreateUtilTarget(os.Getenv("REMOTEINTEGRATION_TRANSPORT"))

func IsRemote() bool {
	return true
}

// Podman executes podman on the filesystem with default options.
func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	return p.PodmanWithOptions(PodmanExecOptions{}, args...)
}

// PodmanWithOptions executes podman on the filesystem with the supplied options.
func (p *PodmanTestIntegration) PodmanWithOptions(options PodmanExecOptions, args ...string) *PodmanSessionIntegration {
	args = p.makeOptions(args, options)
	podmanSession := p.PodmanExecBaseWithOptions(args, options)
	return &PodmanSessionIntegration{podmanSession}
}

func (p *PodmanTestIntegration) setDefaultRegistriesConfigEnv() {
	defaultFile := "registries.conf"
	if UsingCacheRegistry() {
		defaultFile = "registries-cached.conf"
	}
	defaultPath := filepath.Join(INTEGRATION_ROOT, "test", defaultFile)
	os.Setenv("CONTAINERS_REGISTRIES_CONF", defaultPath)
}

func (p *PodmanTestIntegration) setRegistriesConfigEnv(b []byte) {
	outfile := filepath.Join(p.TempDir, "registries.conf")
	os.Setenv("CONTAINERS_REGISTRIES_CONF", outfile)
	err := os.WriteFile(outfile, b, 0o644)
	Expect(err).ToNot(HaveOccurred())
}

func resetRegistriesConfigEnv() {
	os.Setenv("CONTAINERS_REGISTRIES_CONF", "")
}

func (p *PodmanTestIntegration) StartRemoteService() {
	if !isRootless() {
		err := os.MkdirAll("/run/podman", 0o755)
		Expect(err).ToNot(HaveOccurred())
	}

	args := []string{}
	if _, found := os.LookupEnv("DEBUG_SERVICE"); found {
		args = append(args, "--log-level", "trace")
	}
	if p.RemoteTLSDetails != "" {
		args = append(args, "--tls-details", p.RemoteTLSDetails)
	}

	remoteSocket := p.RemoteSocket
	args = append(args, "system", "service", "--time", "0")

	if p.RemoteTLSClientCAFile != "" {
		args = append(args, "--tls-client-ca", p.RemoteTLSClientCAFile)
	}
	if p.RemoteTLSServerCertFile != "" {
		args = append(args, "--tls-cert", p.RemoteTLSServerCertFile)
	}
	if p.RemoteTLSServerKeyFile != "" {
		args = append(args, "--tls-key", p.RemoteTLSServerKeyFile)
	}

	args = append(args, remoteSocket)

	podmanOptions := getRemoteOptions(p, args)
	cacheOptions := []string{
		"--storage-opt",
		fmt.Sprintf("%s.imagestore=%s", p.PodmanTest.ImageCacheFS, p.PodmanTest.ImageCacheDir),
	}

	podmanOptions = append(cacheOptions, podmanOptions...)
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	command.Stdout = GinkgoWriter
	command.Stderr = GinkgoWriter
	GinkgoWriter.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	err := command.Start()
	Expect(err).ToNot(HaveOccurred())
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	p.RemoteCommand = command
	p.RemoteSession = command.Process
	err = p.DelayForService()
	Expect(err).ToNot(HaveOccurred())
}

func (p *PodmanTestIntegration) StopRemoteService() {
	p.stopRemoteService(syscall.SIGTERM)
}

func (p *PodmanTestIntegration) stopRemoteService(signal syscall.Signal) {
	if err := p.RemoteSession.Signal(signal); err != nil {
		GinkgoWriter.Printf("unable to clean up service %d, %v\n", p.RemoteSession.Pid, err)
	}
	if _, err := p.RemoteSession.Wait(); err != nil {
		GinkgoWriter.Printf("error on remote stop-wait %q", err)
	}
	socket := strings.Split(p.RemoteSocket, ":")[1]
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		GinkgoWriter.Printf("%v\n", err)
	}
	if p.RemoteSocketLock != "" {
		if err := os.Remove(p.RemoteSocketLock); err != nil && !errors.Is(err, os.ErrNotExist) {
			GinkgoWriter.Printf("%v\n", err)
		}
	}
}

// getRemoteOptions assembles all the podman main options
func getRemoteOptions(p *PodmanTestIntegration, args []string) []string {
	networkDir := p.NetworkConfigDir
	podmanOptions := []string{
		"--root", p.Root,
		"--runroot", p.RunRoot,
		"--runtime", p.OCIRuntime,
		"--conmon", p.ConmonBinary,
		"--network-config-dir", networkDir,
		"--cgroup-manager", p.CgroupManager,
		"--tmpdir", p.TmpDir,
		"--events-backend", "file",
	}

	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	podmanOptions = append(podmanOptions, args...)
	return podmanOptions
}

func PodmanTestCreate(tempDir string) *PodmanTestIntegration {
	pti := PodmanTestCreateUtil(tempDir, RemoteTestingTarget)
	pti.StartRemoteService()
	return pti
}

// RestoreArtifact puts the cached image into our test store
func (p *PodmanTestIntegration) RestoreArtifact(image string) error {
	tarball := imageTarPath(image)
	if _, err := os.Stat(tarball); err == nil {
		GinkgoWriter.Printf("Restoring %s...\n", image)
		args := []string{"load", "-q", "-i", tarball}
		podmanOptions := getRemoteOptions(p, args)
		command := exec.Command(p.PodmanBinary, podmanOptions...)
		GinkgoWriter.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
		if err := command.Start(); err != nil {
			return err
		}
		if err := command.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func (p *PodmanTestIntegration) DelayForService() error {
	var err error
	var conn net.Conn
	for i := 0; i < 100; i++ {
		conn, err = net.Dial(p.RemoteSocketScheme, strings.TrimPrefix(p.RemoteSocket, p.RemoteSocketScheme+"://"))
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("service socket not detected, timeout after 10 seconds: %w", err)
}
