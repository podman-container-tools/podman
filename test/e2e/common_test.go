//go:build linux || freebsd

package integration

import (
	"bufio"
	"bytes"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/libartifact"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/inspect"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/podman/v6/utils"
	"go.podman.io/storage/pkg/ioutils"
	"go.podman.io/storage/pkg/lockfile"
	"go.podman.io/storage/pkg/reexec"
	"go.podman.io/storage/pkg/stringid"
)

var (
	//lint:ignore ST1003
	PODMAN_BINARY      string
	INTEGRATION_ROOT   string
	CGROUP_MANAGER     = "systemd"
	RESTORE_IMAGES     = []string{ALPINE, BB, NGINX_IMAGE}
	defaultWaitTimeout = 90
)

// PodmanTestIntegration struct for command line options
type PodmanTestIntegration struct {
	PodmanTest
	ConmonBinary        string
	QuadletBinary       string
	Root                string
	NetworkConfigDir    string
	OCIRuntime          string
	RunRoot             string
	StorageOptions      string
	SignaturePolicyPath string
	CgroupManager       string
	Host                HostOS
	TmpDir              string
}

var (
	GlobalTmpDir string // Single top-level tmpdir for all tests
	LockTmpDir   string
)

// PodmanSessionIntegration struct for command line session
type PodmanSessionIntegration struct {
	*PodmanSession
}

type testResult struct {
	name   string
	length float64
}

type testResultsSorted []testResult

func (a testResultsSorted) Len() int      { return len(a) }
func (a testResultsSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type testResultsSortedLength struct{ testResultsSorted }

func (a testResultsSorted) Less(i, j int) bool { return a[i].length < a[j].length }

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

// TestLibpod ginkgo master function
func TestLibpod(t *testing.T) {
	if os.Getenv("NOCACHE") == "1" {
		CACHE_IMAGES = []string{}
		RESTORE_IMAGES = []string{}
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Libpod Suite")
}

var (
	tempdir      string // Working dir for _one_ subtest
	err          error
	podmanTest   *PodmanTestIntegration
	safeIPOctets [2]uint8
	timingsFile  *os.File

	_ = BeforeEach(func() {
		tempdir, err = os.MkdirTemp(GlobalTmpDir, "subtest-")
		Expect(err).ToNot(HaveOccurred())
		podmanTempDir := filepath.Join(tempdir, "p")
		err = os.Mkdir(podmanTempDir, 0o700)
		Expect(err).ToNot(HaveOccurred())
		podmanTest = PodmanTestCreate(podmanTempDir)
		podmanTest.Setup()
		// see GetSafeIPAddress() below
		safeIPOctets[0] = uint8(GinkgoT().ParallelProcess()) + 128
		safeIPOctets[1] = 2
	})

	_ = AfterEach(func() {
		// First unset CONTAINERS_CONF before doing Cleanup() to prevent
		// invalid containers.conf files to fail the cleanup.
		os.Unsetenv("CONTAINERS_CONF")
		os.Unsetenv("CONTAINERS_CONF_OVERRIDE")
		os.Unsetenv("PODMAN_CONNECTIONS_CONF")
		podmanTest.Cleanup()
		f := CurrentSpecReport()
		processTestResult(f)
	})
)

const (
	// lockdir - do not use directly; use LockTmpDir
	lockdir = "libpodlock"
	// imageCacheDir - do not use directly use ImageCacheDir
	imageCacheDir = "imagecachedir"
)

var netnsFiles []fs.DirEntry

func getNetnsDir() string {
	if isRootless() {
		var path string
		if env, ok := os.LookupEnv("XDG_RUNTIME_DIR"); ok {
			path = env
		} else {
			path = fmt.Sprintf("/run/user/%d", os.Getuid())
		}
		return filepath.Join(path, "netns")
	}
	// root is hard coded to
	return "/run/netns"
}

var _ = SynchronizedBeforeSuite(func() []byte {
	globalTmpDir, err := os.MkdirTemp("", "podman-e2e-")
	Expect(err).ToNot(HaveOccurred())

	netnsFiles, err = os.ReadDir(getNetnsDir())
	// dir might not exists which is fine
	if !errors.Is(err, fs.ErrNotExist) {
		Expect(err).ToNot(HaveOccurred())
	}

	// make cache dir
	ImageCacheDir = filepath.Join(globalTmpDir, imageCacheDir)
	err = os.MkdirAll(ImageCacheDir, 0o700)
	Expect(err).ToNot(HaveOccurred())

	// Cache images
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
	podman := PodmanTestSetup(filepath.Join(globalTmpDir, "image-init"))

	// Pull cirros but don't put it into the cache
	pullImages := []string{CIRROS_IMAGE, volumeTest}
	pullImages = append(pullImages, CACHE_IMAGES...)
	for _, image := range pullImages {
		podman.createArtifact(image)
	}

	if err := os.MkdirAll(filepath.Join(ImageCacheDir, podman.ImageCacheFS+"-images"), 0o777); err != nil {
		GinkgoWriter.Printf("%q\n", err)
		os.Exit(1)
	}
	podman.Root = ImageCacheDir
	// If running localized tests, the cache dir is created and populated. if the
	// tests are remote, this is a no-op
	populateCache(podman)

	if err := os.MkdirAll(filepath.Join(globalTmpDir, lockdir), 0o700); err != nil {
		GinkgoWriter.Printf("%q\n", err)
		os.Exit(1)
	}

	// If running remote, we need to stop the associated podman system service
	if podman.RemoteTest {
		podman.StopRemoteService()
	}

	// remove temporary podman files; images are now cached in ImageCacheDir
	rmAll(podman.PodmanBinary, podman.TempDir)

	return []byte(globalTmpDir)
}, func(data []byte) {
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
	GlobalTmpDir = string(data)
	ImageCacheDir = filepath.Join(GlobalTmpDir, imageCacheDir)
	LockTmpDir = filepath.Join(GlobalTmpDir, lockdir)

	timingsFile, err = os.Create(fmt.Sprintf("%s/timings-%d", LockTmpDir, GinkgoParallelProcess()))
	Expect(err).ToNot(HaveOccurred())
})

func (p *PodmanTestIntegration) Setup() {
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
}

var _ = SynchronizedAfterSuite(func() {
	timingsFile.Close()
	timingsFile = nil
},
	func() {
		// perform a netns leak check after all tests run
		newNetnsFiles, err := os.ReadDir(getNetnsDir())
		if !errors.Is(err, fs.ErrNotExist) {
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(newNetnsFiles).To(ConsistOf(netnsFiles), "Netns files were leaked")

		testTimings := make(testResultsSorted, 0, 2000)
		for i := 1; i <= GinkgoT().ParallelTotal(); i++ {
			f, err := os.Open(fmt.Sprintf("%s/timings-%d", LockTmpDir, i))
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				text := scanner.Text()
				name, durationString, ok := strings.Cut(text, "\t\t")
				if !ok {
					Fail(fmt.Sprintf("incorrect timing line: %q", text))
				}
				duration, err := strconv.ParseFloat(durationString, 64)
				Expect(err).ToNot(HaveOccurred(), "failed to parse float from timings file")
				testTimings = append(testTimings, testResult{name: name, length: duration})
			}
			if err := scanner.Err(); err != nil {
				Expect(err).ToNot(HaveOccurred(), "read timings %d", i)
			}
		}
		sort.Sort(testResultsSortedLength{testTimings})
		GinkgoWriter.Println("integration timing results")
		for _, result := range testTimings {
			GinkgoWriter.Printf("%s\t\t%f\n", result.name, result.length)
		}

		cwd, _ := os.Getwd()
		rmAll(getPodmanBinary(cwd), GlobalTmpDir)
	})

func getPodmanBinary(cwd string) string {
	podmanBinary := filepath.Join(cwd, "../../bin/podman")
	if os.Getenv("PODMAN_BINARY") != "" {
		podmanBinary = os.Getenv("PODMAN_BINARY")
	}
	return podmanBinary
}

type PodmanTestCreateUtilTarget string

const (
	PodmanTestCreateUtilTargetLocal = ""
	PodmanTestCreateUtilTargetUnix  = "unix"
	PodmanTestCreateUtilTargetTCP   = "tcp"
	PodmanTestCreateUtilTargetTLS   = "tls"
	PodmanTestCreateUtilTargetMTLS  = "mtls"
)

// PodmanTestCreateUtil creates a PodmanTestIntegration instance for the tests
func PodmanTestCreateUtil(tempDir string, target PodmanTestCreateUtilTarget) *PodmanTestIntegration {
	host := GetHostDistributionInfo()
	cwd, _ := os.Getwd()

	root := filepath.Join(tempDir, "root")
	podmanBinary := getPodmanBinary(cwd)

	podmanRemoteBinary := os.Getenv("PODMAN_REMOTE_BINARY")
	if podmanRemoteBinary == "" {
		podmanRemoteBinary = filepath.Join(cwd, "../../bin/podman-remote")
	}

	quadletBinary := os.Getenv("QUADLET_BINARY")
	if quadletBinary == "" {
		quadletBinary = filepath.Join(cwd, "../../bin/quadlet")
	}

	conmonBinary := os.Getenv("CONMON_BINARY")
	if conmonBinary == "" {
		conmonBinary = "/usr/libexec/podman/conmon"
		if _, err := os.Stat(conmonBinary); errors.Is(err, os.ErrNotExist) {
			conmonBinary = "/usr/bin/conmon"
		}
	}

	cgroupManager := os.Getenv("CGROUP_MANAGER")
	if cgroupManager == "" {
		cgroupManager = CGROUP_MANAGER
	}

	ociRuntime := os.Getenv("OCI_RUNTIME")
	if ociRuntime == "" {
		ociRuntime = "crun"
	}
	os.Setenv("DISABLE_HC_SYSTEMD", "true")

	dbBackend := "sqlite"

	networkConfigDir := "/etc/containers/networks"
	if isRootless() {
		networkConfigDir = filepath.Join(root, "etc", "networks")
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		panic(err)
	}

	if err := os.MkdirAll(networkConfigDir, 0o755); err != nil {
		panic(err)
	}

	storageFs := STORAGE_FS
	if isRootless() {
		storageFs = ROOTLESS_STORAGE_FS
	}

	storageOptions := STORAGE_OPTIONS
	if os.Getenv("STORAGE_FS") != "" {
		storageFs = os.Getenv("STORAGE_FS")
		storageOptions = "--storage-driver " + storageFs

		// Look for STORAGE_OPTIONS_OVERLAY / STORAGE_OPTIONS_VFS
		extraOptions := os.Getenv("STORAGE_OPTIONS_" + strings.ToUpper(storageFs))
		if extraOptions != "" {
			storageOptions += " --storage-opt " + extraOptions
		}
	}

	p := &PodmanTestIntegration{
		PodmanTest: PodmanTest{
			PodmanBinary:       podmanBinary,
			RemotePodmanBinary: podmanRemoteBinary,
			TempDir:            tempDir,
			RemoteTest:         target != PodmanTestCreateUtilTargetLocal,
			ImageCacheFS:       storageFs,
			ImageCacheDir:      ImageCacheDir,
			DatabaseBackend:    dbBackend,
		},
		ConmonBinary:        conmonBinary,
		QuadletBinary:       quadletBinary,
		Root:                root,
		TmpDir:              tempDir,
		NetworkConfigDir:    networkConfigDir,
		OCIRuntime:          ociRuntime,
		RunRoot:             filepath.Join(tempDir, "runroot"),
		StorageOptions:      storageOptions,
		SignaturePolicyPath: filepath.Join(INTEGRATION_ROOT, "test/policy.json"),
		CgroupManager:       cgroupManager,
		Host:                host,
	}

	var pathPrefix string
	switch target {
	case PodmanTestCreateUtilTargetLocal:
	default:
		if !isRootless() {
			pathPrefix = "/run/podman/podman"
			Expect(os.MkdirAll(pathPrefix, 0o700)).To(Succeed())
		} else {
			runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
			pathPrefix = filepath.Join(runtimeDir, "podman")
		}
	}
	switch target {
	case PodmanTestCreateUtilTargetUnix:
		// We want to avoid collisions in socket paths, but using the
		// socket directly for a collision check doesn’t work; bind(2) on AF_UNIX
		// creates the file, and we need to pass a unique path now before the bind(2)
		// happens. So, use a podman-%s.sock-lock empty file as a marker.
		tries := 0
		for {
			uuid := stringid.GenerateRandomID()
			lockPath := fmt.Sprintf("%s-%s.sock-lock", pathPrefix, uuid)
			lockFile, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
			if err == nil {
				lockFile.Close()
				p.RemoteSocketLock = lockPath
				p.RemoteSocket = fmt.Sprintf("unix://%s-%s.sock", pathPrefix, uuid)
				p.RemoteSocketScheme = "unix"
				break
			}
			GinkgoLogr.Error(err, "RemoteSocket collision")
			tries++
			if tries >= 1000 {
				panic("Too many RemoteSocket collisions")
			}
		}
	case PodmanTestCreateUtilTargetTCP, PodmanTestCreateUtilTargetTLS, PodmanTestCreateUtilTargetMTLS:
		tries := 0
		for {
			uuid := stringid.GenerateRandomID()
			lockPath := fmt.Sprintf("%s-%s.sock-lock", pathPrefix, uuid)
			lockFile, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
			if err == nil {
				lockFile.Close()
				p.RemoteSocketLock = lockPath
				lis, err := net.Listen("tcp", "127.0.0.1:0")
				if err == nil {
					defer lis.Close()
					p.RemoteSocket = fmt.Sprintf("tcp://%s", lis.Addr())
					p.RemoteSocketScheme = "tcp"
					break
				}
			}
			GinkgoLogr.Error(err, "RemoteSocket collision")
			tries++
			if tries >= 1000 {
				panic("Too many RemoteSocket collisions")
			}
		}
	}

	caKeyPath := filepath.Join(p.TempDir, "tls.ca.key")
	caCertPath := filepath.Join(p.TempDir, "tls.ca.crt")
	srvCertPath := filepath.Join(p.TempDir, "tls.srv.crt")
	srvKeyPath := filepath.Join(p.TempDir, "tls.srv.key")
	clientCertPath := filepath.Join(p.TempDir, "tls.client.crt")
	clientKeyPath := filepath.Join(p.TempDir, "tls.client.key")
	switch target {
	case PodmanTestCreateUtilTargetTLS, PodmanTestCreateUtilTargetMTLS:
		GinkgoLogr.Info("Generating test TLS certs", "now", time.Now(), "tmpdir", p.TempDir)
		now := time.Now()
		caPriv, err := rsa.GenerateKey(crand.Reader, 2048)
		Expect(err).ToNot(HaveOccurred())
		caTmpl := x509.Certificate{
			NotBefore:             now,
			NotAfter:              now.Add(5 * time.Minute),
			IsCA:                  true,
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			BasicConstraintsValid: true,

			DNSNames:    []string{"localhost"},
			IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		}
		caCertDER, err := x509.CreateCertificate(crand.Reader, &caTmpl, &caTmpl, &caPriv.PublicKey, caPriv)
		Expect(err).ToNot(HaveOccurred())
		caCertPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caCertDER,
		})
		caKeyDER, err := x509.MarshalPKCS8PrivateKey(caPriv)
		Expect(err).ToNot(HaveOccurred())
		caKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: caKeyDER,
		})
		err = os.WriteFile(caCertPath, caCertPEM, 0o600)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(caKeyPath, caKeyPEM, 0o600)
		Expect(err).ToNot(HaveOccurred())

		caCert, err := x509.ParseCertificate(caCertDER)
		Expect(err).ToNot(HaveOccurred())

		srvPriv, err := rsa.GenerateKey(crand.Reader, 2048)
		Expect(err).ToNot(HaveOccurred())
		srvTmpl := x509.Certificate{
			NotBefore:             now,
			NotAfter:              now.Add(5 * time.Minute),
			KeyUsage:              x509.KeyUsageDigitalSignature,
			BasicConstraintsValid: true,
			DNSNames:              []string{"localhost"},
			IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		}
		srvCertDER, err := x509.CreateCertificate(crand.Reader, &srvTmpl, caCert, &srvPriv.PublicKey, caPriv)
		Expect(err).ToNot(HaveOccurred())
		srvCertPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: srvCertDER,
		})
		srvKeyDER, err := x509.MarshalPKCS8PrivateKey(srvPriv)
		Expect(err).ToNot(HaveOccurred())
		srvKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: srvKeyDER,
		})
		err = os.WriteFile(srvCertPath, srvCertPEM, 0o600)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(srvKeyPath, srvKeyPEM, 0o600)
		Expect(err).ToNot(HaveOccurred())

		p.RemoteTLSServerCAFile = caCertPath
		p.RemoteTLSServerCAPool = x509.NewCertPool()
		p.RemoteTLSServerCAPool.AddCert(caCert)
		p.RemoteTLSServerCertFile = srvCertPath
		p.RemoteTLSServerKeyFile = srvKeyPath
		if target == PodmanTestCreateUtilTargetMTLS {
			clientPriv, err := rsa.GenerateKey(crand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())
			clientTmpl := x509.Certificate{
				NotBefore:             now,
				NotAfter:              now.Add(5 * time.Minute),
				KeyUsage:              x509.KeyUsageDigitalSignature,
				BasicConstraintsValid: true,
			}
			clientCertDER, err := x509.CreateCertificate(crand.Reader, &clientTmpl, caCert, &clientPriv.PublicKey, caPriv)
			Expect(err).ToNot(HaveOccurred())
			clientCertPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: clientCertDER,
			})
			clientKeyDER, err := x509.MarshalPKCS8PrivateKey(clientPriv)
			Expect(err).ToNot(HaveOccurred())
			clientCert, err := x509.ParseCertificate(clientCertDER)
			Expect(err).ToNot(HaveOccurred())
			clientKeyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: clientKeyDER,
			})
			err = os.WriteFile(clientCertPath, clientCertPEM, 0o600)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(clientKeyPath, clientKeyPEM, 0o600)
			Expect(err).ToNot(HaveOccurred())

			p.RemoteTLSClientCAFile = caCertPath
			p.RemoteTLSServerCAPool = x509.NewCertPool()
			p.RemoteTLSServerCAPool.AddCert(caCert)
			p.RemoteTLSClientCertFile = clientCertPath
			p.RemoteTLSClientKeyFile = clientKeyPath
			p.RemoteTLSClientCerts = []tls.Certificate{{
				Certificate: [][]byte{clientCertDER},
				PrivateKey:  clientPriv,
				Leaf:        clientCert,
			}}
		}
	}

	// Set up registries.conf ENV variable
	p.setDefaultRegistriesConfigEnv()
	// Rewrite the PodmanAsUser function
	p.PodmanMakeOptions = p.makeOptions
	return p
}

func (p *PodmanTestIntegration) AddImageToRWStore(image string) {
	if err := p.RestoreArtifact(image); err != nil {
		logrus.Errorf("Unable to restore %s to RW store", image)
	}
}

func imageTarPath(image string) string {
	cacheDir := os.Getenv("PODMAN_TEST_IMAGE_CACHE_DIR")
	if cacheDir == "" {
		// Avoid /tmp: it may be tmpfs, and these images are large
		cacheDir = "/var/tmp"
	}

	// e.g., registry.com/fubar:latest -> registry.com-fubar-latest.tar
	imageCacheName := strings.ReplaceAll(strings.ReplaceAll(image, ":", "-"), "/", "-") + ".tar"

	return filepath.Join(cacheDir, imageCacheName)
}

func (p *PodmanTestIntegration) pullImage(image string, toCache bool) {
	if toCache {
		oldRoot := p.Root
		p.Root = p.ImageCacheDir
		defer func() {
			p.Root = oldRoot
		}()
	}
	for try := range 3 {
		podmanSession := p.PodmanExecBaseWithOptions([]string{"pull", image}, PodmanExecOptions{
			NoEvents: toCache,
			NoCache:  true,
		})
		pull := PodmanSessionIntegration{podmanSession}
		pull.Wait(440)
		if pull.ExitCode() == 0 {
			break
		}
		if try == 2 {
			Expect(pull).Should(Exit(0), "Failed after many retries")
		}

		GinkgoWriter.Println("Will wait and retry")
		time.Sleep(time.Duration(try+1) * 5 * time.Second)
	}
}

// createArtifact creates a cached image tarball in a local directory
func (p *PodmanTestIntegration) createArtifact(image string) {
	if os.Getenv("NO_TEST_CACHE") != "" {
		return
	}
	destName := imageTarPath(image)
	if _, err := os.Stat(destName); errors.Is(err, os.ErrNotExist) {
		GinkgoWriter.Printf("Caching %s at %s...\n", image, destName)

		p.pullImage(image, false)

		save := p.PodmanNoCache([]string{"save", "-o", destName, image})
		save.Wait(90)
		Expect(save).Should(Exit(0))
		GinkgoWriter.Printf("\n")
	} else {
		GinkgoWriter.Printf("[image already cached: %s]\n", destName)
	}
}

// InspectImageJSON takes the session output of an inspect
// image and returns json
func (s *PodmanSessionIntegration) InspectImageJSON() []inspect.ImageData {
	var i []inspect.ImageData
	err := jsoniter.Unmarshal(s.Out.Contents(), &i)
	Expect(err).ToNot(HaveOccurred())
	return i
}

// InspectArtifactToJSON takes the session output of an artifact inspect and returns json
func (s *PodmanSessionIntegration) InspectArtifactToJSON() libartifact.Artifact {
	a := libartifact.Artifact{}
	inspectOut := s.OutputToString()
	err := json.Unmarshal([]byte(inspectOut), &a)
	Expect(err).ToNot(HaveOccurred())
	return a
}

// PodmanExitCleanly runs a podman command with args, and expects it to ExitCleanly within the default timeout.
// It returns the session (to allow consuming output if desired).
func (p *PodmanTestIntegration) PodmanExitCleanly(args ...string) *PodmanSessionIntegration {
	GinkgoHelper()
	return p.PodmanExitCleanlyWithOptions(PodmanExecOptions{}, args...)
}

// PodmanExitCleanlyWithOptions runs a podman command with (optinos, args), and expects it to ExitCleanly within the default timeout.
// It returns the session (to allow consuming output if desired).
func (p *PodmanTestIntegration) PodmanExitCleanlyWithOptions(options PodmanExecOptions, args ...string) *PodmanSessionIntegration {
	GinkgoHelper()
	session := p.PodmanWithOptions(options, args...)
	session.WaitWithDefaultTimeout()
	Expect(session).Should(ExitCleanly())
	return session
}

// InspectContainer returns a container's inspect data in JSON format
func (p *PodmanTestIntegration) InspectContainer(name string) []define.InspectContainerData {
	cmd := []string{"inspect", name}
	session := p.Podman(cmd)
	session.WaitWithDefaultTimeout()
	Expect(session).Should(Exit(0))
	return session.InspectContainerToJSON()
}

// InspectArtifact returns an artifact's inspect data in JSON format
func (p *PodmanTestIntegration) InspectArtifact(name string) libartifact.Artifact {
	cmd := []string{"artifact", "inspect", name}
	session := p.Podman(cmd)
	session.WaitWithDefaultTimeout()
	Expect(session).Should(Exit(0))
	return session.InspectArtifactToJSON()
}

// InspectNetwork returns a network's inspect data in JSON format
func (p *PodmanTestIntegration) InspectNetwork(name string) []entities.NetworkInspectReport {
	cmd := []string{"network", "inspect", name}
	session := p.Podman(cmd)
	session.WaitWithDefaultTimeout()
	Expect(session).Should(Exit(0))
	return session.InspectNetworkToJSON()
}

// Pull a single field from a container using `podman inspect --format {{ field }}`,
// and verify it against the given expected value.
func (p *PodmanTestIntegration) CheckContainerSingleField(name, field, expected string) {
	inspect := p.Podman([]string{"inspect", "--format", fmt.Sprintf("{{ %s }}", field), name})
	inspect.WaitWithDefaultTimeout()
	ExpectWithOffset(1, inspect).Should(Exit(0))
	ExpectWithOffset(1, inspect.OutputToString()).To(Equal(expected))
}

// Check that the contents of a single file in the given container matches the expected value.
func (p *PodmanTestIntegration) CheckFileInContainer(name, filepath, expected string) {
	exec := p.Podman([]string{"exec", name, "cat", filepath})
	exec.WaitWithDefaultTimeout()
	ExpectWithOffset(1, exec).Should(Exit(0))
	ExpectWithOffset(1, exec.OutputToString()).To(Equal(expected))
}

// Check that the contents of a single file in the given container containers the given value.
func (p *PodmanTestIntegration) CheckFileInContainerSubstring(name, filepath, expected string) {
	exec := p.Podman([]string{"exec", name, "cat", filepath})
	exec.WaitWithDefaultTimeout()
	ExpectWithOffset(1, exec).Should(Exit(0))
	ExpectWithOffset(1, exec.OutputToString()).To(ContainSubstring(expected))
}

// StopContainer stops a container with no timeout, ensuring a fast test.
func (p *PodmanTestIntegration) StopContainer(nameOrID string) {
	p.PodmanExitCleanly("stop", "-t0", nameOrID)
}

func (p *PodmanTestIntegration) StopPod(nameOrID string) {
	p.PodmanExitCleanly("pod", "stop", "-t0", nameOrID)
}

func processTestResult(r SpecReport) {
	tr := testResult{length: r.RunTime.Seconds(), name: r.FullText()}
	_, err := fmt.Fprintf(timingsFile, "%s\t\t%f\n", tr.name, tr.length)
	Expect(err).ToNot(HaveOccurred(), "write timings")
}

func GetPortLock(port string) *lockfile.LockFile {
	lockFile := filepath.Join(LockTmpDir, port)
	lock, err := lockfile.GetLockFile(lockFile)
	if err != nil {
		GinkgoWriter.Println(err)
		os.Exit(1)
	}
	lock.Lock()
	return lock
}

// GetSafeIPAddress returns a sequentially allocated IP address that _should_
// be safe and unique across parallel tasks
//
// Used by tests which want to use "--ip SOMETHING-SAFE". Picking at random
// just doesn't work: we get occasional collisions. Our current approach
// allocates a /24 subnet for each ginkgo process, starting at .128.x, see
// BeforeEach() above. Netavark allocates sequentially from .0.0, hence
// our .128.x starting point leaves room for parallel test execution.
// The +10 leaves a gap for other parallel jobs that may get IPs in our block.
// This heuristic will fail if run in parallel on >127 processors or if
// one test calls us more than 25 times or if some other test runs more
// than ten networked containers at the same time as any test that
// relies on GetSafeIPAddress(). I'm finding it hard to care.
//
// DO NOT USE THIS FUNCTION unless there is no possible alternative. In
// most cases you should use 'podman network create' + 'podman run --network'.
func GetSafeIPAddress() string {
	safeIPOctets[1] += 10
	return fmt.Sprintf("10.88.%d.%d", safeIPOctets[0], safeIPOctets[1])
}

// RunTopContainer runs a simple container in the background that
// runs top.  If the name passed != "", it will have a name
func (p *PodmanTestIntegration) RunTopContainer(name string) *PodmanSessionIntegration {
	return p.RunTopContainerWithArgs(name, nil)
}

// RunTopContainerWithArgs runs a simple container in the background that
// runs top.  If the name passed != "", it will have a name, command args can also be passed in
func (p *PodmanTestIntegration) RunTopContainerWithArgs(name string, args []string) *PodmanSessionIntegration {
	// In proxy environment, some tests need to the --http-proxy=false option (#16684)
	podmanArgs := []string{"run", "--http-proxy=false"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, args...)
	podmanArgs = append(podmanArgs, "-d", ALPINE, "top", "-b")
	session := p.PodmanExitCleanly(podmanArgs...)
	cid := session.OutputToString()
	// Output indicates that top is running, which means it's safe
	// for our caller to invoke `podman stop`
	if !WaitContainerReady(p, cid, "Mem:", 20, 1) {
		Fail("Could not start a top container")
	}
	return session
}

// RunLsContainer runs a simple container in the background that
// simply runs ls. If the name passed != "", it will have a name
func (p *PodmanTestIntegration) RunLsContainer(name string) (*PodmanSessionIntegration, int, string) {
	podmanArgs := []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "ls")
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	if session.ExitCode() != 0 {
		return session, session.ExitCode(), session.OutputToString()
	}
	cid := session.OutputToString()

	wsession := p.Podman([]string{"wait", cid})
	wsession.WaitWithDefaultTimeout()
	return session, wsession.ExitCode(), cid
}

// RunNginxWithHealthCheck runs the alpine nginx container with an optional name and adds a healthcheck into it
func (p *PodmanTestIntegration) RunNginxWithHealthCheck(name string) (*PodmanSessionIntegration, string) {
	podmanArgs := []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	// curl without -f exits 0 even if http code >= 400!
	podmanArgs = append(podmanArgs, "-dt", "-P", "--health-cmd", "curl -f http://localhost/", NGINX_IMAGE)
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.OutputToString()
}

// RunContainerWithNetworkTest runs curl in FEDORA_MINIMAL with the specified network mode.
func (p *PodmanTestIntegration) RunContainerWithNetworkTest(mode string) *PodmanSessionIntegration {
	podmanArgs := []string{"run"}
	if mode != "" {
		podmanArgs = append(podmanArgs, "--network", mode)
	}
	podmanArgs = append(podmanArgs, FEDORA_MINIMAL, "curl", "-s", "-S", "-k", "-o", "/dev/null", "http://www.redhat.com:80")
	session := p.Podman(podmanArgs)
	return session
}

func (p *PodmanTestIntegration) RunLsContainerInPod(name, pod string) (*PodmanSessionIntegration, int, string) {
	podmanArgs := []string{"run", "--pod", pod}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "ls")
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	if session.ExitCode() != 0 {
		return session, session.ExitCode(), session.OutputToString()
	}
	cid := session.OutputToString()

	wsession := p.Podman([]string{"wait", cid})
	wsession.WaitWithDefaultTimeout()
	return session, wsession.ExitCode(), cid
}

// BuildImage uses podman build and buildah to build an image
// called imageName based on a string dockerfile
func (p *PodmanTestIntegration) BuildImage(dockerfile, imageName string, layers string, extraOptions ...string) string {
	return p.buildImage(dockerfile, imageName, layers, "", extraOptions)
}

// BuildImageWithLabel uses podman build and buildah to build an image
// called imageName based on a string dockerfile, adds desired label to paramset
func (p *PodmanTestIntegration) BuildImageWithLabel(dockerfile, imageName string, layers string, label string, extraOptions ...string) string {
	return p.buildImage(dockerfile, imageName, layers, label, extraOptions)
}

// PodmanPID execs podman and returns its PID
func (p *PodmanTestIntegration) PodmanPID(args []string) (*PodmanSessionIntegration, int) {
	podmanOptions := p.MakeOptions(args, PodmanExecOptions{})
	GinkgoWriter.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))

	command := exec.Command(p.PodmanBinary, podmanOptions...)
	session, err := Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail("unable to run podman command: " + strings.Join(podmanOptions, " "))
	}
	podmanSession := &PodmanSession{Session: session}
	return &PodmanSessionIntegration{podmanSession}, command.Process.Pid
}

func (p *PodmanTestIntegration) Quadlet(args []string, sourceDir string) *PodmanSessionIntegration {
	GinkgoWriter.Printf("Running: %s %s with QUADLET_UNIT_DIRS=%s\n", p.QuadletBinary, strings.Join(args, " "), sourceDir)

	// quadlet uses PODMAN env to get a stable podman path
	podmanPath, found := os.LookupEnv("PODMAN")
	if !found {
		podmanPath = p.PodmanBinary
	}

	command := exec.Command(p.QuadletBinary, args...)
	command.Env = []string{
		fmt.Sprintf("QUADLET_UNIT_DIRS=%s", sourceDir),
		fmt.Sprintf("PODMAN=%s", podmanPath),
	}
	session, err := Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail("unable to run quadlet command: " + strings.Join(args, " "))
	}
	quadletSession := &PodmanSession{Session: session}
	return &PodmanSessionIntegration{quadletSession}
}

// Cleanup cleans up the temporary store
func (p *PodmanTestIntegration) Cleanup() {
	// ginkgo v2 still goes into AfterEach() when Skip() was called,
	// some tests call skip before the podman test is initialized.
	if p == nil {
		return
	}

	// first stop everything, rm -fa is unreliable
	// https://github.com/containers/podman/issues/18180
	stop := p.Podman([]string{"stop", "--all", "-t", "0"})
	stop.WaitWithDefaultTimeout()

	// Remove all pods...
	podrm := p.Podman([]string{"pod", "rm", "-fa", "-t", "0"})
	podrm.WaitWithDefaultTimeout()

	// ...and containers
	rmall := p.Podman([]string{"rm", "-fa", "-t", "0"})
	rmall.WaitWithDefaultTimeout()

	p.StopRemoteService()
	// Nuke tempdir
	rmAll(p.PodmanBinary, p.TempDir)

	// Clean up the registries configuration file ENV variable set in Create
	resetRegistriesConfigEnv()

	// Make sure to only check exit codes after all cleanup is done.
	// An error would cause it to stop and return early otherwise.
	Expect(stop).To(Exit(0), "command: %v\nstdout: %s\nstderr: %s", stop.Command.Args, stop.OutputToString(), stop.ErrorToString())
	checkStderrCleanupError(stop, "stop --all -t0 error logged")
	Expect(podrm).To(Exit(0), "command: %v\nstdout: %s\nstderr: %s", podrm.Command.Args, podrm.OutputToString(), podrm.ErrorToString())
	checkStderrCleanupError(podrm, "pod rm -fa -t0 error logged")
	Expect(rmall).To(Exit(0), "command: %v\nstdout: %s\nstderr: %s", rmall.Command.Args, rmall.OutputToString(), rmall.ErrorToString())
	checkStderrCleanupError(rmall, "rm -fa -t0 error logged")
}

func checkStderrCleanupError(s *PodmanSessionIntegration, cmd string) {
	if strings.Contains(podmanTest.OCIRuntime, "runc") {
		// we cannot check stderr for runc, way to many errors
		return
	}
	// offset is 1 so the stacj trace doesn't link to this helper function here
	ExpectWithOffset(1, s.ErrorToString()).To(BeEmpty(), cmd)
}

// CleanupVolume cleans up the volumes and containers.
// This already calls Cleanup() internally.
func (p *PodmanTestIntegration) CleanupVolume() {
	// Remove all containers
	session := p.Podman([]string{"volume", "rm", "-fa"})
	session.WaitWithDefaultTimeout()
	Expect(session).To(Exit(0), "command: %v\nstdout: %s\nstderr: %s", session.Command.Args, session.OutputToString(), session.ErrorToString())
	checkStderrCleanupError(session, "volume rm -fa error logged")
}

// CleanupSecrets cleans up the secrets and containers.
// This already calls Cleanup() internally.
func (p *PodmanTestIntegration) CleanupSecrets() {
	// Remove all containers
	session := p.Podman([]string{"secret", "rm", "-a"})
	session.Wait(90)
	Expect(session).To(Exit(0), "command: %v\nstdout: %s\nstderr: %s", session.Command.Args, session.OutputToString(), session.ErrorToString())
	checkStderrCleanupError(session, "secret rm -a error logged")
}

// InspectContainerToJSON takes the session output of an inspect
// container and returns json
func (s *PodmanSessionIntegration) InspectContainerToJSON() []define.InspectContainerData {
	var i []define.InspectContainerData
	err := jsoniter.Unmarshal(s.Out.Contents(), &i)
	Expect(err).ToNot(HaveOccurred())
	return i
}

func (s *PodmanSessionIntegration) InspectNetworkToJSON() []entities.NetworkInspectReport {
	var out []entities.NetworkInspectReport
	err := json.Unmarshal(s.Out.Contents(), &out)
	Expect(err).ToNot(HaveOccurred())
	return out
}

// InspectPodToJSON takes the sessions output from a pod inspect and returns json
func (s *PodmanSessionIntegration) InspectPodToJSON() define.InspectPodData {
	var i []define.InspectPodData
	err := jsoniter.Unmarshal(s.Out.Contents(), &i)
	Expect(err).ToNot(HaveOccurred())
	Expect(i).To(HaveLen(1))
	return i[0]
}

// InspectPodToJSON takes the sessions output from an inspect and returns json
func (s *PodmanSessionIntegration) InspectPodArrToJSON() []define.InspectPodData {
	var i []define.InspectPodData
	err := jsoniter.Unmarshal(s.Out.Contents(), &i)
	Expect(err).ToNot(HaveOccurred())
	return i
}

// CreatePod creates a pod with no infra container
// it optionally takes a pod name
func (p *PodmanTestIntegration) CreatePod(options map[string][]string) (*PodmanSessionIntegration, int, string) {
	args := []string{"pod", "create", "--infra=false", "--share", ""}
	for k, values := range options {
		for _, v := range values {
			args = append(args, k+"="+v)
		}
	}

	session := p.Podman(args)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

func (p *PodmanTestIntegration) CreateVolume(options map[string][]string) (*PodmanSessionIntegration, int, string) {
	args := []string{"volume", "create"}
	for k, values := range options {
		for _, v := range values {
			args = append(args, k+"="+v)
		}
	}

	session := p.Podman(args)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

func (p *PodmanTestIntegration) RunTopContainerInPod(name, pod string) *PodmanSessionIntegration {
	return p.RunTopContainerWithArgs(name, []string{"--pod", pod})
}

func (p *PodmanTestIntegration) RunHealthCheck(cid string) error {
	for range 10 {
		hc := p.Podman([]string{"healthcheck", "run", cid})
		hc.WaitWithDefaultTimeout()
		if hc.ExitCode() == 0 {
			return nil
		}
		// Restart container if it's not running
		ps := p.Podman([]string{"ps", "--no-trunc", "--quiet", "--filter", fmt.Sprintf("id=%s", cid)})
		ps.WaitWithDefaultTimeout()
		if ps.ExitCode() == 0 {
			if !strings.Contains(ps.OutputToString(), cid) {
				GinkgoWriter.Printf("Container %s is not running, restarting", cid)
				restart := p.Podman([]string{"restart", cid})
				restart.WaitWithDefaultTimeout()
				if restart.ExitCode() != 0 {
					return fmt.Errorf("unable to restart %s", cid)
				}
			}
		}
		GinkgoWriter.Printf("Waiting for %s to pass healthcheck\n", cid)
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("unable to detect %s as running", cid)
}

func (p *PodmanTestIntegration) CreateSeccompJSON(in []byte) (string, error) {
	jsonFile := filepath.Join(p.TempDir, "seccomp.json")
	err := WriteJSONFile(in, jsonFile)
	if err != nil {
		return "", err
	}
	return jsonFile, nil
}

func checkReason(reason string) {
	if len(reason) < 5 {
		panic("Test must specify a reason to skip")
	}
}

func SkipIfRunc(p *PodmanTestIntegration, reason string) {
	checkReason(reason)
	if p.OCIRuntime == "runc" {
		Skip("[runc]: " + reason)
	}
}

func SkipIfRootless(reason string) {
	checkReason(reason)
	if isRootless() {
		Skip("[rootless]: " + reason)
	}
}

func SkipIfNotRootless(reason string) {
	checkReason(reason)
	if !isRootless() {
		Skip("[notRootless]: " + reason)
	}
}

func SkipIfNotExist(reason, path string) {
	checkReason(reason)
	if _, err := os.Stat(path); err != nil {
		Skip("[doesNotExist]: " + path + " does not exist: " + reason)
	}
}

func SkipIfSystemdNotRunning(reason string) {
	checkReason(reason)

	cmd := exec.Command("systemctl", "list-units")
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.Error); ok {
			Skip("[notSystemd]: not running " + reason)
		}
		Expect(err).ToNot(HaveOccurred())
	}
}

func SkipIfNotSystemd(manager, reason string) {
	checkReason(reason)
	if manager != "systemd" {
		Skip("[notSystemd]: " + reason)
	}
}

func SkipOnOSVersion(os, version string, reason string) {
	info := GetHostDistributionInfo()
	if info.Distribution == os && info.Version == version {
		Skip(fmt.Sprintf("[%s %s]: %s", os, version, reason))
	}
}

func SkipIfNotFedora(reason string) {
	info := GetHostDistributionInfo()
	if info.Distribution != "fedora" {
		Skip(reason)
	}
}

type journaldTests struct {
	journaldSkip bool
	journaldOnce sync.Once
}

var journald journaldTests

// Check if journalctl is unavailable
func checkAvailableJournald() {
	f := func() {
		journald.journaldSkip = false

		cmd := exec.Command("journalctl", "-n", "1")
		if err := cmd.Run(); err != nil {
			journald.journaldSkip = true
		}
	}
	journald.journaldOnce.Do(f)
}

func SkipIfJournaldUnavailable() {
	checkAvailableJournald()

	// In container, journalctl does not return an error even if
	// journald is unavailable
	SkipIfInContainer("[journald]: journalctl inside a container doesn't work correctly")
	if journald.journaldSkip {
		Skip("[journald]: journald is unavailable")
	}
}

// Use isRootless() instead of rootless.IsRootless()
// This function can detect to join the user namespace by mistake
func isRootless() bool {
	return os.Geteuid() != 0
}

func isContainerized() bool {
	// This is set to "podman" by podman automatically
	return os.Getenv("container") != ""
}

func SkipIfContainerized(reason string) {
	checkReason(reason)
	if isContainerized() {
		Skip(reason)
	}
}

func SkipIfRemote(reason string) {
	checkReason(reason)
	if !IsRemote() {
		return
	}
	Skip("[remote]: " + reason)
}

func SkipIfNotRemote(reason string) {
	checkReason(reason)
	if IsRemote() {
		return
	}
	Skip("[local]: " + reason)
}

// SkipIfInContainer skips a test if the test is run inside a container
func SkipIfInContainer(reason string) {
	checkReason(reason)
	if os.Getenv("TEST_ENVIRON") == "container" {
		Skip("[container]: " + reason)
	}
}

// SkipIfConmonVersionLessThan skips a test if the conmon version is less than
// the specified minimum version (e.g., "2.2.0").
func SkipIfConmonVersionLessThan(minVersion string) {
	out, err := exec.Command(podmanTest.ConmonBinary, "--version").Output()
	if err != nil {
		Fail(fmt.Sprintf("[conmon]: failed to get conmon version: %v", err))
	}
	// Output format: "conmon version 2.2.0"
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 3 {
		Fail(fmt.Sprintf("[conmon]: unexpected conmon --version output: %s", out))
	}
	current, err := semver.Parse(fields[2])
	if err != nil {
		Fail(fmt.Sprintf("[conmon]: failed to parse conmon version %q: %v", fields[2], err))
	}
	minVer, err := semver.Parse(minVersion)
	if err != nil {
		Fail(fmt.Sprintf("[conmon]: failed to parse minimum version %q: %v", minVersion, err))
	}
	if current.Compare(minVer) < 0 {
		Skip(fmt.Sprintf("[conmon]: need conmon >= %s; have %s", minVersion, fields[2]))
	}
}

// SkipIfNetavarkVersionLessThan skips a test if the current network backend is
// netavark and its version is less than the specified minimum version (e.g., "1.17.2").
// The function is a no-op when netavark is not the active network backend.
func SkipIfNetavarkVersionLessThan(minVersion string) {
	session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.NetworkBackendInfo.Backend}}=={{.Host.NetworkBackendInfo.Version}}")
	out := strings.TrimSpace(session.OutputToString())

	backend, rawVersion, ok := strings.Cut(out, "==")
	if !ok {
		Fail(fmt.Sprintf("[netavark]: unexpected podman info output: %q", out))
	}
	if backend != "netavark" {
		return
	}

	rawVersion = strings.TrimSpace(rawVersion)
	if rawVersion == "" {
		Fail(fmt.Sprintf("[netavark]: unexpected empty version in podman info output: %q", out))
	}

	parts := strings.Fields(rawVersion)
	if len(parts) == 0 {
		Fail(fmt.Sprintf("[netavark]: unexpected netavark version format: %q", rawVersion))
	}

	versionStr := strings.TrimPrefix(parts[len(parts)-1], "v")

	current, err := semver.Parse(versionStr)
	if err != nil {
		Fail(fmt.Sprintf("[netavark]: failed to parse netavark version %q: %v", rawVersion, err))
	}

	minVersion = strings.TrimPrefix(strings.TrimSpace(minVersion), "v")
	minVer, err := semver.Parse(minVersion)
	if err != nil {
		Fail(fmt.Sprintf("[netavark]: failed to parse minimum version %q: %v", minVersion, err))
	}

	if current.Compare(minVer) < 0 {
		Skip(fmt.Sprintf("[netavark]: need netavark >= %s; have %s", minVersion, versionStr))
	}
}

// SkipIfNotActive skips a test if the given systemd unit is not active
func SkipIfNotActive(unit string, reason string) {
	checkReason(reason)

	cmd := exec.Command("systemctl", "is-active", unit)
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	err := cmd.Run()
	if cmd.ProcessState.ExitCode() == 0 {
		return
	}
	Skip(fmt.Sprintf("[systemd]: unit %s is not active (%v): %s", unit, err, reason))
}

// PodmanAsUser is the exec call to podman on the filesystem with the specified uid/gid and environment
func (p *PodmanTestIntegration) PodmanAsUser(args []string, uid, gid uint32, cwd string, env []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanExecBaseWithOptions(args, PodmanExecOptions{
		UID: uid,
		GID: gid,
		CWD: cwd,
		Env: env,
	})
	return &PodmanSessionIntegration{podmanSession}
}

// RestartRemoteService stop and start API Server, usually to change config
func (p *PodmanTestIntegration) RestartRemoteService() {
	p.StopRemoteService()
	p.StartRemoteService()
}

// RestoreArtifactToCache populates the imagecache from tarballs that were cached earlier
func (p *PodmanTestIntegration) RestoreArtifactToCache(image string) error {
	tarball := imageTarPath(image)
	if _, err := os.Stat(tarball); err == nil {
		GinkgoWriter.Printf("Restoring %s...\n", image)
		p.Root = p.ImageCacheDir
		restore := p.PodmanNoEvents([]string{"load", "-q", "-i", tarball})
		restore.WaitWithDefaultTimeout()
		Expect(restore).To(ExitCleanly())
	}
	return nil
}

func populateCache(podman *PodmanTestIntegration) {
	for _, image := range CACHE_IMAGES {
		// FIXME: Remove this hack once composefs can be used with images
		// pulled from sources other than a registry.
		if strings.Contains(podman.StorageOptions, "overlay.use_composefs=true") {
			podman.pullImage(image, true)
		} else {
			err := podman.RestoreArtifactToCache(image)
			Expect(err).ToNot(HaveOccurred())
		}
	}
	// logformatter uses this to recognize the first test
	GinkgoWriter.Printf("-----------------------------\n")
}

// rmAll removes the directory and its content, when running rootless we use
// podman unshare to prevent any subuid/gid problems
func rmAll(podmanBin string, path string) {
	// Remove cache dirs
	if isRootless() {
		// If rootless, os.RemoveAll() is failed due to permission denied
		cmd := exec.Command(podmanBin, "unshare", "rm", "-rf", path)
		cmd.Stdout = GinkgoWriter
		cmd.Stderr = GinkgoWriter
		if err := cmd.Run(); err != nil {
			GinkgoWriter.Printf("%v\n", err)
		}
	} else {
		if err = os.RemoveAll(path); err != nil {
			GinkgoWriter.Printf("%q\n", err)
		}
	}
}

// PodmanNoCache calls the podman command with no configured imagecache
func (p *PodmanTestIntegration) PodmanNoCache(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanExecBaseWithOptions(args, PodmanExecOptions{
		NoCache: true,
	})
	return &PodmanSessionIntegration{podmanSession}
}

func PodmanTestSetup(tempDir string) *PodmanTestIntegration {
	return PodmanTestCreateUtil(tempDir, PodmanTestCreateUtilTargetLocal)
}

// PodmanNoEvents calls the Podman command without an imagecache and without an
// events backend. It is used mostly for caching and uncaching images.
func (p *PodmanTestIntegration) PodmanNoEvents(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanExecBaseWithOptions(args, PodmanExecOptions{
		NoEvents: true,
		NoCache:  true,
	})
	return &PodmanSessionIntegration{podmanSession}
}

// MakeOptions assembles all the podman main options
func (p *PodmanTestIntegration) makeOptions(args []string, options PodmanExecOptions) []string {
	if p.RemoteTest {
		if !slices.Contains(args, "--remote") {
			remoteArgs := []string{"--remote", "--url", p.RemoteSocket}
			if p.RemoteTLSServerCAFile != "" {
				remoteArgs = append(remoteArgs, "--tls-ca", p.RemoteTLSServerCAFile)
			}
			if p.RemoteTLSClientCertFile != "" {
				remoteArgs = append(remoteArgs, "--tls-cert", p.RemoteTLSClientCertFile)
			}
			if p.RemoteTLSClientKeyFile != "" {
				remoteArgs = append(remoteArgs, "--tls-key", p.RemoteTLSClientKeyFile)
			}
			return append(remoteArgs, args...)
		}
		return args
	}

	podmanOptions := []string{}
	if _, ok := os.LookupEnv("E2E_DEBUG"); ok {
		podmanOptions = append(podmanOptions, "--log-level=debug", "--syslog=true")
	}

	eventsType := "file"
	if options.NoEvents {
		eventsType = "none"
	}

	podmanOptions = append(podmanOptions,
		"--root", p.Root,
		"--runroot", p.RunRoot,
		"--runtime", p.OCIRuntime,
		"--conmon", p.ConmonBinary,
		"--network-config-dir", p.NetworkConfigDir,
		"--cgroup-manager", p.CgroupManager,
		"--tmpdir", p.TmpDir,
		"--events-backend", eventsType,
	)

	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	if !options.NoCache {
		cacheOptions := []string{
			"--storage-opt",
			fmt.Sprintf("%s.imagestore=%s", p.PodmanTest.ImageCacheFS, p.PodmanTest.ImageCacheDir),
		}
		podmanOptions = append(cacheOptions, podmanOptions...)
	}
	podmanOptions = append(podmanOptions, args...)
	return podmanOptions
}

func writeConf(conf []byte, confPath string) {
	GinkgoHelper()
	err := os.MkdirAll(filepath.Dir(confPath), 0o755)
	Expect(err).ToNot(HaveOccurred())

	err = ioutils.AtomicWriteFile(confPath, conf, 0o644)
	Expect(err).ToNot(HaveOccurred())
}

func removeConf(confPath string) {
	GinkgoHelper()
	err := os.Remove(confPath)
	// Network remove test will remove the config and then this can fail.
	// If the config does not exists no reason to hard error here.
	if !errors.Is(err, os.ErrNotExist) {
		Expect(err).ToNot(HaveOccurred())
	}
}

// generateNetworkConfig generates a Netavark config with a random name
// it returns the network name and the filepath
func generateNetworkConfig(p *PodmanTestIntegration) (string, string) {
	// generate a random name to prevent conflicts with other tests
	name := "net" + stringid.GenerateRandomID()
	path := filepath.Join(p.NetworkConfigDir, fmt.Sprintf("%s.json", name))
	conf := fmt.Sprintf(`
{
     "name": "%s",
     "id": "e1ef2749024b88f5663ca693a9118e036d6bfc48bcfe460faf45e9614a513e5c",
     "driver": "bridge",
     "network_interface": "netavark1",
     "created": "2022-01-05T14:15:10.975493521-06:00",
     "subnets": [
          {
               "subnet": "10.100.0.0/16",
               "gateway": "10.100.0.1"
          }
     ],
     "ipv6_enabled": false,
     "internal": false,
     "dns_enabled": true,
     "ipam_options": {
          "driver": "host-local"
     }
}
`, name)
	writeConf([]byte(conf), path)
	return name, path
}

func (p *PodmanTestIntegration) removeNetwork(name string) {
	session := p.Podman([]string{"network", "rm", "-f", name})
	session.WaitWithDefaultTimeout()
	Expect(session.ExitCode()).To(BeNumerically("<=", 1), "Exit code must be 0 or 1")
}

// generatePolicyFile generates a signature verification policy file.
// it returns the policy file path.
func generatePolicyFile(tempDir string, port int, sequoiaKeyPath string) string {
	gpgKeyPath := filepath.Join(tempDir, "key.gpg")
	policyPath := filepath.Join(tempDir, "policy.json")
	conf := fmt.Sprintf(`
{
    "default": [
        {
            "type": "insecureAcceptAnything"
        }
    ],
    "transports": {
        "docker": {
            "localhost:%[1]d": [
                {
                    "type": "signedBy",
                    "keyType": "GPGKeys",
                    "keyPath": "%[2]s"
                }
            ],
            "localhost:%[1]d/sigstore-signed": [
                {
                    "type": "sigstoreSigned",
                    "keyPath": "testdata/sigstore-key.pub"
                }
            ],
            "localhost:%[1]d/sigstore-signed-params": [
                {
                    "type": "sigstoreSigned",
                    "keyPath": "testdata/sigstore-key.pub"
                }
            ],
            "localhost:%[1]d/simple-sq-signed": [
                {
                    "type": "signedBy",
                    "keyType": "GPGKeys",
                    "keyPath": "%[3]s"
                }
            ]
        }
    }
}
`, port, gpgKeyPath, sequoiaKeyPath)
	writeConf([]byte(conf), policyPath)
	return policyPath
}

func (s *PodmanSessionIntegration) jq(jqCommand string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("jq", jqCommand)
	cmd.Stdin = strings.NewReader(s.OutputToString())
	cmd.Stdout = &out
	err := cmd.Run()
	return strings.TrimRight(out.String(), "\n"), err
}

func (p *PodmanTestIntegration) buildImage(dockerfile, imageName string, layers string, label string, extraOptions []string) string {
	buildDir := filepath.Join(p.TempDir, "build"+stringid.GenerateRandomID())
	err := os.Mkdir(buildDir, 0o755)
	Expect(err).ToNot(HaveOccurred())
	dockerfilePath := filepath.Join(buildDir, "Dockerfile-"+stringid.GenerateRandomID())
	err = os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644)
	Expect(err).ToNot(HaveOccurred())
	cmd := []string{"build", "--pull-never", "--layers=" + layers, "--file", dockerfilePath}
	if label != "" {
		cmd = append(cmd, "--label="+label)
	}
	if len(imageName) > 0 {
		cmd = append(cmd, []string{"-t", imageName}...)
	}
	if len(extraOptions) > 0 {
		cmd = append(cmd, extraOptions...)
	}
	cmd = append(cmd, buildDir)
	session := p.Podman(cmd)
	session.Wait(240)
	Expect(session).Should(Exit(0), fmt.Sprintf("BuildImage session output: %q", session.OutputToString()))
	output := session.OutputToStringArray()
	return output[len(output)-1]
}

func writeYaml(content string, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}

// GetPort finds an unused TCP/IP port on the system, in the range 5000-5999
func GetPort() int {
	portMin := 5000
	portMax := 5999

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Avoid dup-allocation races between parallel ginkgo processes
	nProcs := GinkgoT().ParallelTotal()
	myProc := GinkgoT().ParallelProcess() - 1

	for range 50 {
		// Random port within that range
		port := portMin + rng.Intn((portMax-portMin)/nProcs)*nProcs + myProc

		used, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
		if err == nil {
			// it's open. Return it.
			err = used.Close()
			Expect(err).ToNot(HaveOccurred(), "closing random port")
			return port
		}
	}

	Fail(fmt.Sprintf("unable to get free port in range %d-%d", portMin, portMax))
	return 0 // notreached
}

// testPortConnection check if we can connect to the given tcp port
// This is doing some retries in case the container process has not yet bound the port.
func testPortConnection(port int) {
	GinkgoHelper()
	var conn net.Conn
	var err error

	for range 5 {
		conn, err = net.Dial("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)))
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	Expect(err).ToNot(HaveOccurred())
}

// startNCContainer starts a detached container running nc (netcat) listening
// on the given port, waits for it to be ready, and returns the container name.
//
//nolint:unused,nolintlint // only called from linux-only test files, unused on freebsd
func (p *PodmanTestIntegration) startNCContainer(name string, listenPort int, extraArgs ...string) string {
	GinkgoHelper()
	portStr := strconv.Itoa(listenPort)
	args := append([]string{"run", "-d", "--name", name}, extraArgs...)
	args = append(args, ALPINE, "sh", "-c", "nc -l -n -v -p "+portStr+" 2>&1")
	p.PodmanExitCleanly(args...)
	p.WaitForContainerLog(name, "listening")
	return name
}

// WaitForContainerLog polls container logs until the given substring appears
// in either stdout or stderr. Fails the test if not found within the timeout.
func (p *PodmanTestIntegration) WaitForContainerLog(ctrName string, substr string) {
	GinkgoHelper()
	for range 10 {
		logs := p.Podman([]string{"logs", ctrName})
		logs.WaitWithDefaultTimeout()
		if strings.Contains(logs.ErrorToString(), substr) || strings.Contains(logs.OutputToString(), substr) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	Fail(fmt.Sprintf("timed out waiting for %q in logs of container %s", substr, ctrName))
}

func createNetworkName(name string) string {
	return name + stringid.GenerateRandomID()[:10]
}

var IPRegex = `(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`

// digShort execs into the given container and does a dig lookup with a timeout
// backoff.  If it gets a response, it ensures that the output is in the correct
// format and iterates a string array for match
func digShort(container, lookupName, expectedIP string, p *PodmanTestIntegration) {
	digInterval := time.Millisecond * 250
	for i := range 6 {
		time.Sleep(digInterval * time.Duration(i))
		dig := p.Podman([]string{"exec", container, "dig", "+short", lookupName})
		dig.WaitWithDefaultTimeout()
		output := dig.OutputToString()
		if dig.ExitCode() == 0 && output != "" {
			Expect(output).To(Equal(expectedIP))
			// success
			return
		}
	}
	Fail("dns is not responding")
}

// WaitForFile to be created in defaultWaitTimeout seconds, returns false if file not created
func WaitForFile(path string) (err error) {
	until := time.Now().Add(time.Duration(defaultWaitTimeout) * time.Second)
	for time.Now().Before(until) {
		_, err = os.Stat(path)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, os.ErrNotExist):
			time.Sleep(10 * time.Millisecond)
		default:
			return err
		}
	}
	return err
}

// WaitForService blocks for defaultWaitTimeout seconds, waiting for some service listening on given host:port
func WaitForService(address url.URL) {
	// Wait for podman to be ready
	var err error
	until := time.Now().Add(time.Duration(defaultWaitTimeout) * time.Second)
	for time.Now().Before(until) {
		var conn net.Conn
		conn, err = net.Dial("tcp", address.Host)
		if err == nil {
			conn.Close()
			break
		}

		// Podman not available yet...
		time.Sleep(10 * time.Millisecond)
	}
	Expect(err).ShouldNot(HaveOccurred())
}

// useCustomNetworkDir makes sure this test uses a custom network dir.
// This needs to be called for all test they may remove networks from other tests,
// so netwokr prune, system prune, or system reset.
// see https://github.com/containers/podman/issues/17946
// Note that when using this and running containers with custom networks you must use the
// ginkgo Serial decorator to ensure no parallel test are running otherwise we get flakes,
// https://github.com/containers/podman/issues/23876
func useCustomNetworkDir(podmanTest *PodmanTestIntegration, tempdir string) {
	// set custom network directory to prevent flakes since the dir is shared with all tests by default
	podmanTest.NetworkConfigDir = tempdir
	if IsRemote() {
		podmanTest.RestartRemoteService()
	}
}

// copy directories recursively from source path to destination path
func CopyDirectory(srcDir, dest string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %q, error: %q", destPath, err.Error())
			}
			if err := CopyDirectory(sourcePath, destPath); err != nil {
				return err
			}
		case os.ModeSymlink:
			if err := CopySymLink(sourcePath, destPath); err != nil {
				return err
			}
		default:
			if err := Copy(sourcePath, destPath); err != nil {
				return err
			}
		}

		fInfo, err := entry.Info()
		if err != nil {
			return err
		}

		isSymlink := fInfo.Mode()&os.ModeSymlink != 0
		if !isSymlink {
			if err := os.Chmod(destPath, fInfo.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func Copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}

	defer out.Close()

	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	defer in.Close()
	return nil
}

func CopySymLink(source, dest string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}
	return os.Symlink(link, dest)
}

func UsingCacheRegistry() bool {
	return os.Getenv("CI_USE_REGISTRY_CACHE") != ""
}

func setupRegistry(portOverride *int) (*lockfile.LockFile, string, error) {
	var port string
	if isRootless() {
		if err := podmanTest.RestoreArtifact(REGISTRY_IMAGE); err != nil {
			return nil, "", err
		}
	}

	if portOverride != nil {
		port = strconv.Itoa(*portOverride)
	} else {
		p, err := utils.GetRandomPort()
		if err != nil {
			return nil, "", err
		}
		port = strconv.Itoa(p)
	}

	lock := GetPortLock(port)

	session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", fmt.Sprintf("%s:5000", port), REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
	session.WaitWithDefaultTimeout()
	Expect(session).Should(ExitCleanly())

	if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
		lock.Unlock()
		Skip("Cannot start docker registry.")
	}
	return lock, port, nil
}

func createArtifactFile(numBytes int64) (string, error) {
	GinkgoHelper()
	artifactDir := filepath.Join(podmanTest.TempDir, "artifacts")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return "", err
	}
	filename := RandomString(8)
	outFile := filepath.Join(artifactDir, filename)

	f, err := os.Create(filepath.Join(artifactDir, filename))
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = io.CopyN(f, crand.Reader, numBytes)
	if err != nil {
		return "", err
	}
	return outFile, nil
}

func makeTempDirInDir(dir string) string {
	GinkgoHelper()
	path, err := os.MkdirTemp(dir, "podman-test")
	Expect(err).ToNot(HaveOccurred())
	return path
}

func skipWithoutDevNullb0() {
	SkipIfNotExist("use modprobe null_blk nr_devices=1 to create it", "/dev/nullb0")
}

func SkipIfNotAMD64() {
	if podmanTest.Host.Arch != "amd64" {
		Skip("test only valid on amd64")
	}
}
