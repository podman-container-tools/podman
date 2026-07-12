//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

func setupContainersConfWithSystemConnections() {
	// make sure connections are not written to real user config on host
	file := filepath.Join(podmanTest.TempDir, "containers.conf")
	f, err := os.Create(file)
	Expect(err).ToNot(HaveOccurred())
	f.Close()
	os.Setenv("CONTAINERS_CONF", file)

	file = filepath.Join(podmanTest.TempDir, "connections.conf")
	f, err = os.Create(file)
	Expect(err).ToNot(HaveOccurred())
	connections := `{"connection":{"default":"QA","connections":{"QA":{"uri":"ssh://root@podman.test:2222/run/podman/podman.sock"},"QB":{"uri":"ssh://root@podman.test:3333/run/podman/podman.sock"}}}}`
	_, err = f.WriteString(connections)
	Expect(err).ToNot(HaveOccurred())
	f.Close()
	os.Setenv("PODMAN_CONNECTIONS_CONF", file)
}

var _ = Describe("podman farm", func() {
	BeforeEach(setupContainersConfWithSystemConnections)

	Context("without running API service", func() {
		It("verify system connections exist", func() {
			session := podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`QA ssh://root@podman.test:2222/run/podman/podman.sock  true true
QB ssh://root@podman.test:3333/run/podman/podman.sock  false true
`))
		})

		It("create farm", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// create farm with only one system connection
			cmd = []string{"farm", "create", "farm2", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" created"))

			// create empty farm
			cmd = []string{"farm", "create", "farm3"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm3\" created"))

			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`farm1 [QA QB] true true
farm2 [QA] false true
farm3 [] false true
`))

			// create existing farm should exit with error
			cmd = []string{"farm", "create", "farm3"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Not(ExitCleanly()))
		})

		It("update existing farms", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// create farm with only one system connection
			cmd = []string{"farm", "create", "farm2", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" created"))

			// create empty farm
			cmd = []string{"farm", "create", "farm3"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm3\" created"))

			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`farm1 [QA QB] true true
farm2 [QA] false true
farm3 [] false true
`))

			// update farm1 to remove the QA connection from it
			cmd = []string{"farm", "update", "--remove", "QA,QB", "farm1"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" updated"))

			// update farm3 to add QA and QB connections to it
			cmd = []string{"farm", "update", "--add", "QB", "farm3"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm3\" updated"))

			// update farm2 to be the default farm
			cmd = []string{"farm", "update", "--default", "farm2"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" updated"))

			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`farm1 [] false true
farm2 [QA] true true
farm3 [QB] false true
`))

			// update farm2 to not be the default, no farms should be the default
			cmd = []string{"farm", "update", "--default=false", "farm2"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" updated"))

			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`farm1 [] false true
farm2 [QA] false true
farm3 [QB] false true
`))
		})

		It("update farm with non-existing connections", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// create farm with only one system connection
			cmd = []string{"farm", "create", "farm2", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" created"))

			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`farm1 [QA QB] true true
farm2 [QA] false true
`))

			// update farm1 to add no-node connection to it
			cmd = []string{"farm", "update", "--add", "no-node", "farm1"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(125, `cannot add to farm, "no-node" is not a system connection`))

			// update farm2 to remove node not in farm connections from it
			cmd = []string{"farm", "update", "--remove", "QB", "farm2"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(125, `cannot remove from farm, "QB" is not a connection in the farm`))

			// check again to ensure that nothing has changed
			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`farm1 [QA QB] true true
farm2 [QA] false true
`))
		})

		It("update non-existent farm", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// update non-existent farm to add QA connection to it
			cmd = []string{"farm", "update", "--add", "no-node", "non-existent"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(125, `cannot update farm, "non-existent" farm doesn't exist`))

			// update non-existent farm to default
			cmd = []string{"farm", "update", "--default", "non-existent"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(125, `cannot update farm, "non-existent" farm doesn't exist`))

			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal(`farm1 [QA QB] true true`))
		})

		It("remove farms", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// create farm with only one system connection
			cmd = []string{"farm", "create", "farm2", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" created"))

			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`farm1 [QA QB] true true
farm2 [QA] false true
`))

			// remove farm1 and a non-existent farm
			// farm 1 should be removed and a warning printed for nonexistent-farm
			cmd = []string{"farm", "rm", "farm1", "nonexistent-farm"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" deleted"))
			Expect(session.ErrorToString()).Should(ContainSubstring("doesn't exist; nothing to remove"))

			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal(`farm2 [QA] true true`))

			// remove all non-existent farms and expect an error
			cmd = []string{"farm", "rm", "foo", "bar"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Not(ExitCleanly()))
		})

		It("remove --all farms", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// create farm with only one system connection
			cmd = []string{"farm", "create", "farm2", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" created"))

			// remove --all
			cmd = []string{"farm", "rm", "--all"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("All farms have been deleted"))

			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal(""))
		})
	})
})
