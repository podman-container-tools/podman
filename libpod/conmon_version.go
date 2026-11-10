//go:build !remote && (linux || freebsd)

package libpod

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/utils"
)

// requireConmonV3 fails when the configured conmon is older than version 3.
// feature is the user-facing name of the thing that needs the new conmon,
// for example "the syslog log driver".
func (r *Runtime) requireConmonV3(feature string) error {
	ver, major, err := r.cachedConmonVersion()
	if err != nil {
		return fmt.Errorf("checking conmon version for %s: %w", feature, err)
	}
	if major < 3 {
		return fmt.Errorf("%s requires conmon version 3 or newer, but %s reports %s: %w", feature, r.conmonPath, ver, define.ErrInvalidArg)
	}
	return nil
}

func (r *Runtime) cachedConmonVersion() (string, int, error) {
	r.conmonVersionOnce.Do(func() {
		output, err := utils.ExecCmd(r.conmonPath, "--version")
		if err != nil {
			r.conmonVersionErr = err
			return
		}
		r.conmonVersion = output
	})
	if r.conmonVersionErr != nil {
		return "", 0, r.conmonVersionErr
	}
	return parseConmonVersion(r.conmonVersion)
}

// parseConmonVersion reads the first line of `conmon --version`.
// conmon v2 and v3 both print "conmon version <semver>".
func parseConmonVersion(output string) (string, int, error) {
	line, _, _ := strings.Cut(output, "\n")
	fields := strings.Fields(line)
	if len(fields) < 3 || fields[0] != "conmon" || fields[1] != "version" {
		return "", 0, fmt.Errorf("parsing conmon version %q: %s", strings.TrimSpace(output), define.ErrConmonVersionFormat)
	}
	ver := strings.TrimRight(fields[2], ",")
	parsed, err := semver.Parse(ver)
	if err != nil {
		return "", 0, fmt.Errorf("parsing conmon version %q: %s", ver, define.ErrConmonVersionFormat)
	}
	return ver, int(parsed.Major), nil
}
