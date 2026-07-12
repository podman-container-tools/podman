//go:build windows

package integration

import (
	"os/exec"
	"os/user"
	"strings"

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
		//  - The registry entries for vsock have been created (for 2 mounts)
		//  - The user is a member of the Hyper-V Administrators group
		Expect(vsock.CheckIfHVSockRegistryEntriesExist(2)).To(BeTrue(),
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
