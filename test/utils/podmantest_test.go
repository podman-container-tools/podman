package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("PodmanTest test", func() {
	var podmanTest *FakePodmanTest

	BeforeEach(func() {
		podmanTest = FakePodmanTestCreate()
	})

	AfterEach(func() {
		FakeOutputs = make(map[string][]string)
	})

	It("Test NumberOfContainersRunning", func() {
		FakeOutputs["ps -q"] = []string{"one", "two"}
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("Test NumberOfContainers", func() {
		FakeOutputs["ps -aq"] = []string{"one", "two"}
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))
	})

	It("Test NumberOfPods", func() {
		FakeOutputs["pod ps -q"] = []string{"one", "two"}
		Expect(podmanTest.NumberOfPods()).To(Equal(2))
	})

	It("Test GetContainerStatus", func() {
		FakeOutputs["ps --all --format={{.Status}}"] = []string{"Need func update"}
		Expect(podmanTest.GetContainerStatus()).To(Equal("Need func update"))
	})

	It("Test WaitContainerReady", func() {
		FakeOutputs["logs testimage"] = []string{""}
		Expect(WaitContainerReady(podmanTest, "testimage", "ready", 2, 1)).To(Not(BeTrue()))

		FakeOutputs["logs testimage"] = []string{"I am ready"}
		Expect(WaitContainerReady(podmanTest, "testimage", "am ready", 2, 1)).To(BeTrue())

		FakeOutputs["logs testimage"] = []string{"I am ready"}
		Expect(WaitContainerReady(podmanTest, "testimage", "", 2, 1)).To(BeTrue())
	})
})
