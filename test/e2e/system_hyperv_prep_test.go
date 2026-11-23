//go:build windows

package integration

import (
	"os/exec"
	"os/user"
	"strings"

	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/machine/hyperv/vsock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("podman system hyperv-prep", func() {
	It("rejects incompatible flags together", func() {
		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--reset"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "none of the others can be"))
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--mounts", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "none of the others can be"))
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--reset", "--mounts", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "none of the others can be"))
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--registry-only"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "none of the others can be"))
	})

	It("--registry-only leaves the Hyper-V Administrators group unchanged", func() {
		skipIfNotAdmin("test requires an elevated (admin) terminal")

		for _, purpose := range []vsock.HVSockPurpose{vsock.Network, vsock.Events, vsock.Fileserver} {
			entries, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(purpose)
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(BeEmpty(), "%s vsock registry entries already exist, cannot run test", purpose)
		}
		wasHyperVAdmin := isCurrentUserHyperVAdmin()

		// Entries persist, so remove them even if an assertion fails
		defer func() {
			podmanTest.Podman([]string{"system", "hyperv-prep", "--reset", "--registry-only", "--force"}).WaitWithDefaultTimeout()
		}()

		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--registry-only"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(vsock.CheckIfHVSockRegistryEntriesExist(1)).To(BeTrue(),
			"vsock registry entries don't exist after running hyperv-prep --registry-only")
		Expect(isCurrentUserHyperVAdmin()).To(Equal(wasHyperVAdmin),
			"hyperv-prep --registry-only changed the group membership")

		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--reset", "--registry-only", "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		for _, purpose := range []vsock.HVSockPurpose{vsock.Network, vsock.Events, vsock.Fileserver} {
			entries, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(purpose)
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(BeEmpty(), "%s vsock registry entries still exist after reset", purpose)
		}
		Expect(isCurrentUserHyperVAdmin()).To(Equal(wasHyperVAdmin),
			"hyperv-prep --reset --registry-only changed the group membership")
	})

	It("creates registry entries and resets them", func() {
		skipIfNotAdmin("test requires an elevated (admin) terminal")

		// Preconditions: no existing registry entries and user not in Hyper-V Administrators group
		Expect(vsock.CheckIfHVSockRegistryEntriesExist(1)).To(BeFalse(),
			"vsock registry entries already exist, cannot run test")
		Expect(isCurrentUserHyperVAdmin()).To(BeFalse(),
			"user is already a member of the Hyper-V Administrators group, cannot run test")

		// Run `--status` a first time to check that it reports correctly that:
		//   - No vsock registry keys exist
		//   - User isn't a member of the Hyper-V Administrators group
		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("No vsock registry entries found"))
		Expect(session.OutputToString()).To(ContainSubstring("Current user is NOT a member"))

		// Run hyperv-prep to create registry entries
		session = podmanTest.Podman([]string{"system", "hyperv-prep"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Network"))
		Expect(session.OutputToString()).To(ContainSubstring("Events"))
		Expect(session.OutputToString()).To(ContainSubstring("Fileserver"))

		// Check that running hyperv-prep worked:
		//  - The registry entries for vsock have been created (for the default
		//    machine mounts: home dir, drive root and containers config dir)
		//  - The user is a member of the Hyper-V Administrators group
		cfg, err := config.Default()
		Expect(err).ToNot(HaveOccurred())
		expectedVSockNum := len(cfg.Machine.Volumes.Get())
		Expect(vsock.CheckIfHVSockRegistryEntriesExist(expectedVSockNum)).To(BeTrue(),
			"vsock registry entries don't exist after running hyperv-prep")
		Expect(isCurrentUserHyperVAdmin()).To(BeTrue(),
			"user isn't a member of the Hyper-V Administrators group after running hyperv-prep")

		// Run --status a second time to shows the created entries
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Network"))
		Expect(session.OutputToString()).To(ContainSubstring("Events"))
		Expect(session.OutputToString()).To(ContainSubstring("Fileserver"))

		// Reset the entries to clean up
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--reset", "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Successfully removed"))
	})
})

func isCurrentUserHyperVAdmin() bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`Get-LocalGroupMember -Name "Hyper-V Administrators"`).Output()
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, u.Username) {
			return true
		}
	}
	return false
}
