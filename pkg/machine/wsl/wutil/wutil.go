//go:build windows

package wutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
)

var (
	onceStatus              sync.Once
	status                  wslStatus
	onceVersion             sync.Once
	version                 wslVersion
	wslNotInstalledMessages = []string{"kernel file is not found", "The Windows Subsystem for Linux is not installed"}
	vmpDisabledMessages     = []string{"enable the Virtual Machine Platform Windows feature", "Enable \"Virtual Machine Platform\""}
	wslDisabledMessages     = []string{"enable the \"Windows Subsystem for Linux\" optional component"}
)

type wslStatus struct {
	installed         bool
	vmpFeatureEnabled bool
	wslFeatureEnabled bool
}

type wslVersion struct {
	val      [3]int
	cmdErr   error
	parseErr error
}

func NewWSLCommand(arg ...string) *exec.Cmd {
	cmd := exec.Command("wsl", arg...)
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	return cmd
}

func SilentExec(command string, args ...string) error {
	cmd := NewWSLCommand(args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w", command, args, err)
	}
	return nil
}

func SilentExecCmd(args ...string) *exec.Cmd {
	cmd := NewWSLCommand(args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	return cmd
}

func parseWSLStatus() wslStatus {
	onceStatus.Do(func() {
		cmd := SilentExecCmd("--status")
		out, err := cmd.StdoutPipe()
		cmd.Stderr = nil
		if err != nil {
			return
		}
		if err = cmd.Start(); err != nil {
			return
		}
		var outputErr error
		status, outputErr = matchOutputLine(out)
		err = cmd.Wait()
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			// If `wsl --status` returns an exit error
			// we assume that WSL isn't installed and
			// override whatever was returned by
			// matchOutputLine()
			status = wslStatus{}
		}
		if outputErr != nil {
			logrus.Errorf("Failed to read wsl --status output: %v", outputErr)
			status = wslStatus{}
		}
	})
	return status
}

func IsWSLInstalled() bool {
	status := parseWSLStatus()
	return status.installed && status.vmpFeatureEnabled
}

func IsWSLStoreVersionInstalled() bool {
	// The Windows built-in version of WSL returns an error when running
	// `wsl --version`. Recent versions of WSL instead, are distributed
	// via the Microsoft store and supports `wsl --version`.
	return parseWSLVersion().cmdErr == nil
}

// WSLVersionAtLeast returns true if the installed WSL version is >= major.minor.patch
// or if it fails to parse the wsl --version output. It returns false otherwise and if
// the `wsl --version` command has failed.
func WSLVersionAtLeast(major, minor, patch int) bool {
	v := parseWSLVersion()
	// When the `wsl --version` command fails we return false
	// because it's older WSL versions behavior.
	if v.cmdErr != nil {
		logrus.Debugf("command error while checking the WSL version: %v", v.cmdErr)
		return false
	}
	// When there is a parsing error we assume that's a WSL future version
	// where the output format has been changed.
	if v.parseErr != nil {
		logrus.Debugf("parsing error while checking the WSL version: %v", v.parseErr)
		return true
	}
	return v.val[0] > major ||
		(v.val[0] == major && v.val[1] > minor) ||
		(v.val[0] == major && v.val[1] == minor && v.val[2] >= patch)
}

// parseWSLVersion returns the WSL version as [major, minor, patch] in the
// returned wslVersion struct.
// The returned `wslVersion.cmdErr` is non nil if the `wsl --version` command fails.
// The returned `wslVersion.parseErr` is non nil if the major.minor.patch string
// cannot found in the returned output.
func parseWSLVersion() wslVersion {
	onceVersion.Do(func() {
		cmd := SilentExecCmd("--version")
		out, err := cmd.Output()
		if err != nil {
			version = wslVersion{cmdErr: fmt.Errorf("running wsl --version failed: %w", err)}
			return
		}
		version = versionFromLocalizedOutput(string(out))
	})
	return version
}

// versionFromLocalizedOutput returns the WSL version from the output of "wsl --version".
// The output labels are localized, so the version is the first version number
// in the output, that is the WSL package version in all languages.
func versionFromLocalizedOutput(out string) wslVersion {
	v := wslVersion{}
	versionRegexp := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	m := versionRegexp.FindStringSubmatch(out)
	if m == nil {
		return wslVersion{parseErr: fmt.Errorf("could not parse WSL version from %q", out)}
	}
	for i := range v.val {
		var err error
		if v.val[i], err = strconv.Atoi(m[i+1]); err != nil {
			return wslVersion{parseErr: fmt.Errorf("could not parse the WSL version digit %q", m[i+1])}
		}
	}
	return v
}

func matchOutputLine(output io.ReadCloser) (wslStatus, error) {
	status := wslStatus{
		installed:         true,
		vmpFeatureEnabled: true,
		wslFeatureEnabled: true,
	}
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		line := scanner.Text()
		for _, match := range wslNotInstalledMessages {
			if strings.Contains(line, match) {
				status.installed = false
			}
		}
		for _, match := range vmpDisabledMessages {
			if strings.Contains(line, match) {
				status.vmpFeatureEnabled = false
			}
		}
		for _, match := range wslDisabledMessages {
			if strings.Contains(line, match) {
				status.wslFeatureEnabled = false
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return status, err
	}
	return status, nil
}
