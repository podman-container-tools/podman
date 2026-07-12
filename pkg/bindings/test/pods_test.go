package bindings_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/pods"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/utils"
)

var _ = Describe("Podman pods", func() {
	var (
		bt     *bindingTest
		s      *gexec.Session
		newpod string
		err    error
	)

	BeforeEach(func() {
		bt = newBindingTest()
		newpod = "newpod"
		bt.RestoreImagesFromCache()
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

	It("inspect pod", func() {
		// Inspect an invalid pod name
		_, err := pods.Inspect(bt.conn, "dummyname", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Inspect a valid pod name
		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.Name).To(Equal(newpod))
	})

	// Test validates the list all api returns
	It("list pod", func() {
		// List all the pods in the current instance
		podSummary, err := pods.List(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(podSummary).To(HaveLen(1))

		// Start the pod
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())

		// Adding an alpine container to the existing pod
		_, err = bt.RunTopContainer(nil, &newpod)
		Expect(err).ToNot(HaveOccurred())
		podSummary, err = pods.List(bt.conn, nil)
		// Verify no errors.
		Expect(err).ToNot(HaveOccurred())
		// Verify number of containers in the pod.
		Expect(podSummary[0].Containers).To(HaveLen(2))

		// Add multiple pods and verify them by name and size.
		newpod2 := "newpod2"
		bt.Podcreate(&newpod2)
		podSummary, err = pods.List(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred(), "Error from pods.List")
		Expect(podSummary).To(HaveLen(2))
		var names []string
		for _, i := range podSummary {
			names = append(names, i.Name)
		}
		Expect(slices.Contains(names, newpod)).To(BeTrue())
		Expect(slices.Contains(names, "newpod2")).To(BeTrue())
	})

	// The test validates the list pod endpoint with passing filters as the params.
	It("List pods with filters", func() {
		newpod2 := "newpod2"
		bt.Podcreate(&newpod2)

		// Start the pod
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())

		_, err = bt.RunTopContainer(nil, &newpod)
		Expect(err).ToNot(HaveOccurred())

		// Expected err with invalid filter params
		filters := make(map[string][]string)
		filters["dummy"] = []string{"dummy"}
		options := new(pods.ListOptions).WithFilters(filters)
		filteredPods, err := pods.List(bt.conn, options)
		Expect(err).To(HaveOccurred())
		Expect(filteredPods).To(BeEmpty(), "len(filteredPods)")
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		// Expected empty response with invalid filters
		filters = make(map[string][]string)
		filters["name"] = []string{"dummy"}
		options = new(pods.ListOptions).WithFilters(filters)
		filteredPods, err = pods.List(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(filteredPods).To(BeEmpty())

		// Validate list pod with name filter
		filters = make(map[string][]string)
		filters["name"] = []string{newpod2}
		options = new(pods.ListOptions).WithFilters(filters)
		filteredPods, err = pods.List(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(filteredPods).To(HaveLen(1))
		var names []string
		for _, i := range filteredPods {
			names = append(names, i.Name)
		}
		Expect(slices.Contains(names, "newpod2")).To(BeTrue())

		// Validate list pod with id filter
		filters = make(map[string][]string)
		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		id := response.ID
		filters["id"] = []string{id}
		options = new(pods.ListOptions).WithFilters(filters)
		filteredPods, err = pods.List(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(filteredPods).To(HaveLen(1))
		names = names[:0]
		for _, i := range filteredPods {
			names = append(names, i.Name)
		}
		Expect(slices.Contains(names, "newpod")).To(BeTrue())

		// Using multiple filters
		filters["name"] = []string{newpod}
		options = new(pods.ListOptions).WithFilters(filters)
		filteredPods, err = pods.List(bt.conn, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(filteredPods).To(HaveLen(1))
		names = names[:0]
		for _, i := range filteredPods {
			names = append(names, i.Name)
		}
		Expect(slices.Contains(names, "newpod")).To(BeTrue())
	})

	// The test validates if the exists responds
	It("exists pod", func() {
		response, err := pods.Exists(bt.conn, "dummyName", nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response).To(BeFalse())

		// Should exit with no error and response should be true
		response, err = pods.Exists(bt.conn, "newpod", nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response).To(BeTrue())
	})

	// This test validates if All running containers within
	// each specified pod are paused and unpaused
	It("pause unpause pod", func() {
		// TODO fix this
		Skip("Pod behavior is jacked right now.")
		// Pause invalid container
		_, err := pods.Pause(bt.conn, "dummyName", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Adding an alpine container to the existing pod
		_, err = bt.RunTopContainer(nil, &newpod)
		Expect(err).ToNot(HaveOccurred())

		// Binding needs to be modified to inspect the pod state.
		// Since we don't have a pod state we inspect the states of the containers within the pod.
		// Pause a valid container
		_, err = pods.Pause(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.State).To(Equal(define.PodStatePaused))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStatePaused))
		}

		// Unpause a valid container
		_, err = pods.Unpause(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		response, err = pods.Inspect(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.State).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}
	})

	It("start pod with port conflict", func() {
		randomport, err := utils.GetRandomPort()
		Expect(err).ToNot(HaveOccurred())

		portPublish := fmt.Sprintf("%d:%d", randomport, randomport)
		podwithport := "newpodwithport"
		bt.PodcreateAndExpose(&podwithport, &portPublish)

		// Start pod and expose port 12345
		_, err = pods.Start(bt.conn, podwithport, nil)
		Expect(err).ToNot(HaveOccurred())

		// Start another pod and expose same port 12345
		podwithport2 := "newpodwithport2"
		bt.PodcreateAndExpose(&podwithport2, &portPublish)

		_, err = pods.Start(bt.conn, podwithport2, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusConflict))
		Expect(err).To(BeAssignableToTypeOf(&errorhandling.PodConflictErrorModel{}))
	})

	It("start stop restart pod", func() {
		// Start an invalid pod
		_, err = pods.Start(bt.conn, "dummyName", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Stop an invalid pod
		_, err = pods.Stop(bt.conn, "dummyName", nil)
		Expect(err).To(HaveOccurred())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Restart an invalid pod
		_, err = pods.Restart(bt.conn, "dummyName", nil)
		Expect(err).To(HaveOccurred())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Start a valid pod and inspect status of each container
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())

		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.State).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}

		// Start an already running  pod
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())

		// Stop the running pods
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		response, _ = pods.Inspect(bt.conn, newpod, nil)
		Expect(response.State).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateExited))
		}

		// Stop an already stopped pod
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())

		_, err = pods.Restart(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		response, _ = pods.Inspect(bt.conn, newpod, nil)
		Expect(response.State).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}
	})

	// Test to validate all the pods in the stopped/exited state are pruned successfully.
	It("prune pod", func() {
		// Add a new pod
		newpod2 := "newpod2"
		bt.Podcreate(&newpod2)
		// No pods pruned since no pod in exited state
		pruneResponse, err := pods.Prune(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(pruneResponse).To(BeEmpty(), "len(pruneResponse)")
		podSummary, err := pods.List(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(podSummary).To(HaveLen(2))

		// Prune only one pod which is in exited state.
		// Start then stop a pod.
		// pod moves to exited state one pod should be pruned now.
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.State).To(Equal(define.PodStateExited))
		pruneResponse, err = pods.Prune(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(pruneResponse).To(HaveLen(1), "len(pruneResponse)")
		// Validate status and record pod id of pod to be pruned
		Expect(response.State).To(Equal(define.PodStateExited))
		podID := response.ID
		// Check if right pod was pruned
		Expect(pruneResponse).To(HaveLen(1))
		Expect(pruneResponse[0].Id).To(Equal(podID))
		// One pod is pruned hence only one pod should be active.
		podSummary, err = pods.List(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(podSummary).To(HaveLen(1))

		// Test prune multiple pods.
		bt.Podcreate(&newpod)
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = pods.Start(bt.conn, newpod2, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		response, err = pods.Inspect(bt.conn, newpod, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.State).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateExited))
		}
		_, err = pods.Stop(bt.conn, newpod2, nil)
		Expect(err).ToNot(HaveOccurred())
		response, err = pods.Inspect(bt.conn, newpod2, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.State).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateExited))
		}
		_, err = pods.Prune(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		podSummary, err = pods.List(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(podSummary).To(BeEmpty())
	})

	It("simple create pod", func() {
		ps := entities.PodSpec{PodSpecGen: specgen.PodSpecGenerator{InfraContainerSpec: &specgen.SpecGenerator{}}}
		ps.PodSpecGen.Name = "foobar"
		_, err := pods.CreatePodFromSpec(bt.conn, &ps)
		Expect(err).ToNot(HaveOccurred())

		exists, err := pods.Exists(bt.conn, "foobar", nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	// Test validates the pod top bindings
	It("pod top", func() {
		name := "podA"

		bt.Podcreate(&name)
		_, err := pods.Start(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// By name
		_, err = pods.Top(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// With descriptors
		options := new(pods.TopOptions).WithDescriptors([]string{"user,pid,hpid"})
		output, err := pods.Top(bt.conn, name, options)
		Expect(err).ToNot(HaveOccurred())
		header := strings.Split(output[0], "\t")
		for _, d := range []string{"USER", "PID", "HPID"} {
			Expect(d).To(BeElementOf(header))
		}

		// With bogus ID
		_, err = pods.Top(bt.conn, "IdoNotExist", nil)
		Expect(err).To(HaveOccurred())

		// With bogus descriptors
		options = new(pods.TopOptions).WithDescriptors([]string{"Me,Neither"})
		_, err = pods.Top(bt.conn, name, options)
		Expect(err).To(HaveOccurred())
	})
})
