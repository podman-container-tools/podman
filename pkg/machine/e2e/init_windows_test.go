package e2e_test

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/Microsoft/go-winio/vhd"
	"github.com/containers/libhvee/pkg/hypervctl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/hyperv/vsock"
	"go.podman.io/podman/v6/pkg/machine/windows"
)

var _ = Describe("podman machine init - windows only", func() {
	It("init with user mode networking", func() {
		if testProvider.VMType() != define.WSLVirt {
			Skip("test is only supported by WSL")
		}
		i := new(initMachine)
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withFakeImage(mb).withUserModeNetworking(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		defer func() {
			runWslCommand([]string{"--terminate", "podman-net-usermode"})
			runWslCommand([]string{"--unregister", "podman-net-usermode"})
		}()

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.UserModeNetworking}}")
		inspectSession, err := mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal("true"))

		// Ensure port 2222 is free
		listener, err := net.Listen("tcp", "0.0.0.0:2222")
		Expect(err).ToNot(HaveOccurred())
		defer listener.Close()
	})
	It("init should not overwrite existing HyperV vms", func() {
		skipIfNotVmtype(define.HyperVVirt, "HyperV test only")
		name := randomString()
		vhdxPath := filepath.Join(testDir, fmt.Sprintf("%s.vhdx", name))
		if err := vhd.CreateVhdx(vhdxPath, 1, 1); err != nil {
			Fail(fmt.Sprintf("failed to create dummy vhdx %q: %q", vhdxPath, err))
		}
		vmm := hypervctl.NewVirtualMachineManager()
		hwConfig := hypervctl.HardwareConfig{
			CPUs:     1,
			DiskPath: vhdxPath,
			DiskSize: 0,
			Memory:   0,
			Network:  false,
		}

		if err := vmm.NewVirtualMachine(name, &hwConfig); err != nil {
			Fail(fmt.Sprintf("failed to create vm %q: %q", name, err))
		}
		vm, err := vmm.GetMachine(name)
		if err != nil {
			Fail(fmt.Sprintf("failed to get vm %q: %q", name, err))
		}
		defer func() {
			if err := vm.Remove(""); err != nil {
				fmt.Printf("failed to clean up vm %q from hypervisor: %q /n", name, err)
			}
		}()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("already exists on hypervisor"))
	})

	It("init should not overwrite existing WSL vms", func() {
		skipIfNotVmtype(define.WSLVirt, "WSL test only")

		name := randomString()
		distName := fmt.Sprintf("podman-%s", name)
		exportedPath := filepath.Join(testDir, "bogus.tar")
		distrDir := filepath.Join(testDir, "testDistro")
		err := os.Mkdir(distrDir, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// create a bogus machine
		i := new(initMachine)
		session, err := mb.setName("foobarexport").setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// export the bogus machine so we have input for making
		// a vm outside the context of podman-machine and also
		// so we dont have to download a distribution from microsoft
		// servers
		exportSession := runWslCommand([]string{"--export", "podman-foobarexport", exportedPath})
		Expect(exportSession).To(Exit(0))

		// importing the machine and creating a vm
		importSession := runWslCommand([]string{"--import", distName, distrDir, exportedPath})
		Expect(importSession).To(Exit(0))

		defer func() {
			runWslCommand([]string{"--unregister", distName})
		}()

		// Trying to make a vm with the same name as an existing name should result in a 125
		checkSession, err := mb.setName(name).setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(checkSession).To(Exit(125))
	})

	It("init should create hvsock entries if they do not exist, otherwise should reuse existing ones", func() {
		skipIfNotVmtype(define.HyperVVirt, "HyperV test only")

		name := randomString()

		// Ensure no HVSock entries exist before we start
		networkHvSocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Network)
		Expect(err).ToNot(HaveOccurred())
		Expect(networkHvSocks).To(BeEmpty())

		readySocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Events)
		Expect(err).ToNot(HaveOccurred())
		Expect(readySocks).To(BeEmpty())

		fileServerVsocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Fileserver)
		Expect(err).ToNot(HaveOccurred())
		Expect(fileServerVsocks).To(BeEmpty())

		// Execute init for the first machine. This should create new HVSock entries
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// Check that the HVSock entries were created
		networkHvSocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Network)
		Expect(err).ToNot(HaveOccurred())
		Expect(networkHvSocks).To(HaveLen(1))

		readySocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Events)
		Expect(err).ToNot(HaveOccurred())
		Expect(readySocks).To(HaveLen(1))

		fileServerVsocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Fileserver)
		countFileServerVsocks := len(fileServerVsocks)
		Expect(err).ToNot(HaveOccurred())
		// check that many file server vsock was created
		Expect(countFileServerVsocks).To(BeNumerically(">", 1))

		// Execute init	for another machine. This should reuse the existing HVSock entries created above
		otherName := randomString()
		i = new(initMachine)
		session, err = mb.setName(otherName).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		newNetworkHvSocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Network)
		Expect(err).ToNot(HaveOccurred())
		Expect(newNetworkHvSocks).To(HaveLen(1))
		Expect(newNetworkHvSocks[0].Port).To(Equal(networkHvSocks[0].Port))

		newReadySocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Events)
		Expect(err).ToNot(HaveOccurred())
		Expect(newReadySocks).To(HaveLen(1))
		Expect(newReadySocks[0].Port).To(Equal(readySocks[0].Port))

		newFileServerVsocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Fileserver)
		Expect(err).ToNot(HaveOccurred())
		Expect(newFileServerVsocks).To(HaveLen(countFileServerVsocks))
		for i := range countFileServerVsocks {
			Expect(newFileServerVsocks[i].Port).To(Equal(fileServerVsocks[i].Port))
		}

		// remove first created machine
		rm := rmMachine{}
		removeSession, err := mb.setName(name).setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeSession).To(Exit(0))

		// Check that HVSock entries still exist after removing one machine
		networkHvSocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Network)
		Expect(err).ToNot(HaveOccurred())
		Expect(networkHvSocks).To(HaveLen(1))

		readySocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Events)
		Expect(err).ToNot(HaveOccurred())
		Expect(readySocks).To(HaveLen(1))

		fileServerVsocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Fileserver)
		Expect(err).ToNot(HaveOccurred())
		Expect(fileServerVsocks).To(HaveLen(countFileServerVsocks))

		// remove second created machine
		rm = rmMachine{}
		removeSession, err = mb.setName(otherName).setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeSession).To(Exit(0))

		// Verify all hvsock entries created during the test were removed
		networkHvSocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Network)
		Expect(err).ToNot(HaveOccurred())
		Expect(networkHvSocks).To(BeEmpty())

		readySocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Events)
		Expect(err).ToNot(HaveOccurred())
		Expect(readySocks).To(BeEmpty())

		fileServerVsocks, err = vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Fileserver)
		Expect(err).ToNot(HaveOccurred())
		Expect(fileServerVsocks).To(BeEmpty())
	})

	// We want to verify that an unprivileged user can initialize a Hyper-V based machine when
	// an administrator has prepared the host with `podman system hyperv-prep`.
	// But this scenario cannot be run in CI (the tests are always executed as an administrator).
	// What we check instead is that `podman machine init` doesn't create additional registry entries
	// when `hyperv-prep` has been used.
	It("when hyperv-prep is used, init should not create new hvsock entries and rm should not remove them", func() {
		skipIfNotVmtype(define.HyperVVirt, "HyperV test only")
		if !windows.HasAdminRights() {
			Skip("test requires an elevated (admin) terminal")
		}

		Expect(loadHVSockEntries()).To(BeEmpty())

		// Registry entries created by hyperv-prep survive machine removal. Remove
		// them or later tests that expect an empty registry would break.
		DeferCleanup(func() {
			bm := basicMachine{}
			session, err := mb.setCmd(bm.withPodmanCommand([]string{
				"system", "hyperv-prep", "--reset", "--registry-only", "--force",
			})).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(0))
			Expect(loadHVSockEntries()).To(BeEmpty())
		})

		// Run `podman system hyperv-prep --registry-only`
		bm := basicMachine{}
		session, err := mb.setCmd(bm.withPodmanCommand([]string{"system", "hyperv-prep", "--registry-only"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// Retrieve the registry entries created by hyperv-prep
		preparedEntries, err := loadHVSockEntries()
		Expect(err).ToNot(HaveOccurred())
		Expect(preparedEntries).ToNot(BeEmpty())
		for key, entry := range preparedEntries {
			Expect(entry.KeepAfterMachineRemove).To(BeTrue(), "entry %s should persist after machine removal", key)
		}

		// Run `podman machine init`
		name := randomString()
		i := new(initMachine)
		session, err = mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// The registry entries before and after machine init should match
		Expect(loadHVSockEntries()).To(Equal(preparedEntries))

		// Run `podman machine rm`
		rm := rmMachine{}
		session, err = mb.setName(name).setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// The registry entries before and after machine rm should match
		Expect(loadHVSockEntries()).To(Equal(preparedEntries))
	})
})

// loadHVSockEntries returns the Podman HVSock registry entries of all purposes
// indexed by registry key name.
func loadHVSockEntries() (map[string]vsock.HVSockRegistryEntry, error) {
	allEntries := make(map[string]vsock.HVSockRegistryEntry)
	for _, purpose := range []vsock.HVSockPurpose{vsock.Network, vsock.Events, vsock.Fileserver} {
		entries, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(purpose)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			allEntries[entry.KeyName] = *entry
		}
	}
	return allEntries, nil
}
