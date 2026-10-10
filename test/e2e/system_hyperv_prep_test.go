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

	It("rejects --format without --status", func() {
		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "'--format' can only be used with '--status'"))

		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--format", "json"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "'--format' can only be used with '--status'"))

		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--reset", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "'--format' can only be used with '--status'"))
	})

	It("rejects --format with invalid Go template syntax", func() {
		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format", "{{.Invalid"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "template:"))
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

	It("outputs status with --format (JSON and Go template)", func() {
		type statusReport struct {
			CurrentUserIsHyperVAdmin   bool   `json:"currentUserIsHyperVAdmin"`
			HasRequiredRegistryEntries bool   `json:"hasRequiredRegistryEntries"`
			Status                     string `json:"status"`
		}

		// 1. Test --format=json (equals syntax)
		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		var status statusReport
		Expect(json.Unmarshal([]byte(session.OutputToString()), &status)).To(Succeed())
		Expect(status.Status).To(BeElementOf("applied", "notApplied", "partiallyApplied"))

		// 2. Test --format json (space syntax)
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format", "json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(json.Unmarshal([]byte(session.OutputToString()), &status)).To(Succeed())

		// 3. Test Go template field formatting
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format", "{{.Status}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(strings.TrimSpace(session.OutputToString())).To(BeElementOf("applied", "notApplied", "partiallyApplied"))

		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format", "{{.CurrentUserIsHyperVAdmin}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(strings.TrimSpace(session.OutputToString())).To(BeElementOf("true", "false"))

		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format", "{{.HasRequiredRegistryEntries}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(strings.TrimSpace(session.OutputToString())).To(BeElementOf("true", "false"))

		// 4. Test Go template built-in json function
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format", "{{json .}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(json.Unmarshal([]byte(session.OutputToString()), &status)).To(Succeed())
	})

	It("updates formatted status through prep and reset lifecycle", func() {
		skipIfNotAdmin("test requires an elevated (admin) terminal")

		// Preconditions: no existing registry entries
		Expect(vsock.CheckIfHVSockRegistryEntriesExist(1)).To(BeFalse(),
			"vsock registry entries already exist, cannot run test")

		DeferCleanup(func() {
			session := podmanTest.Podman([]string{"system", "hyperv-prep", "--reset", "--force"})
			session.WaitWithDefaultTimeout()
		})

		type statusReport struct {
			CurrentUserIsHyperVAdmin   bool   `json:"currentUserIsHyperVAdmin"`
			HasRequiredRegistryEntries bool   `json:"hasRequiredRegistryEntries"`
			Status                     string `json:"status"`
		}

		// 1. Before prep: registry entries must not exist
		session := podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		var status statusReport
		Expect(json.Unmarshal([]byte(session.OutputToString()), &status)).To(Succeed())
		Expect(status.HasRequiredRegistryEntries).To(BeFalse())
		Expect(status.Status).To(BeElementOf("notApplied", "partiallyApplied"))

		// 2. Run hyperv-prep to configure host
		session = podmanTest.Podman([]string{"system", "hyperv-prep"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// 3. After prep: registry entries and group membership must exist
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(json.Unmarshal([]byte(session.OutputToString()), &status)).To(Succeed())
		Expect(status.HasRequiredRegistryEntries).To(BeTrue())
		Expect(status.CurrentUserIsHyperVAdmin).To(BeTrue())
		Expect(status.Status).To(Equal("applied"))

		// Also verify via Go template
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format", "{{.Status}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(strings.TrimSpace(session.OutputToString())).To(Equal("applied"))

		// 4. Run reset
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--reset", "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// 5. After reset: registry entries must be gone
		session = podmanTest.Podman([]string{"system", "hyperv-prep", "--status", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(json.Unmarshal([]byte(session.OutputToString()), &status)).To(Succeed())
		Expect(status.HasRequiredRegistryEntries).To(BeFalse())
		Expect(status.Status).To(BeElementOf("notApplied", "partiallyApplied"))
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
