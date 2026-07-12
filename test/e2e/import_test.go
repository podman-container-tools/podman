//go:build linux || freebsd

package integration

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman import", func() {
	It("podman import with source and reference", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(ExitCleanly())

		importImage := podmanTest.Podman([]string{"import", outfile, "foobar.com/imported-image:latest"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(Exit(0))
		if !IsRemote() {
			messages := importImage.ErrorToString()
			Expect(messages).Should(ContainSubstring("Getting image source signatures"))
			Expect(messages).Should(ContainSubstring("Copying blob"))
			Expect(messages).Should(ContainSubstring("Writing manifest to image destination"))
			Expect(messages).Should(Not(ContainSubstring("level=")), "Unexpected logrus messages in stderr")
		}

		results := podmanTest.Podman([]string{"inspect", "--type", "image", "foobar.com/imported-image:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
	})

	It("podman import with custom os, arch and variant", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(ExitCleanly())

		importImage := podmanTest.Podman([]string{"import", "-q", "--os", "testos", "--arch", "testarch", outfile, "foobar.com/imported-image:latest"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"inspect", "--type", "image", "foobar.com/imported-image:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		Expect(results.OutputToString()).To(ContainSubstring("testos"))
		Expect(results.OutputToString()).To(ContainSubstring("testarch"))
	})

	It("podman import without reference", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(ExitCleanly())

		importImage := podmanTest.Podman([]string{"import", "-q", outfile})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(ExitCleanly())

		// tag the image which proves it is in R/W storage
		tag := podmanTest.Podman([]string{"tag", importImage.OutputToString(), "foo"})
		tag.WaitWithDefaultTimeout()
		Expect(tag).Should(ExitCleanly())
	})

	It("podman import with message flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(ExitCleanly())

		importImage := podmanTest.Podman([]string{"import", "-q", "--message", "importing container test message", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"history", "imported-image", "--format", "{{.Comment}}"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		Expect(results.OutputToStringArray()).To(ContainElement(HavePrefix("importing container test message")))
	})

	It("podman import with change flag CMD=<path>", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(ExitCleanly())

		importImage := podmanTest.Podman([]string{"import", "-q", "--change", "CMD=/bin/bash", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"inspect", "imported-image"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		imageData := results.InspectImageJSON()
		Expect(imageData[0].Config.Cmd[0]).To(Equal("/bin/sh"))
		Expect(imageData[0].Config.Cmd[1]).To(Equal("-c"))
		Expect(imageData[0].Config.Cmd[2]).To(Equal("/bin/bash"))
	})

	It("podman import with change flag CMD <path>", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(ExitCleanly())

		importImage := podmanTest.Podman([]string{"import", "-q", "--change", "CMD /bin/sh", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"inspect", "imported-image"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		imageData := results.InspectImageJSON()
		Expect(imageData[0].Config.Cmd[0]).To(Equal("/bin/sh"))
		Expect(imageData[0].Config.Cmd[1]).To(Equal("-c"))
		Expect(imageData[0].Config.Cmd[2]).To(Equal("/bin/sh"))
	})

	It("podman import with change flag CMD [\"path\",\"path'\"", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(ExitCleanly())

		importImage := podmanTest.Podman([]string{"import", "-q", "--change", "CMD [\"/bin/bash\"]", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"inspect", "imported-image"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		imageData := results.InspectImageJSON()
		Expect(imageData[0].Config.Cmd[0]).To(Equal("/bin/bash"))
	})

	It("podman import with signature", func() {
		SkipIfRemote("--signature-policy N/A for remote")

		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(ExitCleanly())

		importImage := podmanTest.Podman([]string{"import", "-q", "--signature-policy", "/no/such/file", outfile})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).To(ExitWithError(125, "open /no/such/file: no such file or directory"))

		result := podmanTest.Podman([]string{"import", "-q", "--signature-policy", "/etc/containers/policy.json", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})
})
