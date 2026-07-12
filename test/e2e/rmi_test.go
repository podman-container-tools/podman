//go:build linux || freebsd

package integration

import (
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman rmi", func() {
	It("podman rmi bogus image", func() {
		session := podmanTest.Podman([]string{"rmi", "debian:6.0.10"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, "debian:6.0.10: image not known"))
	})

	It("podman rmi with fq name", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman rmi with short name", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		session := podmanTest.Podman([]string{"rmi", "cirros"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman rmi all images", func() {
		podmanTest.AddImageToRWStore(NGINX_IMAGE)
		session := podmanTest.Podman([]string{"rmi", "-a"})
		session.WaitWithDefaultTimeout()
		images := podmanTest.Podman([]string{"images"})
		images.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman rmi all images forcibly with short options", func() {
		podmanTest.AddImageToRWStore(NGINX_IMAGE)
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman rmi tagged image", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		setup := podmanTest.Podman([]string{"images", "-q", CIRROS_IMAGE})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"tag", CIRROS_IMAGE, "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"images", "-q", "foo"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		Expect(result.OutputToString()).To(ContainSubstring(setup.OutputToString()))
	})

	It("podman rmi image with tags by ID cannot be done without force", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		setup := podmanTest.Podman([]string{"images", "-q", "--no-trunc", CIRROS_IMAGE})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cirrosID := setup.OutputToString()[7:]

		session := podmanTest.Podman([]string{"tag", "cirros", "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Trying without --force should fail
		result := podmanTest.Podman([]string{"rmi", cirrosID})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, fmt.Sprintf(`unable to delete image "%s" by ID with more than one tag ([localhost/foo:latest localhost/foo:bar %s]): please force removal`, cirrosID, CIRROS_IMAGE)))

		// With --force it should work
		resultForce := podmanTest.Podman([]string{"rmi", "-f", cirrosID})
		resultForce.WaitWithDefaultTimeout()
		Expect(resultForce).Should(ExitCleanly())
	})

	It("podman rmi image that is a parent of another image", func() {
		SkipIfNotAMD64() // List of images is different
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		session := podmanTest.Podman([]string{"run", "--name", "c_test", CIRROS_IMAGE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"commit", "-q", "c_test", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rm", "c_test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rmi", CIRROS_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("Untagged: " + CIRROS_IMAGE))

		session = podmanTest.Podman([]string{"images", "--sort", "created", "--format", "{{.Id}}", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)+1),
			"Output from 'podman images'")
		untaggedImg := session.OutputToStringArray()[1]

		session = podmanTest.Podman([]string{"rmi", "-f", untaggedImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, fmt.Sprintf(`cannot remove read-only image "%s"`, untaggedImg)))
	})

	It("podman rmi image that is created from another named imaged", func() {
		SkipIfNotAMD64() // List of images is different
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"create", "--name", "c_test1", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"commit", "-q", "c_test1", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--name", "c_test2", "test1", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"commit", "-q", "c_test2", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rm", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rmi", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"images", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES) + 1))
	})

	It("podman rmi with cached images", func() {
		SkipIfNotAMD64() // List of images is different
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		dockerfile := fmt.Sprintf(`FROM %s
		RUN mkdir hello
		RUN touch test.txt
		ENV foo=bar
		`, CIRROS_IMAGE)
		podmanTest.BuildImage(dockerfile, "test", "true")

		dockerfile = fmt.Sprintf(`FROM %s
		RUN mkdir hello
		RUN touch test.txt
		RUN mkdir blah
		ENV foo=bar
		`, CIRROS_IMAGE)

		podmanTest.BuildImage(dockerfile, "test2", "true")

		session := podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		numOfImages := len(session.OutputToStringArray())

		session = podmanTest.Podman([]string{"rmi", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(numOfImages - len(session.OutputToStringArray())).To(Equal(2))

		session = podmanTest.Podman([]string{"rmi", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES) + 1))

		podmanTest.BuildImage(dockerfile, "test3", "true")

		session = podmanTest.Podman([]string{"rmi", CIRROS_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rmi", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"images", "-q", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	It("podman rmi -a with no images should be exit 0", func() {
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"rmi", "-fa"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
	})

	It("podman rmi -a with parent|child images", func() {
		SkipIfNotAMD64() // List of images is different
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		dockerfile := fmt.Sprintf(`FROM %s AS base
RUN touch /1
ENV LOCAL=/1
RUN find $LOCAL
FROM base
RUN find $LOCAL

`, CIRROS_IMAGE)
		podmanTest.BuildImage(dockerfile, "test", "true")
		session := podmanTest.Podman([]string{"rmi", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		Expect(images).Should(ExitCleanly())
		Expect(images.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	// Don't rerun all tests; just assume that if we get that diagnostic,
	// we're getting rmi
	It("podman image rm is the same as rmi", func() {
		session := podmanTest.Podman([]string{"image", "rm"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "image name or ID must be specified"))
	})

	It("podman image rm - concurrent with shared layers", func() {
		// #6510 has shown a fairly simple reproducer to force storage
		// errors during parallel image removal.  Since it's subject to
		// a race, we may not hit the condition a 100 percent of times
		// but ocal reproducers hit it all the time.

		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		var wg sync.WaitGroup

		// Prepare images
		wg.Add(10)
		for i := range 10 {
			go func(i int) {
				defer GinkgoRecover()
				defer wg.Done()
				imageName := fmt.Sprintf("rmtest:%d", i)
				containerfile := fmt.Sprintf(`FROM %s
	RUN touch %s`, CIRROS_IMAGE, imageName)

				podmanTest.BuildImage(containerfile, imageName, "false")
			}(i)
		}
		wg.Wait()

		// A herd of concurrent removals
		wg.Add(10)
		for i := range 10 {
			go func(i int) {
				defer GinkgoRecover()
				defer wg.Done()

				imageName := fmt.Sprintf("rmtest:%d", i)
				session := podmanTest.Podman([]string{"rmi", "-f", imageName})
				session.WaitWithDefaultTimeout()
				Expect(session).Should(ExitCleanly())
			}(i)
		}
		wg.Wait()
	})

	It("podman rmi --no-prune with dangling parents", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"create", "--name", "c_test1", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"commit", "-q", "c_test1", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--name", "c_test2", "test1", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"commit", "-q", "c_test2", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		imageID2 := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--name", "c_test3", "test2", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"commit", "-q", "c_test3", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		imageID3 := session.OutputToString()

		session = podmanTest.Podman([]string{"untag", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"untag", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"rmi", "-f", "--no-prune", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(imageID3))
		Expect(session.OutputToString()).NotTo(ContainSubstring(imageID2))
	})

	It("podman rmi --no-prune with undangling parents", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"create", "--name", "c_test1", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"commit", "-q", "c_test1", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--name", "c_test2", "test1", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"commit", "-q", "c_test2", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		imageID2 := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--name", "c_test3", "test2", "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"commit", "-q", "c_test3", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		imageID3 := session.OutputToString()

		session = podmanTest.Podman([]string{"rmi", "-f", "--no-prune", "test3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(imageID3))
		Expect(session.OutputToString()).NotTo(ContainSubstring(imageID2))
	})
})
