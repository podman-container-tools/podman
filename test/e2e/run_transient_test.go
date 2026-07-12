//go:build linux || freebsd

package integration

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman run with volumes", func() {
	var (
		containerStorageDir    string
		dbDir                  string
		runContainerStorageDir string
		runDBDir               string
	)

	BeforeEach(func() {
		containerStorageDir = filepath.Join(podmanTest.Root, podmanTest.ImageCacheFS+"-containers")
		dbDir = filepath.Join(podmanTest.Root, "libpod")
		runContainerStorageDir = filepath.Join(podmanTest.RunRoot, podmanTest.ImageCacheFS+"-containers")
		runDBDir = podmanTest.TempDir
	})

	It("podman run with no transient-store", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_ = SystemExec("ls", []string{"-l", containerStorageDir})

		// All files should be in permanent store, not volatile
		Expect(filepath.Join(containerStorageDir, "containers.json")).Should(BeARegularFile())
		Expect(filepath.Join(containerStorageDir, "volatile-containers.json")).Should(Not(BeAnExistingFile()))
		Expect(filepath.Join(runContainerStorageDir, "containers.json")).Should(Not(BeAnExistingFile()))
		Expect(filepath.Join(runContainerStorageDir, "volatile-containers.json")).Should(Not(BeAnExistingFile()))

		if podmanTest.DatabaseBackend == "sqlite" {
			Expect(filepath.Join(podmanTest.Root, "db.sql")).Should(BeARegularFile())
			Expect(filepath.Join(podmanTest.RunRoot, "db.sql")).Should(Not(BeAnExistingFile()))
		} else {
			Expect(filepath.Join(dbDir, "bolt_state.db")).Should(BeARegularFile())
			Expect(filepath.Join(runDBDir, "bolt_state.db")).Should(Not(BeAnExistingFile()))
		}
	})

	It("podman run --rm with no transient-store", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// All files should not be in permanent store, not volatile
		Expect(filepath.Join(containerStorageDir, "containers.json")).Should(Not(BeAnExistingFile()))
		Expect(filepath.Join(containerStorageDir, "volatile-containers.json")).Should(BeARegularFile())
		Expect(filepath.Join(runContainerStorageDir, "containers.json")).Should(Not(BeAnExistingFile()))
		Expect(filepath.Join(runContainerStorageDir, "volatile-containers.json")).Should(Not(BeAnExistingFile()))

		if podmanTest.DatabaseBackend == "sqlite" {
			Expect(filepath.Join(podmanTest.Root, "db.sql")).Should(BeARegularFile())
			Expect(filepath.Join(podmanTest.RunRoot, "db.sql")).Should(Not(BeAnExistingFile()))
		} else {
			Expect(filepath.Join(dbDir, "bolt_state.db")).Should(BeARegularFile())
			Expect(filepath.Join(runDBDir, "bolt_state.db")).Should(Not(BeAnExistingFile()))
		}
	})

	It("podman run --transient-store", func() {
		SkipIfRemote("Can't change store options remotely")
		session := podmanTest.Podman([]string{"run", "--transient-store", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// All files should be in runroot store, volatile
		Expect(filepath.Join(containerStorageDir, "containers.json")).Should(Not(BeAnExistingFile()))
		Expect(filepath.Join(containerStorageDir, "volatile-containers.json")).Should(Not(BeAnExistingFile()))
		Expect(filepath.Join(runContainerStorageDir, "containers.json")).Should(Not(BeAnExistingFile()))
		Expect(filepath.Join(runContainerStorageDir, "volatile-containers.json")).Should(BeARegularFile())

		if podmanTest.DatabaseBackend == "sqlite" {
			Expect(filepath.Join(podmanTest.Root, "db.sql")).Should(Not(BeAnExistingFile()))
			Expect(filepath.Join(podmanTest.RunRoot, "db.sql")).Should(BeARegularFile())
		} else {
			Expect(filepath.Join(dbDir, "bolt_state.db")).Should(Not(BeAnExistingFile()))
			Expect(filepath.Join(runDBDir, "bolt_state.db")).Should(BeARegularFile())
		}
	})
})
