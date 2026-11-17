package bindings_test

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/image/v5/types"
	podmanRegistry "go.podman.io/podman/v6/hack/podman-registry-go"
	"go.podman.io/podman/v6/pkg/bindings/images"
)

var _ = Describe("Podman images", func() {
	var (
		registry *podmanRegistry.Registry
		bt       *bindingTest
		s        *gexec.Session
		err      error
	)

	BeforeEach(func() {
		registryOptions := &podmanRegistry.Options{
			PodmanPath: getPodmanBinary(),
		}

		// Note: we need to start the registry **before** setting up
		// the test. Otherwise, the registry is not reachable for
		// currently unknown reasons.
		registry, err = podmanRegistry.StartWithOptions(registryOptions)
		Expect(err).ToNot(HaveOccurred())

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
		err := registry.Stop()
		Expect(err).ToNot(HaveOccurred())
	})

	// Test using credentials.
	It("tag + push + pull + search (with credentials)", func() {
		imageRep := "localhost:" + registry.Port + "/test"
		imageTag := "latest"
		imageRef := imageRep + ":" + imageTag

		// Tag the alpine image and verify it has worked.
		err = images.Tag(bt.conn, alpine.shortName, imageTag, imageRep, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = images.GetImage(bt.conn, imageRef, nil)
		Expect(err).ToNot(HaveOccurred())

		// Now push the image.
		pushOpts := new(images.PushOptions)
		err = images.Push(bt.conn, imageRef, imageRef, pushOpts.WithUsername(registry.User).WithPassword(registry.Password).WithSkipTLSVerify(true))
		Expect(err).ToNot(HaveOccurred())

		// Now pull the image.
		pullOpts := new(images.PullOptions)
		_, err = images.Pull(bt.conn, imageRef, pullOpts.WithSkipTLSVerify(true).WithPassword(registry.Password).WithUsername(registry.User))
		Expect(err).ToNot(HaveOccurred())

		// Last, but not least, exercise search.
		searchOptions := new(images.SearchOptions)
		_, err = images.Search(bt.conn, imageRef, searchOptions.WithSkipTLSVerify(true).WithPassword(registry.Password).WithUsername(registry.User))
		Expect(err).ToNot(HaveOccurred())
	})

	// Test using authfile.
	It("tag + push + pull + search (with authfile)", func() {
		imageRep := "localhost:" + registry.Port + "/test"
		imageTag := "latest"
		imageRef := imageRep + ":" + imageTag

		// Create a temporary authentication file.
		tmpFile, err := os.CreateTemp("", "auth.json.")
		Expect(err).ToNot(HaveOccurred())
		_, err = tmpFile.Write([]byte{'{', '}'})
		Expect(err).ToNot(HaveOccurred())
		err = tmpFile.Close()
		Expect(err).ToNot(HaveOccurred())

		authFilePath := tmpFile.Name()

		// Now login to a) test the credentials and to b) store them in
		// the authfile for later use.
		sys := types.SystemContext{
			AuthFilePath:                authFilePath,
			DockerInsecureSkipTLSVerify: types.OptionalBoolTrue,
		}
		loginOptions := auth.LoginOptions{
			Username: registry.User,
			Password: registry.Password,
			AuthFile: authFilePath,
			Stdin:    os.Stdin,
			Stdout:   os.Stdout,
		}
		err = auth.Login(bt.conn, &sys, &loginOptions, []string{imageRep})
		Expect(err).ToNot(HaveOccurred())

		// Tag the alpine image and verify it has worked.
		err = images.Tag(bt.conn, alpine.shortName, imageTag, imageRep, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = images.GetImage(bt.conn, imageRef, nil)
		Expect(err).ToNot(HaveOccurred())

		// Now push the image.
		pushOpts := new(images.PushOptions)
		err = images.Push(bt.conn, imageRef, imageRef, pushOpts.WithAuthfile(authFilePath).WithSkipTLSVerify(true))
		Expect(err).ToNot(HaveOccurred())

		// Now pull the image.
		pullOpts := new(images.PullOptions)
		_, err = images.Pull(bt.conn, imageRef, pullOpts.WithAuthfile(authFilePath).WithSkipTLSVerify(true))
		Expect(err).ToNot(HaveOccurred())

		// Last, but not least, exercise search.
		searchOptions := new(images.SearchOptions)
		_, err = images.Search(bt.conn, imageRef, searchOptions.WithSkipTLSVerify(true).WithAuthfile(authFilePath))
		Expect(err).ToNot(HaveOccurred())
	})
})
