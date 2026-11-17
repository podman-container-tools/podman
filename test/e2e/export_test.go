//go:build linux || freebsd

package integration

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	It("podman export preserves ownership with keep-id user namespaces", func() {
		SkipIfNotRootless("rootless --userns=keep-id stores host-mapped IDs in the container filesystem")

		// Mirrors the reproducer of #29856: an image with a dedicated
		// user (UID/GID 1000), a keep-id container that is created but
		// never started, and an export whose tar headers must show
		// container ownership (user 1000/1000, root-owned parents).
		imageName := "test-export-keepid-user-image"
		containerfile := fmt.Sprintf("FROM %s\nRUN adduser -D -u 1000 user && touch /home/user/newfile && chown -R 1000:1000 /home/user\n", ALPINE)
		podmanTest.BuildImage(containerfile, imageName, "false", "--network=none")

		create := podmanTest.PodmanExitCleanly("create", "--userns=keep-id", imageName)
		cid := strings.TrimSpace(create.OutputToString())

		outfile := filepath.Join(podmanTest.TempDir, "keep-id-export.tar")
		podmanTest.PodmanExitCleanly("export", "-o", outfile, cid)

		file, err := os.Open(outfile)
		Expect(err).ToNot(HaveOccurred())
		defer file.Close()

		reader := tar.NewReader(file)
		owners := map[string]string{}
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			Expect(err).ToNot(HaveOccurred())
			owners[strings.TrimSuffix(header.Name, "/")] = fmt.Sprintf("%d/%d", header.Uid, header.Gid)
		}

		Expect(owners["home"]).To(Equal("0/0"), "home/ should be owned by root in the exported tar")
		Expect(owners["home/user"]).To(Equal("1000/1000"), "home/user should keep container ownership in the exported tar")
		Expect(owners["home/user/newfile"]).To(Equal("1000/1000"), "files under home/user should keep container ownership in the exported tar")
	})
})
