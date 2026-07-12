package bindings_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/network"
)

var _ = Describe("Podman networks", func() {
	var (
		bt       *bindingTest
		s        *gexec.Session
		connText context.Context
		err      error
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		connText, err = bindings.NewConnection(context.Background(), bt.sock) //nolint:fatcontext
		Expect(err).ToNot(HaveOccurred())
		_, err = network.Prune(connText, &network.PruneOptions{})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("podman prune unused networks with filters", func() {
		name := "foobar"
		net := types.Network{
			Name: name,
		}
		_, err = network.Create(connText, &net)
		Expect(err).ToNot(HaveOccurred())

		// Invalid filters should return error
		filtersIncorrect := map[string][]string{
			"status": {"dummy"},
		}
		_, err = network.Prune(connText, new(network.PruneOptions).WithFilters(filtersIncorrect))
		Expect(err).To(HaveOccurred())

		// List filter params should not work with prune.
		filtersIncorrect = map[string][]string{
			"name": {name},
		}
		_, err = network.Prune(connText, new(network.PruneOptions).WithFilters(filtersIncorrect))
		Expect(err).To(HaveOccurred())

		// Mismatched label, correct filter params => no network should be pruned.
		filtersIncorrect = map[string][]string{
			"label": {"xyz"},
		}
		pruneResponse, err := network.Prune(connText, new(network.PruneOptions).WithFilters(filtersIncorrect))
		Expect(err).ToNot(HaveOccurred())
		Expect(pruneResponse).To(BeEmpty())

		// Mismatched until, correct filter params => no network should be pruned.
		filters := map[string][]string{
			"until": {"50"}, // January 1, 1970
		}
		pruneResponse, err = network.Prune(connText, new(network.PruneOptions).WithFilters(filters))
		Expect(err).ToNot(HaveOccurred())
		Expect(pruneResponse).To(BeEmpty())

		// Valid filter params => network should be pruned now.
		filters = map[string][]string{
			"until": {"5000000000"}, // June 11, 2128
		}
		pruneResponse, err = network.Prune(connText, new(network.PruneOptions).WithFilters(filters))
		Expect(err).ToNot(HaveOccurred())
		Expect(pruneResponse).To(HaveLen(1))
	})

	It("create network", func() {
		// create a network with blank config should work
		_, err = network.Create(connText, nil)
		Expect(err).ToNot(HaveOccurred())

		name := "foobar"
		net := types.Network{
			Name: name,
		}

		report, err := network.Create(connText, &net)
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Name).To(Equal(name))

		// create network with same name should 409
		_, err = network.Create(connText, &net)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusConflict))

		// create network with same name and ignore false should 409
		options := new(network.ExtraCreateOptions).WithIgnoreIfExists(false)
		_, err = network.CreateWithOptions(connText, &net, options)
		Expect(err).To(HaveOccurred())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusConflict))

		// create network with same name and ignore true succeed
		options = new(network.ExtraCreateOptions).WithIgnoreIfExists(true)
		report, err = network.CreateWithOptions(connText, &net, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Name).To(Equal(name))
	})

	It("inspect network", func() {
		name := "foobar"
		net := types.Network{
			Name: name,
		}
		_, err = network.Create(connText, &net)
		Expect(err).ToNot(HaveOccurred())
		data, err := network.Inspect(connText, name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(data.Name).To(Equal(name))
	})

	It("list networks", func() {
		// create a bunch of named networks and make verify with list
		netNames := []string{"homer", "bart", "lisa", "maggie", "marge"}
		for i := range 5 {
			net := types.Network{
				Name: netNames[i],
			}
			_, err = network.Create(connText, &net)
			Expect(err).ToNot(HaveOccurred())
		}
		list, err := network.List(connText, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(list)).To(BeNumerically(">=", 5))
		for _, n := range list {
			if n.Name != "podman" {
				Expect(slices.Contains(netNames, n.Name)).To(BeTrue())
			}
		}

		// list with bad filter should be 500
		filters := make(map[string][]string)
		filters["foobar"] = []string{"1234"}
		options := new(network.ListOptions).WithFilters(filters)
		_, err = network.List(connText, options)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		// filter list with success
		filters = make(map[string][]string)
		filters["name"] = []string{"homer"}
		options = new(network.ListOptions).WithFilters(filters)
		list, err = network.List(connText, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(list).To(HaveLen(1))
		Expect(list[0].Name).To(Equal("homer"))
	})

	It("remove network", func() {
		// removing a noName network should result in 404
		_, err := network.Remove(connText, "noName", nil)
		code, err := bindings.CheckResponseCode(err)
		Expect(err).ToNot(HaveOccurred())
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// removing a noName network with ignore should succeed
		options := new(network.RemoveOptions).WithIgnore(true)
		report, err := network.Remove(connText, "noName", options)
		Expect(err).ToNot(HaveOccurred())
		Expect(report).To(BeEmpty())

		// Removing an unused network should work
		name := "unused"
		net := types.Network{
			Name: name,
		}
		_, err = network.Create(connText, &net)
		Expect(err).ToNot(HaveOccurred())
		report, err = network.Remove(connText, name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(report[0].Name).To(Equal(name))

		// Removing a network that is being used without force should be 500
		name = "used"
		net = types.Network{
			Name: name,
		}
		_, err = network.Create(connText, &net)
		Expect(err).ToNot(HaveOccurred())

		// Start container and wait
		container := "ntest"
		session := bt.runPodman([]string{"run", "-dt", fmt.Sprintf("--network=%s", name), "--name", container, alpine.name, "top"})
		session.Wait(45)
		Expect(session.ExitCode()).To(BeZero())

		_, err = network.Remove(connText, name, nil)
		code, err = bindings.CheckResponseCode(err)
		Expect(err).ToNot(HaveOccurred())
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		// Removing with a network in use with force should work with a stopped container
		err = containers.Stop(connText, container, new(containers.StopOptions).WithTimeout(0))
		Expect(err).ToNot(HaveOccurred())
		options = new(network.RemoveOptions).WithForce(true)
		report, err = network.Remove(connText, name, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(report[0].Name).To(Equal(name))
	})
})
