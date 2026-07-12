package bindings_test

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	podmanRegistry "go.podman.io/podman/v6/hack/podman-registry-go"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/bindings/manifests"
)

var _ = Describe("Podman manifests", func() {
	var (
		bt *bindingTest
		s  *gexec.Session
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("create", func() {
		// create manifest list without images
		id, err := manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{}, nil)
		Expect(err).ToNot(HaveOccurred(), err)
		list, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(list.Manifests).To(BeEmpty())

		// creating a duplicate should fail as a 500
		_, err = manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", nil, nil)
		Expect(err).To(HaveOccurred())

		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		_, errs := images.Remove(bt.conn, []string{id}, nil)
		Expect(errs).To(BeEmpty())

		// create manifest list with images
		id, err = manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{alpine.name}, nil)
		Expect(err).ToNot(HaveOccurred())

		list, err = manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(list.Manifests).To(HaveLen(1))
	})

	It("delete manifest", func() {
		id, err := manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{}, nil)
		Expect(err).ToNot(HaveOccurred(), err)
		list, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(list.Manifests).To(BeEmpty())

		removeReport, err := manifests.Delete(bt.conn, "quay.io/libpod/foobar:latest")
		Expect(err).ToNot(HaveOccurred())
		Expect(removeReport.Deleted).To(HaveLen(1))
	})

	It("inspect", func() {
		_, err := manifests.Inspect(bt.conn, "larry", nil)
		Expect(err).To(HaveOccurred())

		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("add", func() {
		// add to bogus should 404
		_, err := manifests.Add(bt.conn, "foobar", nil)
		Expect(err).To(HaveOccurred())

		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound), err.Error())

		id, err := manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{}, nil)
		Expect(err).ToNot(HaveOccurred())

		options := new(manifests.AddOptions).WithImages([]string{alpine.name})
		_, err = manifests.Add(bt.conn, id, options)
		Expect(err).ToNot(HaveOccurred())

		list, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(list.Manifests).To(HaveLen(1))

		// add bogus name to existing list should fail
		options.WithImages([]string{"larry"})
		_, err = manifests.Add(bt.conn, id, options)
		Expect(err).To(HaveOccurred())

		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusBadRequest))
	})

	It("remove digest", func() {
		// removal on bogus manifest list should be 404
		_, err := manifests.Remove(bt.conn, "larry", "1234", nil)
		Expect(err).To(HaveOccurred())

		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		id, err := manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{alpine.name}, nil)
		Expect(err).ToNot(HaveOccurred())

		data, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(data.Manifests).To(HaveLen(1))

		// removal on a good manifest list with a bad digest should be 400
		_, err = manifests.Remove(bt.conn, id, "!234", nil)
		Expect(err).To(HaveOccurred())

		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusBadRequest))

		digest := data.Manifests[0].Digest.String()
		_, err = manifests.Remove(bt.conn, id, digest, nil)
		Expect(err).ToNot(HaveOccurred())

		// removal on good manifest with good digest should work
		data, err = manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(data.Manifests).Should(BeEmpty())
	})

	It("annotate", func() {
		id, err := manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{}, nil)
		Expect(err).ToNot(HaveOccurred())

		opts := manifests.AddOptions{Images: []string{"quay.io/libpod/alpine:latest"}}

		_, err = manifests.Add(bt.conn, id, &opts)
		Expect(err).ToNot(HaveOccurred())

		data, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(data.Manifests).To(HaveLen(1))

		digest := data.Manifests[0].Digest.String()
		annoOpts := new(manifests.ModifyOptions).WithOS("foo")
		_, err = manifests.Annotate(bt.conn, id, []string{digest}, annoOpts)
		Expect(err).ToNot(HaveOccurred())

		list, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(list.Manifests).To(HaveLen(1))
		Expect(list.Manifests[0].Platform.OS).To(Equal("foo"))
	})

	It("Manifest Push", func() {
		registryOptions := &podmanRegistry.Options{
			PodmanPath: getPodmanBinary(),
		}
		registry, err := podmanRegistry.StartWithOptions(registryOptions)
		Expect(err).ToNot(HaveOccurred())

		name := "quay.io/libpod/foobar:latest"
		_, err = manifests.Create(bt.conn, name, []string{alpine.name}, nil)
		Expect(err).ToNot(HaveOccurred())

		var writer bytes.Buffer
		pushOpts := new(images.PushOptions).WithUsername(registry.User).WithPassword(registry.Password).WithAll(true).WithSkipTLSVerify(true).WithProgressWriter(&writer).WithQuiet(false)
		_, err = manifests.Push(bt.conn, name, fmt.Sprintf("localhost:%s/test:latest", registry.Port), pushOpts)
		Expect(err).ToNot(HaveOccurred())

		output := writer.String()
		Expect(output).To(ContainSubstring("Writing manifest list to image destination"))
		Expect(output).To(ContainSubstring("Storing list signatures"))
	})
})
