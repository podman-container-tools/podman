//go:build !remote

package libpod

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/pasta"
	"go.podman.io/common/pkg/apparmor"
	"go.podman.io/common/pkg/cgroups"
	"go.podman.io/common/pkg/seccomp"
	"go.podman.io/common/pkg/version"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage/pkg/unshare"
)

func (r *Runtime) setPlatformHostInfo(info *define.HostInfo) error {
	seccompProfilePath, err := DefaultSeccompPath()
	if err != nil {
		return fmt.Errorf("getting Seccomp profile path: %w", err)
	}

	availableControllers, err := cgroups.AvailableControllers()
	if err != nil {
		return fmt.Errorf("getting available cgroup controllers: %w", err)
	}

	info.CgroupManager = r.config.Engine.CgroupManager
	info.CgroupControllers = availableControllers
	info.IDMappings = define.IDMappings{}
	info.Security = define.SecurityInfo{
		AppArmorEnabled:     apparmor.IsEnabled(),
		DefaultCapabilities: strings.Join(r.config.Containers.DefaultCapabilities.Get(), ","),
		Rootless:            rootless.IsRootless(),
		SECCOMPEnabled:      seccomp.IsEnabled(),
		SECCOMPProfilePath:  seccompProfilePath,
		SELinuxEnabled:      selinux.GetEnabled(),
	}

	info.CgroupsVersion = "v2"

	pastaPath, _ := r.config.FindHelperBinary(pasta.BinaryName, true)
	if pastaPath != "" {
		ver, err := version.Program(pastaPath)
		if err != nil {
			logrus.Warnf("Failed to retrieve program version for %s: %v", pastaPath, err)
		}
		program := define.PastaInfo{
			Executable: pastaPath,
			Package:    version.Package(pastaPath),
			Version:    ver,
		}
		info.Pasta = program
	}

	if rootless.IsRootless() {
		uidmappings, gidmappings, err := unshare.GetHostIDMappings("")
		if err != nil {
			return fmt.Errorf("reading id mappings: %w", err)
		}
		idmappings := define.IDMappings{
			GIDMap: util.RuntimeSpecToIDtools(gidmappings),
			UIDMap: util.RuntimeSpecToIDtools(uidmappings),
		}
		info.IDMappings = idmappings
	}

	return nil
}

func statToPercent(stats []string) (*define.CPUUsage, error) {
	userTotal, err := strconv.ParseFloat(stats[1], 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse user value %q: %w", stats[1], err)
	}
	systemTotal, err := strconv.ParseFloat(stats[3], 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse system value %q: %w", stats[3], err)
	}
	idleTotal, err := strconv.ParseFloat(stats[4], 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse idle value %q: %w", stats[4], err)
	}
	total := userTotal + systemTotal + idleTotal
	s := define.CPUUsage{
		UserPercent:   math.Round((userTotal/total*100)*100) / 100,
		SystemPercent: math.Round((systemTotal/total*100)*100) / 100,
		IdlePercent:   math.Round((idleTotal/total*100)*100) / 100,
	}
	return &s, nil
}

// getCPUUtilization Returns a CPUUsage object that summarizes CPU
// usage for userspace, system, and idle time.
func getCPUUtilization() (*define.CPUUsage, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	// Read first line of /proc/stat that has entries for system ("cpu" line)
	for scanner.Scan() {
		break
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// column 1 is user, column 3 is system, column 4 is idle
	stats := strings.Fields(scanner.Text())
	return statToPercent(stats)
}
