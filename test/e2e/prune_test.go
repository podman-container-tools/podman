//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var pruneImage = fmt.Sprintf(`
FROM  %s
LABEL RUN podman --version
RUN echo hello > /hello
RUN echo hello2 > /hello2`, ALPINE)

var emptyPruneImage = `
FROM scratch
ENV test1=test1
ENV test2=test2`

var longBuildImage = fmt.Sprintf(`
FROM %s
RUN echo "Hello, World!"
RUN RUN echo "Please use signal 9 this will never ends" && sleep 10000s`, ALPINE)

var _ = Describe("Podman prune", func() {
	It("podman container prune containers", func() {
		top := podmanTest.RunTopContainer("")
		top.WaitWithDefaultTimeout()
		Expect(top).Should(ExitCleanly())

		top = podmanTest.RunTopContainer("")
		top.WaitWithDefaultTimeout()
		Expect(top).Should(ExitCleanly())
		cid := top.OutputToString()

		podmanTest.StopContainer(cid)

		prune := podmanTest.Podman([]string{"container", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman container prune after create containers", func() {
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		prune := podmanTest.Podman([]string{"container", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman container prune after create & init containers", func() {
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		init := podmanTest.Podman([]string{"init", "test"})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(ExitCleanly())

		prune := podmanTest.Podman([]string{"container", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman container prune filters by annotation", func() {
		podmanTest.PodmanExitCleanly("create", "--annotation", "prune=me", "--name", "prune-me", BB)
		podmanTest.PodmanExitCleanly("create", "--annotation", "prune=keep", "--name", "keep-me", BB)
		podmanTest.PodmanExitCleanly("container", "prune", "-f", "--filter", "annotation=prune=me")
		session := podmanTest.PodmanExitCleanly("ps", "-a", "--format", "{{.Names}}")
		Expect(session.OutputToStringArray()).To(Equal([]string{"keep-me"}))
	})

	It("podman container prune filters by negated annotation", func() {
		podmanTest.PodmanExitCleanly("create", "--annotation", "prune=me", "--name", "prune-me", BB)
		podmanTest.PodmanExitCleanly("create", "--annotation", "prune=keep", "--name", "keep-me", BB)
		podmanTest.PodmanExitCleanly("container", "prune", "-f", "--filter", "annotation!=prune=me")
		session := podmanTest.PodmanExitCleanly("ps", "-a", "--format", "{{.Names}}")
		Expect(session.OutputToStringArray()).To(Equal([]string{"prune-me"}))
	})

	It("podman image prune - remove only dangling images", func() {
		session := podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("<none>")))
		numImages := len(session.OutputToStringArray())

		// Since there's no dangling image, none should be removed.
		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(BeEmpty())

		// Let's be extra sure that the same number of images is
		// reported.
		session = podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(numImages))

		// Now build an image and untag it.  The (intermediate) images
		// should be removed recursively during pruning.
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		session = podmanTest.Podman([]string{"untag", "alpine_bash:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("<none>"))
		numImages = len(session.OutputToStringArray())

		// Since there's at least one dangling image, prune should
		// remove them.
		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		numPrunedImages := len(session.OutputToStringArray())
		Expect(numPrunedImages).To(BeNumerically(">=", 1), "numPrunedImages")

		// Now make sure that exactly the number of pruned images has
		// been removed.
		session = podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(numImages - numPrunedImages))
	})

	It("podman image prune - handle empty images", func() {
		// As shown in #10832, empty images were not treated correctly
		// in Podman.
		podmanTest.BuildImage(emptyPruneImage, "empty:scratch", "true")

		session := podmanTest.Podman([]string{"images", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("<none>"))

		// Nothing will be pruned.
		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(BeEmpty())

		// Now the image will be untagged, and its parent images will
		// be removed recursively.
		session = podmanTest.Podman([]string{"untag", "empty:scratch"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"image", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman image prune dangling images", func() {
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		none := podmanTest.Podman([]string{"images", "-a"})
		none.WaitWithDefaultTimeout()
		Expect(none).Should(ExitCleanly())
		hasNone, result := none.GrepString("<none>")
		Expect(result).To(HaveLen(2))
		Expect(hasNone).To(BeTrue())

		prune := podmanTest.Podman([]string{"image", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		after := podmanTest.Podman([]string{"images", "-a"})
		after.WaitWithDefaultTimeout()
		Expect(after).Should(ExitCleanly())
		hasNoneAfter, result := after.GrepString("<none>")
		Expect(hasNoneAfter).To(BeTrue())
		Expect(len(after.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(result).ToNot(BeEmpty())
	})

	It("podman image prune unused images", func() {
		SkipIfNotAMD64() // List of images is different
		podmanTest.AddImageToRWStore(ALPINE)
		podmanTest.AddImageToRWStore(BB)

		images := podmanTest.Podman([]string{"images", "-a"})
		images.WaitWithDefaultTimeout()
		Expect(images).Should(ExitCleanly())

		prune := podmanTest.Podman([]string{"image", "prune", "-af"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		images = podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		Expect(images).Should(ExitCleanly())
		// all images are unused, so they all should be deleted!
		Expect(images.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	It("podman image prune dry run", func() {
		SkipIfNotAMD64() // List of images is different
		podmanTest.AddImageToRWStore(ALPINE)
		podmanTest.AddImageToRWStore(BB)

		images := podmanTest.Podman([]string{"images", "-a"})
		images.WaitWithDefaultTimeout()
		Expect(images).Should(ExitCleanly())

		prune := podmanTest.Podman([]string{"image", "prune", "-af", "--dry-run"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		after := podmanTest.Podman([]string{"images", "-a"})
		after.WaitWithDefaultTimeout()
		Expect(after).Should(ExitCleanly())
		// the dry run should not remove or untag any images
		Expect(after.OutputToStringArray()).To(HaveLen(len(images.OutputToStringArray())))
	})
	
	It("podman system image prune unused images", func() {
		SkipIfNotAMD64() // List of images is different
		useCustomNetworkDir(podmanTest, tempdir)
		podmanTest.AddImageToRWStore(ALPINE)
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		prune := podmanTest.Podman([]string{"system", "prune", "-a", "--force"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(images.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	It("podman system prune pods", func() {
		useCustomNetworkDir(podmanTest, tempdir)
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podmanTest.StopPod(podid1)

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods).Should(ExitCleanly())
		Expect(pods.OutputToStringArray()).To(HaveLen(3))

		prune := podmanTest.Podman([]string{"system", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		pods = podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods).Should(ExitCleanly())
		Expect(pods.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman system prune networks", func() {
		useCustomNetworkDir(podmanTest, tempdir)
		// Create new network.
		session := podmanTest.Podman([]string{"network", "create", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Remove all unused networks.
		session = podmanTest.Podman([]string{"system", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Default network should exists.
		session = podmanTest.Podman([]string{"network", "ls", "-q", "--filter", "name=^podman$"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		// Unused networks removed.
		session = podmanTest.Podman([]string{"network", "ls", "-q", "--filter", "name=^test$"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(BeEmpty())

		// Create new network.
		session = podmanTest.Podman([]string{"network", "create", "test1", "--label", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Remove all unused networks.
		session = podmanTest.Podman([]string{"system", "prune", "-f", "--filter", "label!=foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("Total reclaimed space: 0B"))

		// Unused networks removed.
		session = podmanTest.Podman([]string{"network", "ls", "-q", "--filter", "name=^test1$"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// label should make sure we do not remove this network
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman system prune - pod,container stopped", func() {
		useCustomNetworkDir(podmanTest, tempdir)
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podid1 := session.OutputToString()

		// Start and stop a pod to get it in exited state.
		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podmanTest.StopPod(podid1)

		// Create a container. This container should be pruned.
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		prune := podmanTest.Podman([]string{"system", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods).Should(ExitCleanly())
		Expect(podmanTest.NumberOfPods()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman system prune with running, exited pod and volume prune set true", func() {
		useCustomNetworkDir(podmanTest, tempdir)
		// Start and stop a pod to get it in exited state.
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podmanTest.StopPod(podid1)

		// Start a pod and leave it running
		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podid2 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podid2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Number of pod should be 2. One exited one running.
		Expect(podmanTest.NumberOfPods()).To(Equal(2))

		// Create a container. This container should be pruned.
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		// Number of containers should be three now.
		// Two as pods infra container and one newly created.
		Expect(podmanTest.NumberOfContainers()).To(Equal(3))

		// image list current count should not be pruned if all flag isn't enabled
		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		numberOfImages := len(session.OutputToStringArray())

		// Adding unused volume should be pruned
		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "-v", "myvol:/myvol", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(3))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Volumes should be pruned.
		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		// One Pod should not be pruned as it was running
		Expect(podmanTest.NumberOfPods()).To(Equal(1))

		// Running pods infra container should not be pruned.
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		// Image should not be pruned and number should be same.
		images := podmanTest.Podman([]string{"images"})
		images.WaitWithDefaultTimeout()
		Expect(images.OutputToStringArray()).To(HaveLen(numberOfImages))
	})

	It("podman system prune - with dangling images true", func() {
		SkipIfNotAMD64() // List of images is different
		useCustomNetworkDir(podmanTest, tempdir)
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podid1 := session.OutputToString()

		// Start and stop a pod to get it in exited state.
		session = podmanTest.Podman([]string{"pod", "start", podid1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		podmanTest.StopPod(podid1)

		// Create a container. This container should be pruned.
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		// Adding unused volume should not be pruned as volumes not set
		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		prune := podmanTest.Podman([]string{"system", "prune", "-f", "-a"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods).Should(ExitCleanly())
		Expect(podmanTest.NumberOfPods()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Volumes should not be pruned
		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(images.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	It("podman system prune --volumes --filter", func() {
		useCustomNetworkDir(podmanTest, tempdir)
		session := podmanTest.Podman([]string{"volume", "create", "--label", "label1=value1", "myvol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1=slv1", "myvol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1=slv2", "myvol3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1", "myvol4"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "-v", "myvol5:/myvol5", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "-v", "myvol6:/myvol6", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(7))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes", "--filter", "label=label1=value1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(6))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes", "--filter", "label=sharedlabel1=slv1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(5))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes", "--filter", "label=sharedlabel1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(3))
	})

	It("podman system prune --all --external fails", func() {
		prune := podmanTest.Podman([]string{"system", "prune", "--all", "--external"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitWithError(125, "--external cannot be combined with other options"))
	})

	It("podman system prune --external leaves referenced containers", func() {
		useCustomNetworkDir(podmanTest, tempdir)
		containerStorageDir := filepath.Join(podmanTest.Root, podmanTest.ImageCacheFS+"-containers")

		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		// Container should exist
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		// have: containers.json, containers.lock and container dir
		dirents, err := os.ReadDir(containerStorageDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(dirents).To(HaveLen(3))

		prune := podmanTest.Podman([]string{"system", "prune", "--external", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		// Container should still exist
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		// still have: containers.json, containers.lock and container dir
		dirents, err = os.ReadDir(containerStorageDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(dirents).To(HaveLen(3))
	})

	It("podman system prune --external removes unreferenced containers", func() {
		SkipIfRemote("Can't drop database while daemon running")
		useCustomNetworkDir(podmanTest, tempdir)

		containerStorageDir := filepath.Join(podmanTest.Root, podmanTest.ImageCacheFS+"-containers")

		// Create container 1
		create := podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		// containers.json, containers.lock and container 1 dir
		dirents, err := os.ReadDir(containerStorageDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(dirents).To(HaveLen(3))

		// Drop podman database and storage, losing track of container 1 (but directory remains)
		err = os.Remove(filepath.Join(containerStorageDir, "containers.json"))
		Expect(err).ToNot(HaveOccurred())

		if podmanTest.DatabaseBackend == "sqlite" {
			err = os.Remove(filepath.Join(podmanTest.Root, "db.sql"))
			Expect(err).ToNot(HaveOccurred())
		} else {
			dbDir := filepath.Join(podmanTest.Root, "libpod")
			err = os.RemoveAll(dbDir)
			Expect(err).ToNot(HaveOccurred())
		}

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Create container 2
		create = podmanTest.Podman([]string{"create", "--name", "test", BB})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		// containers.json, containers.lock and container 1&2 dir
		dirents, err = os.ReadDir(containerStorageDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(dirents).To(HaveLen(4))

		prune := podmanTest.Podman([]string{"system", "prune", "--external", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		// container 1 dir should be gone now
		dirents, err = os.ReadDir(containerStorageDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(dirents).To(HaveLen(3))
	})

	It("podman system prune --build clean up after terminated build", func() {
		useCustomNetworkDir(podmanTest, tempdir)

		podmanTest.BuildImage(pruneImage, "alpine_notleaker:latest", "false")

		create := podmanTest.Podman([]string{"create", "--name", "test", BB, "sleep", "10000"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		containerFilePath := filepath.Join(podmanTest.TempDir, "ContainerFile-podman-leaker")
		err := os.WriteFile(containerFilePath, []byte(longBuildImage), 0o755)
		Expect(err).ToNot(HaveOccurred())

		build := podmanTest.Podman([]string{"build", "--network=none", "-f", containerFilePath, "-t", "podmanleaker"})
		// Build will never finish so let's wait for build to ask for SIGKILL to simulate a failed build that leaves stage containers.
		matchedOutput := false
		for range 900 {
			if build.LineInOutputContains("Please use signal 9") {
				matchedOutput = true
				build.Signal(syscall.SIGKILL)
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if !matchedOutput {
			Fail("Did not match special string in podman build")
		}

		// kill is async, wait for process exit here and make sure it was killed (137).
		build.WaitWithDefaultTimeout()
		Expect(build).To(Exit(137))

		// Check Intermediate image of stage container
		none := podmanTest.Podman([]string{"images", "-a"})
		none.WaitWithDefaultTimeout()
		Expect(none).Should(ExitCleanly())
		Expect(none.OutputToString()).Should(ContainSubstring("none"))

		// Check if Container and Stage Container exist
		count := podmanTest.Podman([]string{"ps", "-aq", "--external"})
		count.WaitWithDefaultTimeout()
		Expect(count).Should(ExitCleanly())
		Expect(count.OutputToStringArray()).To(HaveLen(3))

		prune := podmanTest.Podman([]string{"system", "prune", "--build", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		// Container should still exist, but no stage containers
		count = podmanTest.Podman([]string{"ps", "-aq", "--external"})
		count.WaitWithDefaultTimeout()
		Expect(count).Should(ExitCleanly())
		Expect(count.OutputToString()).To(BeEmpty())

		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		after := podmanTest.Podman([]string{"images", "-a"})
		after.WaitWithDefaultTimeout()
		Expect(after).Should(ExitCleanly())
		Expect(after.OutputToString()).ShouldNot(ContainSubstring("none"))
		Expect(after.OutputToString()).Should(ContainSubstring("notleaker"))
	})
})
