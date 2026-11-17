package bindings_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/libpod/define"
	. "go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/specgen"
)

type testImage struct {
	name        string
	shortName   string
	tarballName string
}

const (
	devPodmanBinaryLocation     string = "../../../bin/podman"
	defaultPodmanBinaryLocation string = "/usr/bin/podman"
)

func getPodmanBinary() string {
	_, err := os.Stat(devPodmanBinaryLocation)
	if errors.Is(err, os.ErrNotExist) {
		return defaultPodmanBinaryLocation
	}
	return devPodmanBinaryLocation
}

var (
	ImageCacheDir = "/tmp/podman/imagecachedir"
	LockTmpDir    string
	alpine        = testImage{
		name:        "docker.io/library/alpine:latest",
		shortName:   "alpine",
		tarballName: "alpine.tar",
	}
	busybox = testImage{
		name:        "docker.io/library/busybox:latest",
		shortName:   "busybox",
		tarballName: "busybox.tar",
	}
	CACHE_IMAGES = []testImage{alpine, busybox}
)

type bindingTest struct {
	artifactDirPath string
	imageCacheDir   string
	sock            string
	tempDirPath     string
	runRoot         string
	crioRoot        string
	conn            context.Context
}

func (b *bindingTest) NewConnection() error {
	connText, err := NewConnection(context.Background(), b.sock)
	if err != nil {
		return err
	}
	b.conn = connText
	return nil
}

func (b *bindingTest) runPodman(command []string) *Session {
	var cmd []string
	podmanBinary := getPodmanBinary()
	val, ok := os.LookupEnv("PODMAN_BINARY")
	if ok {
		podmanBinary = val
	}
	val, ok = os.LookupEnv("CGROUP_MANAGER")
	if ok {
		cmd = append(cmd, "--cgroup-manager", val)
	}
	val, ok = os.LookupEnv("CONMON")
	if ok {
		cmd = append(cmd, "--conmon", val)
	}
	val, ok = os.LookupEnv("ROOT")
	if ok {
		cmd = append(cmd, "--root", val)
	} else {
		cmd = append(cmd, "--root", b.crioRoot)
	}
	val, ok = os.LookupEnv("OCI_RUNTIME")
	if ok {
		cmd = append(cmd, "--runtime", val)
	}
	val, ok = os.LookupEnv("RUNROOT")
	if ok {
		cmd = append(cmd, "--runroot", val)
	} else {
		cmd = append(cmd, "--runroot", b.runRoot)
	}
	val, ok = os.LookupEnv("TEMPDIR")
	if ok {
		cmd = append(cmd, "--tmpdir", val)
	} else {
		cmd = append(cmd, "--tmpdir", b.tempDirPath)
	}
	val, ok = os.LookupEnv("STORAGE_DRIVER")
	if ok {
		cmd = append(cmd, "--storage-driver", val)
	}
	val, ok = os.LookupEnv("STORAGE_OPTIONS")
	if ok {
		cmd = append(cmd, "--storage", val)
	}
	cmd = append(cmd, command...)
	c := exec.Command(podmanBinary, cmd...)
	fmt.Printf("Running: %s %s\n", podmanBinary, strings.Join(cmd, " "))
	session, err := Start(c, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		panic(fmt.Errorf("unable to run podman command: %q", cmd))
	}
	return session
}

func newBindingTest() *bindingTest {
	tmpPath, _ := createTempDirInTempDir()
	b := bindingTest{
		crioRoot:        filepath.Join(tmpPath, "crio"),
		runRoot:         filepath.Join(tmpPath, "run"),
		artifactDirPath: "",
		imageCacheDir:   "",
		sock:            fmt.Sprintf("unix://%s", filepath.Join(tmpPath, "api.sock")),
		tempDirPath:     tmpPath,
	}
	return &b
}

// createTempDirinTempDir create a temp dir with prefix podman_test
func createTempDirInTempDir() (string, error) {
	return os.MkdirTemp("", "libpod_api")
}

func (b *bindingTest) startAPIService() *Session {
	cmd := []string{"--log-level=debug", "system", "service", "--timeout=0", b.sock}
	session := b.runPodman(cmd)

	sock := strings.TrimPrefix(b.sock, "unix://")
	for range 10 {
		if _, err := os.Stat(sock); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				break
			}
			time.Sleep(time.Second)
			continue
		}
		break
	}
	return session
}

func (b *bindingTest) cleanup() {
	s := b.runPodman([]string{"stop", "-a", "-t", "0"})
	s.Wait(45)
	if err := os.RemoveAll(b.tempDirPath); err != nil {
		fmt.Println(err)
	}
}

// Pull is a helper function to pull in images
func (b *bindingTest) Pull(name string) {
	p := b.runPodman([]string{"pull", name})
	p.Wait(45)
	Expect(p).To(Exit(0))
}

func (b *bindingTest) Save(i testImage) {
	p := b.runPodman([]string{"save", "-o", filepath.Join(ImageCacheDir, i.tarballName), i.name})
	p.Wait(45)
	Expect(p).To(Exit(0))
}

func (b *bindingTest) RestoreImagesFromCache() {
	for _, i := range CACHE_IMAGES {
		b.restoreImageFromCache(i)
	}
}

func (b *bindingTest) restoreImageFromCache(i testImage) {
	p := b.runPodman([]string{"load", "-i", filepath.Join(ImageCacheDir, i.tarballName)})
	p.Wait(45)
	Expect(p).To(Exit(0))
}

// Run a container within or without a pod
// and add or append the alpine image to it
func (b *bindingTest) RunTopContainer(containerName *string, podName *string) (string, error) {
	s := specgen.NewSpecGenerator(alpine.name, false)
	terminal := false
	s.Terminal = &terminal
	s.Command = []string{"/usr/bin/top"}
	if containerName != nil {
		s.Name = *containerName
	}
	if podName != nil {
		s.Pod = *podName
	}
	ctr, err := containers.CreateWithSpec(b.conn, s, nil)
	if err != nil {
		return "", err
	}
	err = containers.Start(b.conn, ctr.ID, nil)
	if err != nil {
		return "", err
	}
	wait := define.ContainerStateRunning
	_, err = containers.Wait(b.conn, ctr.ID, new(containers.WaitOptions).WithCondition([]define.ContainerStatus{wait}))
	return ctr.ID, err
}

// This method creates a pod with the given pod name.
// Podname is an optional parameter
func (b *bindingTest) Podcreate(name *string) {
	b.PodcreateAndExpose(name, nil)
}

// This method creates a pod with the given pod name and publish port.
// Podname is an optional parameter
// port is an optional parameter
func (b *bindingTest) PodcreateAndExpose(name *string, port *string) {
	command := []string{"pod", "create"}
	if name != nil {
		podname := *name
		command = append(command, "--name", podname)
	}
	if port != nil {
		podport := *port
		command = append(command, "--publish", podport)
	}
	b.runPodman(command).Wait(45)
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	// make cache dir
	err := os.MkdirAll(ImageCacheDir, 0o777)
	Expect(err).ToNot(HaveOccurred())

	// If running localized tests, the cache dir is created and populated. if the
	// tests are remote, this is a no-op
	createCache()
	path, err := os.MkdirTemp("", "libpodlock")
	Expect(err).ToNot(HaveOccurred())

	return []byte(path)
}, func(data []byte) {
	LockTmpDir = string(data)
})

func createCache() {
	b := newBindingTest()
	for _, i := range CACHE_IMAGES {
		_, err := os.Stat(filepath.Join(ImageCacheDir, i.tarballName))
		if errors.Is(err, os.ErrNotExist) {
			//	pull the image
			b.Pull(i.name)
			b.Save(i)
		}
	}
	b.cleanup()
}

func isStopped(state string) bool {
	return state == "exited" || state == "stopped"
}
