package bindings_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
	podmanRegistry "go.podman.io/podman/v6/hack/podman-registry-go"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var _ = Describe("Podman images", func() {
	var (
		// tempdir    string
		// err        error
		// podmanTest *PodmanTestIntegration
		bt  *bindingTest
		s   *Session
		err error
	)

	BeforeEach(func() {
		// tempdir, err = CreateTempDirInTempDir()
		// if err != nil {
		//	os.Exit(1)
		// }
		// podmanTest = PodmanTestCreate(tempdir)
		// podmanTest.Setup()
		// podmanTest.SeedImages()
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		// podmanTest.Cleanup()
		// f := CurrentSpecReport()
		// processTestResult(f)
		s.Kill()
		bt.cleanup()
	})

	It("inspect image", func() {
		// Inspect invalid image be 404
		_, err = images.GetImage(bt.conn, "foobar5000", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Inspect by short name
		data, err := images.GetImage(bt.conn, alpine.shortName, nil)
		Expect(err).ToNot(HaveOccurred())

		// Inspect with full ID
		_, err = images.GetImage(bt.conn, data.ID, nil)
		Expect(err).ToNot(HaveOccurred())

		// Inspect with partial ID
		_, err = images.GetImage(bt.conn, data.ID[0:12], nil)
		Expect(err).ToNot(HaveOccurred())

		// Inspect by long name
		_, err = images.GetImage(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		// TODO it looks like the images API always returns size regardless
		// of bool or not. What should we do ?
		// Expect(data.Size).To(BeZero())

		options := new(images.GetOptions).WithSize(true)
		// Enabling the size parameter should result in size being populated
		data, err = images.GetImage(bt.conn, alpine.name, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(data.Size).To(BeNumerically(">", 0))
	})

	// Test to validate the remove image api
	It("remove image", func() {
		// NOTE that removing an image that does not exist will still
		// return a 200 http status.  The response, however, includes
		// the exit code that podman-remote should exit with.
		//
		// The libpod/images/remove endpoint supports batch removal of
		// images for performance reasons and for hiding the logic of
		// deciding which exit code to use from the client.
		response, errs := images.Remove(bt.conn, []string{"foobar5000"}, nil)
		Expect(errs).ToNot(BeEmpty())
		Expect(response.ExitCode).To(BeNumerically("==", 1)) // podman-remote would exit with 1

		// Remove an image by name, validate image is removed and error is nil
		inspectData, err := images.GetImage(bt.conn, busybox.shortName, nil)
		Expect(err).ToNot(HaveOccurred())
		response, errs = images.Remove(bt.conn, []string{busybox.shortName}, nil)
		Expect(errs).To(BeEmpty())

		Expect(inspectData.ID).To(Equal(response.Deleted[0]))
		_, err = images.GetImage(bt.conn, busybox.shortName, nil)
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Start a container with alpine image
		top := "top"
		_, err = bt.RunTopContainer(&top, nil)
		Expect(err).ToNot(HaveOccurred())
		// we should now have a container called "top" running
		containerResponse, err := containers.Inspect(bt.conn, "top", nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(containerResponse.Name).To(Equal("top"))

		// try to remove the image "alpine". This should fail since we are not force
		// deleting hence image cannot be deleted until the container is deleted.
		_, errs = images.Remove(bt.conn, []string{alpine.shortName}, nil)
		code, _ = bindings.CheckResponseCode(errs[0])
		Expect(code).To(BeNumerically("==", -1))

		// Removing the image "alpine" where force = true
		options := new(images.RemoveOptions).WithForce(true)
		_, errs = images.Remove(bt.conn, []string{alpine.shortName}, options)
		Expect(errs).To(Or(HaveLen(0), BeNil()))
		// To be extra sure, check if the previously created container
		// is gone as well.
		_, err = containers.Inspect(bt.conn, "top", nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Now make sure both images are gone.
		_, err = images.GetImage(bt.conn, busybox.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		_, err = images.GetImage(bt.conn, alpine.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	// Tests to validate the image tag command.
	It("tag image", func() {
		// Validates if invalid image name is given a bad response is encountered.
		err = images.Tag(bt.conn, "dummy", "demo", alpine.shortName, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Validates if the image is tagged successfully.
		err = images.Tag(bt.conn, alpine.shortName, "demo", alpine.shortName, nil)
		Expect(err).ToNot(HaveOccurred())

		// Validates if name updates when the image is retagged.
		_, err := images.GetImage(bt.conn, "alpine:demo", nil)
		Expect(err).ToNot(HaveOccurred())
	})

	// Test to validate the List images command.
	It("List image", func() {
		// Array to hold the list of images returned
		imageSummary, err := images.List(bt.conn, nil)
		// There Should be no errors in the response.
		Expect(err).ToNot(HaveOccurred())
		// Since in the begin context two images are created the
		// list context should have only 2 images
		Expect(imageSummary).To(HaveLen(2))

		// Adding one more image. There Should be no errors in the response.
		// And the count should be three now.
		bt.Pull("testimage:20221018")
		imageSummary, err = images.List(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(imageSummary)).To(BeNumerically(">=", 2))

		// Validate the image names.
		var names []string
		for _, i := range imageSummary {
			names = append(names, i.RepoTags...)
		}
		Expect(names).To(ContainElement(alpine.name))
		Expect(names).To(ContainElement(busybox.name))

		// List  images with a filter
		filters := make(map[string][]string)
		filters["reference"] = []string{alpine.name}
		options := new(images.ListOptions).WithFilters(filters).WithAll(false)
		filteredImages, err := images.List(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(filteredImages).To(HaveLen(1))

		// List  images with a bad filter
		filters["name"] = []string{alpine.name}
		options = new(images.ListOptions).WithFilters(filters)
		_, err = images.List(bt.conn, options)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("Image Exists", func() {
		// exists on bogus image should be false, with no error
		exists, err := images.Exists(bt.conn, "foobar", nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())

		// exists with shortname should be true
		exists, err = images.Exists(bt.conn, alpine.shortName, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// exists with fqname should be true
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("Load|Import Image", func() {
		// load an image
		_, errs := images.Remove(bt.conn, []string{alpine.name}, nil)
		Expect(errs).To(BeEmpty())
		exists, err := images.Exists(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
		f, err := os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		names, err := images.Load(bt.conn, f)
		Expect(err).ToNot(HaveOccurred())
		Expect(names.Names[0]).To(Equal(alpine.name))
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// load with a repo name
		f, err = os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		Expect(err).ToNot(HaveOccurred())
		_, errs = images.Remove(bt.conn, []string{alpine.name}, nil)
		Expect(errs).To(BeEmpty())
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
		names, err = images.Load(bt.conn, f)
		Expect(err).ToNot(HaveOccurred())
		Expect(names.Names[0]).To(Equal(alpine.name))

		// load with a bad repo name should trigger a 500
		_, errs = images.Remove(bt.conn, []string{alpine.name}, nil)
		Expect(errs).To(BeEmpty())
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("Export Image", func() {
		// Export an image
		exportPath := filepath.Join(bt.tempDirPath, alpine.tarballName)
		w, err := os.Create(filepath.Join(bt.tempDirPath, alpine.tarballName))
		Expect(err).ToNot(HaveOccurred())
		defer w.Close()
		err = images.Export(bt.conn, []string{alpine.name}, w, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = os.Stat(exportPath)
		Expect(err).ToNot(HaveOccurred())

		// TODO how do we verify that a format change worked?
	})

	It("Import Image", func() {
		// load an image
		_, errs := images.Remove(bt.conn, []string{alpine.name}, nil)
		Expect(errs).To(BeEmpty())
		exists, err := images.Exists(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
		f, err := os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		changes := []string{"CMD /bin/foobar"}
		testMessage := "test_import"
		options := new(images.ImportOptions).WithMessage(testMessage).WithChanges(changes).WithReference(alpine.name)
		_, err = images.Import(bt.conn, f, options)
		Expect(err).ToNot(HaveOccurred())
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
		data, err := images.GetImage(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(data.Comment).To(Equal(testMessage))
	})

	It("History Image", func() {
		// a bogus name should return a 404
		_, err := images.History(bt.conn, "foobar", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		var foundID bool
		data, err := images.GetImage(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		history, err := images.History(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		for _, i := range history {
			if i.ID == data.ID {
				foundID = true
				break
			}
		}
		Expect(foundID).To(BeTrue())
	})

	It("Search for an image", func() {
		reports, err := images.Search(bt.conn, "alpine", nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(reports)).To(BeNumerically(">", 1))
		var foundAlpine bool
		for _, i := range reports {
			if i.Name == "docker.io/library/alpine" {
				foundAlpine = true
				break
			}
		}
		Expect(foundAlpine).To(BeTrue())

		// Search for alpine with a limit of 10
		options := new(images.SearchOptions).WithLimit(10)
		reports, err = images.Search(bt.conn, "docker.io/alpine", options)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(reports)).To(BeNumerically("<=", 10))

		filters := make(map[string][]string)
		filters["stars"] = []string{"100"}
		// Search for alpine with stars greater than 100
		options = new(images.SearchOptions).WithFilters(filters)
		reports, err = images.Search(bt.conn, "docker.io/alpine", options)
		Expect(err).ToNot(HaveOccurred())
		for _, i := range reports {
			Expect(i.Stars).To(BeNumerically(">=", 100))
		}

		//	Search with a fqdn
		reports, err = images.Search(bt.conn, "quay.io/podman/stable", nil)
		Expect(err).ToNot(HaveOccurred(), "Error in images.Search()")
		Expect(reports).ToNot(BeEmpty())
	})

	It("Prune images", func() {
		options := new(images.PruneOptions).WithAll(true)
		results, err := images.Prune(bt.conn, options)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).ToNot(BeEmpty())
	})

	// TODO: we really need to extent to pull tests once we have a more sophisticated CI.
	It("Image Pull", func() {
		rawImage := "docker.io/library/busybox:latest"

		var writer bytes.Buffer
		pullOpts := new(images.PullOptions).WithProgressWriter(&writer)
		pulledImages, err := images.Pull(bt.conn, rawImage, pullOpts)
		Expect(err).NotTo(HaveOccurred())
		Expect(pulledImages).To(HaveLen(1))
		output := writer.String()
		Expect(output).To(ContainSubstring("Trying to pull "))
		Expect(output).To(ContainSubstring("Getting image source signatures"))

		exists, err := images.Exists(bt.conn, rawImage, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Make sure the normalization AND the full-transport reference works.
		_, err = images.Pull(bt.conn, "docker://"+rawImage, nil)
		Expect(err).NotTo(HaveOccurred())

		// The v2 endpoint only supports the docker transport.  Let's see if that's really true.
		_, err = images.Pull(bt.conn, "bogus-transport:bogus.com/image:reference", nil)
		Expect(err).To(HaveOccurred())
	})

	It("Image Push", func() {
		registryOptions := &podmanRegistry.Options{
			PodmanPath: getPodmanBinary(),
		}
		registry, err := podmanRegistry.StartWithOptions(registryOptions)
		Expect(err).ToNot(HaveOccurred())

		var writer bytes.Buffer
		pushOpts := new(images.PushOptions).WithUsername(registry.User).WithPassword(registry.Password).WithSkipTLSVerify(true).WithProgressWriter(&writer).WithQuiet(false)
		err = images.Push(bt.conn, alpine.name, fmt.Sprintf("localhost:%s/test:latest", registry.Port), pushOpts)
		Expect(err).ToNot(HaveOccurred())

		output := writer.String()
		Expect(output).To(ContainSubstring("Copying blob "))
		Expect(output).To(ContainSubstring("Copying config "))
		Expect(output).To(ContainSubstring("Writing manifest to image destination"))
	})

	It("Build no options", func() {
		results, err := images.Build(bt.conn, []string{"fixture/Containerfile"}, entities.BuildOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(*results).To(MatchFields(IgnoreMissing, Fields{
			"ID":         Not(BeEmpty()),
			"SaveFormat": ContainSubstring(define.OCIArchive),
		}))
	})
})
