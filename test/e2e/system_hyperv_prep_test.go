//go:build windows

package integration

import (
	"encoding/json"
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

	It("rejects invalid --format value", func() {
		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format=invalid"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "only supported value for '--format' is 'json'"))
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

	It("outputs JSON status when --format=json is specified", func() {
		skipIfNotAdmin("test requires an elevated (admin) terminal")

		// Preconditions: no existing registry entries
		Expect(vsock.CheckIfHVSockRegistryEntriesExist(1)).To(BeFalse(),
			"vsock registry entries already exist, cannot run test")

		// Ensure cleanup even if test fails
		DeferCleanup(func() {
			session := podmanTest.Podman([]string{"system", "hyperv-prep", "--reset", "--force"})
			session.WaitWithDefaultTimeout()
		})

		// Run --status with --format=json
		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Parse JSON output
		var status struct {
			IsGroupMember      bool `json:"isGroupMember"`
			HasRegistryEntries bool `json:"hasRegistryEntries"`
		}
		err := json.Unmarshal([]byte(session.OutputToString()), &status)
		Expect(err).NotTo(HaveOccurred())

		// Verify expected values when nothing is configured
		Expect(status.HasRegistryEntries).To(BeFalse())

		// Run hyperv-prep to create registry entries
		session = podmanTest.Podman([]string{"system", "hyperv-prep"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Verify hyperv-prep actually succeeded by checking registry entries
		Expect(vsock.CheckIfHVSockRegistryEntriesExist(2)).To(BeTrue(),
			"vsock registry entries don't exist after running hyperv-prep")

		// Run --status with --format=json again
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Parse JSON output
		err = json.Unmarshal([]byte(session.OutputToString()), &status)
		Expect(err).NotTo(HaveOccurred())

		// Verify registry entries exist in JSON output
		Expect(status.HasRegistryEntries).To(BeTrue())

		// Test --format json (with space instead of equals)
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format", "json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Parse JSON output
		err = json.Unmarshal([]byte(session.OutputToString()), &status)
		Expect(err).NotTo(HaveOccurred())
		Expect(status.HasRegistryEntries).To(BeTrue())
	})

	It("preserves human-readable output when --format is not specified", func() {
		skipIfNotAdmin("test requires an elevated (admin) terminal")

		// Preconditions: no existing registry entries and user not in Hyper-V Administrators group
		Expect(vsock.CheckIfHVSockRegistryEntriesExist(1)).To(BeFalse(),
			"vsock registry entries already exist, cannot run test")
		Expect(isCurrentUserHyperVAdmin()).To(BeFalse(),
			"user is already a member of the Hyper-V Administrators group, cannot run test")

		// Ensure cleanup even if test fails
		DeferCleanup(func() {
			session := podmanTest.Podman([]string{"system", "hyperv-prep", "--reset", "--force"})
			session.WaitWithDefaultTimeout()
		})

		// Run --status without --format
		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("No vsock registry entries found"))
		Expect(session.OutputToString()).To(ContainSubstring("Current user is NOT a member"))
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
