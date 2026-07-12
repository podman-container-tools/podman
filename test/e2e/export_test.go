//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman export", func() {
	It("podman export output flag", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		result := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		_, err := os.Stat(outfile)
		Expect(err).ToNot(HaveOccurred())

		err = os.Remove(outfile)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman container export output flag", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		result := podmanTest.Podman([]string{"container", "export", "-o", outfile, cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		_, err := os.Stat(outfile)
		Expect(err).ToNot(HaveOccurred())

		err = os.Remove(outfile)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman export bad filename", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		outfile := filepath.Join(podmanTest.TempDir, "container:with:colon.tar")
		result := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, "invalid filename (should not contain ':')"))
	})

	It("podman export emits export event", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		result := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		eventsResult := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=export", "--since", "30s"})
		eventsResult.WaitWithDefaultTimeout()
		Expect(eventsResult).Should(ExitCleanly())
		events := eventsResult.OutputToStringArray()
		Expect(events).ToNot(BeEmpty(), "export event should be present")
		Expect(events[0]).To(ContainSubstring("export"))
		Expect(events[0]).To(ContainSubstring(cid))
	})
})
