package bindings_test

import (
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/bindings/system"
	"go.podman.io/podman/v6/pkg/specgen"
)

var _ = Describe("Podman info", func() {
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

	It("podman info", func() {
		info, err := system.Info(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Host.Arch).To(Equal(runtime.GOARCH))
		Expect(info.Host.OS).To(Equal(runtime.GOOS))
		listOptions := new(images.ListOptions)
		i, err := images.List(bt.conn, listOptions.WithAll(true))
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Store.ImageStore.Number).To(Equal(len(i)))
	})

	It("podman info container counts", func() {
		s := specgen.NewSpecGenerator(alpine.name, false)
		_, err := containers.CreateWithSpec(bt.conn, s, nil)
		Expect(err).ToNot(HaveOccurred())

		idPause, err := bt.RunTopContainer(nil, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, idPause, nil)
		Expect(err).ToNot(HaveOccurred())

		idStop, err := bt.RunTopContainer(nil, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, idStop, nil)
		Expect(err).ToNot(HaveOccurred())

		_, err = bt.RunTopContainer(nil, nil)
		Expect(err).ToNot(HaveOccurred())

		info, err := system.Info(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(info.Store.ContainerStore.Number).To(BeNumerically("==", 4))
		Expect(info.Store.ContainerStore.Paused).To(Equal(1))
		Expect(info.Store.ContainerStore.Stopped).To(Equal(2))
		Expect(info.Store.ContainerStore.Running).To(Equal(1))
	})
})
