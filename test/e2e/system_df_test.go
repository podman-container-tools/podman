//go:build linux || freebsd

package integration

import (
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("podman system df", func() {
	It("podman system df", func() {
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// run two containers with volumes to create something in the volume
		session = podmanTest.Podman([]string{"run", "-v", "data1:/data", "--name", "container1", BB, "sh", "-c", "echo test > /data/1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-v", "data2:/data", "--name", "container2", BB, "sh", "-c", "echo test > /data/1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// remove one container, we keep the volume
		session = podmanTest.Podman([]string{"rm", "container2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"images", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		totImages := strconv.Itoa(len(session.OutputToStringArray()))

		session = podmanTest.Podman([]string{"system", "df"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(4))
		images := strings.Fields(session.OutputToStringArray()[1])
		containers := strings.Fields(session.OutputToStringArray()[2])
		volumes := strings.Fields(session.OutputToStringArray()[3])
		Expect(images[1]).To(Equal(totImages), "total images expected")
		Expect(containers[1]).To(Equal("2"), "total containers expected")
		Expect(volumes[2]).To(Equal("2"), "total volumes expected")
		Expect(volumes[6]).To(Equal("(50%)"), "percentage usage expected")

		session = podmanTest.Podman([]string{"rm", "container1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"system", "df"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volumes = strings.Fields(session.OutputToStringArray()[3])
		// percentages on volumes were being calculated incorrectly. Make sure we only report 100% and not above
		Expect(volumes[6]).To(Equal("(100%)"), "percentage usage expected")
	})

	It("podman system df image with no tag", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"image", "untag", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"system", "df"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman system df --format \"{{ json . }}\"", func() {
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"system", "df", "--format", "{{ json . }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Size"))
		Expect(session.OutputToString()).To(ContainSubstring("Reclaimable"))

		// Note: {{ json . }} returns one json object per line, this matches docker!
		for i, out := range session.OutputToStringArray() {
			Expect(out).To(BeValidJSON(), "line %d failed to be parsed", i)
		}
	})

	It("podman system df --format with --verbose", func() {
		session := podmanTest.Podman([]string{"system", "df", "--format", "json", "--verbose"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "Error: cannot combine --format and --verbose flags"))
	})

	It("podman system df --format json", func() {
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"system", "df", "--format", "json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Size"))
		Expect(session.OutputToString()).To(ContainSubstring("Reclaimable"))
		Expect(session.OutputToString()).To(BeValidJSON())
	})
})
