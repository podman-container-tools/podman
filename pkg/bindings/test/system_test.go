package bindings_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/pods"
	"go.podman.io/podman/v6/pkg/bindings/system"
	"go.podman.io/podman/v6/pkg/bindings/volumes"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
)

var _ = Describe("Podman system", func() {
	var (
		bt     *bindingTest
		s      *gexec.Session
		newpod string
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		newpod = "newpod"
		bt.Podcreate(&newpod)
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("podman events", func() {
		name := "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())

		filters := make(map[string][]string)
		filters["container"] = []string{name}

		binChan := make(chan entities.Event)
		done := sync.Mutex{}
		done.Lock()
		eventCounter := 0
		go func() {
			defer done.Unlock()
			for range binChan {
				eventCounter++
			}
		}()
		options := new(system.EventsOptions).WithFilters(filters).WithStream(false)
		err = system.Events(bt.conn, binChan, nil, options)
		Expect(err).ToNot(HaveOccurred())
		done.Lock()
		Expect(eventCounter).To(BeNumerically(">", 0))
	})

	It("podman system prune - pod,container stopped", func() {
		// Start and stop a pod to enter in exited state.
		_, err := pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		// Start and stop a container to enter in exited state.
		name := "top"
		_, err = bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		options := new(system.PruneOptions).WithAll(true)
		systemPruneResponse, err := system.Prune(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(systemPruneResponse.PodPruneReport).To(HaveLen(1))
		Expect(systemPruneResponse.ContainerPruneReports).To(HaveLen(1))
		Expect(systemPruneResponse.ImagePruneReports).
			ToNot(BeEmpty())
		Expect(systemPruneResponse.VolumePruneReports).To(BeEmpty())
	})

	It("podman system prune running alpine container", func() {
		// Start and stop a pod to enter in exited state.
		_, err := pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())

		// Start and stop a container to enter in exited state.
		name := "top"
		_, err = bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Start container and leave in running
		name2 := "top2"
		_, err = bt.RunTopContainer(&name2, nil)
		Expect(err).ToNot(HaveOccurred())

		// Adding an unused volume
		_, err = volumes.Create(bt.conn, entities.VolumeCreateOptions{}, nil)
		Expect(err).ToNot(HaveOccurred())
		options := new(system.PruneOptions).WithAll(true)
		systemPruneResponse, err := system.Prune(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(systemPruneResponse.PodPruneReport).To(HaveLen(1))
		Expect(systemPruneResponse.ContainerPruneReports).To(HaveLen(1))
		Expect(systemPruneResponse.ImagePruneReports).
			ToNot(BeEmpty())
		// Alpine image should not be pruned as used by running container
		Expect(reports.PruneReportsIds(systemPruneResponse.ImagePruneReports)).
			ToNot(ContainElement("docker.io/library/alpine:latest"))
		// Though unused volume is available it should not be pruned as flag set to false.
		Expect(systemPruneResponse.VolumePruneReports).To(BeEmpty())
	})

	It("podman system prune running alpine container volume prune", func() {
		// Start a pod and leave it running
		_, err := pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())

		// Start and stop a container to enter in exited state.
		name := "top"
		_, err = bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Start second container and leave in running
		name2 := "top2"
		_, err = bt.RunTopContainer(&name2, nil)
		Expect(err).ToNot(HaveOccurred())

		// Adding an unused volume should work
		_, err = volumes.Create(bt.conn, entities.VolumeCreateOptions{}, nil)
		Expect(err).ToNot(HaveOccurred())

		options := new(system.PruneOptions).WithAll(true).WithVolumes(true)
		systemPruneResponse, err := system.Prune(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(systemPruneResponse.PodPruneReport).To(BeEmpty())
		Expect(systemPruneResponse.ContainerPruneReports).To(HaveLen(1))
		Expect(systemPruneResponse.ImagePruneReports).
			ToNot(BeEmpty())
		// Alpine image should not be pruned as used by running container
		Expect(reports.PruneReportsIds(systemPruneResponse.ImagePruneReports)).
			ToNot(ContainElement("docker.io/library/alpine:latest"))
		// Volume should be pruned now as flag set true
		Expect(systemPruneResponse.VolumePruneReports).To(HaveLen(1))
	})

	It("podman system prune running alpine container volume prune --filter", func() {
		// Start a pod and leave it running
		_, err := pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())

		// Start and stop a container to enter in exited state.
		name := "top"
		_, err = bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Start second container and leave in running
		name2 := "top2"
		_, err = bt.RunTopContainer(&name2, nil)
		Expect(err).ToNot(HaveOccurred())

		// Adding an unused volume should work
		_, err = volumes.Create(bt.conn, entities.VolumeCreateOptions{}, nil)
		Expect(err).ToNot(HaveOccurred())

		// Adding an unused volume with label should work
		_, err = volumes.Create(bt.conn, entities.VolumeCreateOptions{Label: map[string]string{
			"label1": "value1",
		}}, nil)
		Expect(err).ToNot(HaveOccurred())

		f := make(map[string][]string)
		f["label"] = []string{"label1=idontmatch"}

		options := new(system.PruneOptions).WithAll(true).WithVolumes(true).WithFilters(f)
		systemPruneResponse, err := system.Prune(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(systemPruneResponse.PodPruneReport).To(BeEmpty())
		Expect(systemPruneResponse.ContainerPruneReports).To(BeEmpty())
		Expect(systemPruneResponse.ImagePruneReports).To(BeEmpty())
		// Alpine image should not be pruned as used by running container
		Expect(reports.PruneReportsIds(systemPruneResponse.ImagePruneReports)).
			ToNot(ContainElement("docker.io/library/alpine:latest"))
		// Volume shouldn't be pruned because the PruneOptions filters doesn't match
		Expect(systemPruneResponse.VolumePruneReports).To(BeEmpty())

		// Fix filter and re prune
		f["label"] = []string{"label1=value1"}
		options = new(system.PruneOptions).WithAll(true).WithVolumes(true).WithFilters(f)
		systemPruneResponse, err = system.Prune(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())

		// Volume should be pruned because the PruneOptions filters now match
		Expect(systemPruneResponse.VolumePruneReports).To(HaveLen(1))
	})
})
