//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/docker/go-units"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman images", func() {
	It("podman images", func() {
		session := podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("quay.io/libpod/alpine")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("quay.io/libpod/busybox")))
	})

	It("podman image List", func() {
		session := podmanTest.Podman([]string{"image", "list"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("quay.io/libpod/alpine")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("quay.io/libpod/busybox")))
	})

	It("podman images with multiple tags", func() {
		SkipIfNotAMD64() // List of images is different
		// tag "docker.io/library/alpine:latest" to "foo:{a,b,c}"
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"tag", ALPINE, "foo:a", "foo:b", "foo:c"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// tag "foo:c" to "bar:{a,b}"
		session = podmanTest.Podman([]string{"tag", "foo:c", "bar:a", "bar:b"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// check all previous and the newly tagged images
		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.LineInOutputContainsTag("quay.io/libpod/alpine", "latest")).To(BeTrue())
		Expect(session.LineInOutputContainsTag("quay.io/libpod/busybox", "latest")).To(BeTrue())
		Expect(session.LineInOutputContainsTag("localhost/foo", "a")).To(BeTrue())
		Expect(session.LineInOutputContainsTag("localhost/foo", "b")).To(BeTrue())
		Expect(session.LineInOutputContainsTag("localhost/foo", "c")).To(BeTrue())
		Expect(session.LineInOutputContainsTag("localhost/bar", "a")).To(BeTrue())
		Expect(session.LineInOutputContainsTag("localhost/bar", "b")).To(BeTrue())
		session = podmanTest.Podman([]string{"images", "-qn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES)))
	})

	It("podman images with digests", func() {
		session := podmanTest.Podman([]string{"images", "--digests"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("quay.io/libpod/alpine")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("quay.io/libpod/busybox")))
	})

	It("podman empty images list in JSON format", func() {
		session := podmanTest.Podman([]string{"images", "--format=json", "not-existing-image"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeValidJSON())
	})

	It("podman images in JSON format", func() {
		session := podmanTest.Podman([]string{"images", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		output := session.OutputToString()
		Expect(output).To(BeValidJSON())

		images := []map[string]any{}
		err := json.Unmarshal([]byte(output), &images)

		Expect(err).ToNot(HaveOccurred())
		Expect(images).ToNot(BeEmpty())

		for _, image := range images {
			Expect(image).To(HaveKey("Repository"))
			Expect(image).To(HaveKey("Tag"))
		}
	})

	It("podman images in GO template format", func() {
		formatStr := "{{.ID}}\t{{.Created}}\t{{.CreatedAt}}\t{{.CreatedSince}}\t{{.CreatedTime}}"
		session := podmanTest.Podman([]string{"images", fmt.Sprintf("--format=%s", formatStr)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman images with short options", func() {
		session := podmanTest.Podman([]string{"images", "-qn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman images filter by image name", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		podmanTest.AddImageToRWStore(BB)

		session := podmanTest.Podman([]string{"images", "-q", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		session = podmanTest.Podman([]string{"tag", ALPINE, "foo:a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"tag", BB, "foo:b"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"images", "-q", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman images filter by image ID", func() {
		session := podmanTest.Podman([]string{"inspect", ALPINE, "--format", "{{.ID}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
		imgID := session.OutputToString()

		session = podmanTest.Podman([]string{"images", "--noheading", "--filter", "id=" + imgID[:5]})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman images filter by image digest", func() {
		session := podmanTest.Podman([]string{"inspect", ALPINE, "--format", "{{.Digest}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
		imgDigest := session.OutputToString()

		session = podmanTest.Podman([]string{"images", "--noheading", "--filter", "digest=" + imgDigest[:10]})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman images filter reference", func() {
		SkipIfNotAMD64() // List of images is different
		result := podmanTest.Podman([]string{"images", "-q", "-f", "reference=quay.io/libpod/*"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(HaveLen(9))

		retalpine := podmanTest.Podman([]string{"images", "-f", "reference=*lpine*"})
		retalpine.WaitWithDefaultTimeout()
		Expect(retalpine).Should(ExitCleanly())
		Expect(retalpine.OutputToStringArray()).To(HaveLen(5))
		Expect(retalpine.OutputToString()).To(ContainSubstring("alpine"))

		retalpine = podmanTest.Podman([]string{"images", "-f", "reference=alpine"})
		retalpine.WaitWithDefaultTimeout()
		Expect(retalpine).Should(ExitCleanly())
		Expect(retalpine.OutputToStringArray()).To(HaveLen(2))
		Expect(retalpine.OutputToString()).To(ContainSubstring("alpine"))

		retnone := podmanTest.Podman([]string{"images", "-q", "-f", "reference=bogus"})
		retnone.WaitWithDefaultTimeout()
		Expect(retnone).Should(ExitCleanly())
		Expect(retnone.OutputToStringArray()).To(BeEmpty())
	})

	It("podman images filter before image", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
RUN echo hello > /hello
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "before=foobar.com/before:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman images workingdir from  image", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
WORKDIR /test
`
		podmanTest.BuildImage(dockerfile, "foobar.com/workdir:latest", "false")
		result := podmanTest.Podman([]string{"run", "foobar.com/workdir:latest", "pwd"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("/test"))
	})

	It("podman images filter since/after image", func() {
		dockerfile := `FROM scratch
`
		podmanTest.BuildImage(dockerfile, "foobar.com/one:latest", "false")
		podmanTest.BuildImage(dockerfile, "foobar.com/two:latest", "false")
		podmanTest.BuildImage(dockerfile, "foobar.com/three:latest", "false")

		// `since` filter
		result := podmanTest.PodmanNoCache([]string{"images", "-q", "-f", "since=foobar.com/one:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(HaveLen(2))

		// `after` filter
		result = podmanTest.Podman([]string{"image", "list", "-q", "-f", "after=foobar.com/one:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).Should(HaveLen(2), "list filter output: %q", result.OutputToString())
	})

	It("podman images filter dangling", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "dangling=true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).Should(BeEmpty(), "dangling image output")
	})

	It("podman images filter intermediate", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
		RUN touch /tmp/test.txt
		RUN touch /tmp/test-2.txt
`
		podmanTest.BuildImage(dockerfile, "foobar.com/test-build", "true")
		result := podmanTest.Podman([]string{"images", "--noheading", "--filter", "intermediate=true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman pull by digest and list --all", func() {
		SkipIfNotAMD64() // List of images is different
		// Prevent regressing on issue #7651: error parsing name that includes a digest
		// component as if were a name that includes tag component.
		digestPullAndList := func(noneTag bool) {
			session := podmanTest.Podman([]string{"pull", "-q", ALPINEAMD64DIGEST})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			result := podmanTest.Podman([]string{"images", "--all", ALPINEAMD64DIGEST})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(ExitCleanly())

			if noneTag {
				Expect(result.OutputToString()).To(ContainSubstring("<none>"))
			} else {
				Expect(result.OutputToString()).To(Not(ContainSubstring("<none>")))
			}
		}
		// No "<none>" in the tag column as image tagged as "ALPINE" should be present in
		// the additional image store we're using.  Pull the same image by another name to
		// copy an entry for the image into read-write storage so that the name can be
		// attached to it.
		session := podmanTest.Podman([]string{"pull", "-q", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		digestPullAndList(false)

		// Now remove all names from the read-write image record, re-pull by digest and
		// check for the "<none>" in its listing.
		session = podmanTest.Podman([]string{"untag", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		digestPullAndList(true)
	})

	It("podman check for image with sha256: prefix", func() {
		session := podmanTest.Podman([]string{"inspect", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeValidJSON())
		imageData := session.InspectImageJSON()

		result := podmanTest.Podman([]string{"images", "sha256:" + imageData[0].ID})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman check for image with sha256: prefix", func() {
		session := podmanTest.Podman([]string{"image", "inspect", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeValidJSON())
		imageData := session.InspectImageJSON()

		result := podmanTest.Podman([]string{"image", "ls", fmt.Sprintf("sha256:%s", imageData[0].ID)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman images sort by values", func() {
		sortValueTest := func(value string, result int, format string) []string {
			f := fmt.Sprintf("{{.%s}}", format)
			session := podmanTest.Podman([]string{"images", "--noheading", "--sort", value, "--format", f})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(result))

			return session.OutputToStringArray()
		}

		sortedArr := sortValueTest("created", 0, "CreatedAt")
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] > sortedArr[j] })).To(BeTrue())

		sortedArr = sortValueTest("id", 0, "ID")
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).To(BeTrue())

		sortedArr = sortValueTest("repository", 0, "Repository")
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).To(BeTrue())

		sortedArr = sortValueTest("size", 0, "Size")
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool {
			size1, _ := units.FromHumanSize(sortedArr[i])
			size2, _ := units.FromHumanSize(sortedArr[j])
			return size1 < size2
		})).To(BeTrue())
		sortedArr = sortValueTest("tag", 0, "Tag")
		Expect(sort.SliceIsSorted(sortedArr,
			func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).
			To(BeTrue())

		sortValueTest("badvalue", 125, "Tag")
		sortValueTest("id", 125, "badvalue")
	})

	It("test for issue #6670", func() {
		expected := podmanTest.Podman([]string{"images", "--sort", "created", "--format", "{{.ID}}", "-q"})
		expected.WaitWithDefaultTimeout()

		actual := podmanTest.Podman([]string{"images", "--sort", "created", "-q"})
		actual.WaitWithDefaultTimeout()
		Expect(actual.Out).Should(Equal(expected.Out))
	})

	It("podman images --all flag", func() {
		SkipIfNotAMD64() // List of images is different
		dockerfile := `FROM quay.io/libpod/alpine:latest
RUN mkdir hello
RUN touch test.txt
ENV foo=bar
`
		podmanTest.BuildImage(dockerfile, "test", "true")
		session := podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES) + 2))

		session2 := podmanTest.Podman([]string{"images", "--all"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		Expect(session2.OutputToStringArray()).To(HaveLen(len(CACHE_IMAGES) + 4))
	})

	It("podman images filter by label", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
LABEL version="1.0"
LABEL "com.example.vendor"="Example Vendor"
`
		podmanTest.BuildImage(dockerfile, "test", "true")
		session := podmanTest.Podman([]string{"images", "-f", "label=version=1.0"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman with images with no layers", func() {
		dockerfile := strings.Join([]string{
			`FROM scratch`,
			`LABEL org.opencontainers.image.authors="<somefolks@example.org>"`,
			`LABEL org.opencontainers.image.created=2019-06-11T19:03:37Z`,
			`LABEL org.opencontainers.image.description="This is a test image"`,
			`LABEL org.opencontainers.image.title=test`,
			`LABEL org.opencontainers.image.vendor="Example.org"`,
			`LABEL org.opencontainers.image.version=1`,
		}, "\n")
		podmanTest.BuildImage(dockerfile, "foo", "true")

		session := podmanTest.Podman([]string{"images", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(Not(MatchRegexp("<missing>")))
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"image", "tree", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(MatchRegexp("No Image Layers"))

		session = podmanTest.Podman([]string{"history", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"history", "--quiet", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(6))

		session = podmanTest.Podman([]string{"image", "list", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(Not(MatchRegexp("<missing>")))
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"image", "list"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(Not(MatchRegexp("<missing>")))
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(Not(MatchRegexp("<missing>")))
		Expect(output).To(Not(MatchRegexp("error")))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(Equal("[]"))
	})

	It("podman images --filter label=with,comma", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
`
		podmanTest.BuildImageWithLabel(dockerfile, "foobar.com/before:latest", "false", "test=with,comma")
		result := podmanTest.Podman([]string{"images", "--filter", "label=test=with,comma"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman images --filter readonly", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.Podman([]string{"images", "-f", "readonly=true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result1 := podmanTest.Podman([]string{"images", "--filter", "readonly=false"})
		result1.WaitWithDefaultTimeout()
		Expect(result1).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(Not(Equal(result1.OutputToStringArray())))
	})

	It("podman image prune --filter", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
RUN > file
`
		dockerfile2 := `FROM quay.io/libpod/alpine:latest
RUN > file2
`
		podmanTest.BuildImageWithLabel(dockerfile, "foobar.com/workdir:latest", "false", "abc")
		podmanTest.BuildImageWithLabel(dockerfile2, "foobar.com/workdir:latest", "false", "xyz")
		// --force used to avoid y/n question
		result := podmanTest.Podman([]string{"image", "prune", "--filter", "label=abc", "--force"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(HaveLen(1))

		// check if really abc is removed
		result = podmanTest.Podman([]string{"image", "list", "--filter", "label=abc"})
		Expect(result.OutputToStringArray()).To(BeEmpty())
	})

	It("podman builder prune", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
RUN > file
`
		dockerfile2 := `FROM quay.io/libpod/alpine:latest
RUN > file2
`
		podmanTest.BuildImageWithLabel(dockerfile, "foobar.com/workdir:latest", "false", "abc")
		podmanTest.BuildImageWithLabel(dockerfile2, "foobar.com/workdir:latest", "false", "xyz")
		// --force used to to avoid y/n question
		result := podmanTest.Podman([]string{"builder", "prune", "--filter", "label=abc", "--force"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(HaveLen(1))

		// check if really abc is removed
		result = podmanTest.Podman([]string{"image", "list", "--filter", "label=abc"})
		Expect(result.OutputToStringArray()).To(BeEmpty())
	})

	It("podman images filter should be AND logic", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
LABEL abc=""
LABEL xyz=""
`
		podmanTest.BuildImage(dockerfile, "test-abc-xyz", "true")

		dockerfile2 := `FROM quay.io/libpod/alpine:latest
LABEL xyz="bar"
`
		podmanTest.BuildImage(dockerfile2, "test-xyz", "true")

		session := podmanTest.Podman([]string{"images", "-f", "label=xyz"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToString()).To(ContainSubstring("test-abc-xyz"))
		Expect(session.OutputToString()).To(ContainSubstring("test-xyz"))

		session = podmanTest.Podman([]string{"images", "-f", "label=xyz=bar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test-xyz"))

		session = podmanTest.Podman([]string{"images", "-f", "label=abc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test-abc-xyz"))

		session = podmanTest.Podman([]string{"images", "-f", "label=abc", "-f", "label=xyz"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test-abc-xyz"))

		session = podmanTest.Podman([]string{"images", "-f", "label=xyz=bar", "-f", "label=abc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		session = podmanTest.Podman([]string{"images", "-f", "label=xyz", "-f", "reference=test-abc-xyz"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("test-abc-xyz"))
	})
})
