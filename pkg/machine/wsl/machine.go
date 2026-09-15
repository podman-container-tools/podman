//go:build windows

package wsl

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/ignition"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/pkg/machine/wsl/wutil"
	"github.com/containers/podman/v5/utils"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/strongunits"
	"go.podman.io/storage/pkg/homedir"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc)
	vmtype             = define.WSLVirt
	ErrWslNotSupported = errors.New("wsl features not supported or configured correctly")
)

type ExitCodeError struct {
	code uint
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("Process failed with exit code: %d", e.code)
}

//nolint:unused
func getConfigPath(name string) (string, error) {
	return getConfigPathExt(name, "json")
}

//nolint:unused
func getConfigPathExt(name string, extension string) (string, error) {
	vmConfigDir, err := env.GetConfDir(vmtype)
	if err != nil {
		return "", err
	}

	return filepath.Join(vmConfigDir, fmt.Sprintf("%s.%s", name, extension)), nil
}

// TODO like provisionWSL, i think this needs to be pushed to use common
// paths and types where possible
func unprovisionWSL(mc *vmconfigs.MachineConfig) error {
	dist := env.WithPodmanPrefix(mc.Name)
	if err := terminateDist(dist); err != nil {
		logrus.Error(err)
	}
	if err := unregisterDist(dist); err != nil {
		logrus.Error(err)
	}

	vmDataDir, err := env.GetDataDir(vmtype)
	if err != nil {
		return err
	}
	distDir := filepath.Join(vmDataDir, "wsldist")
	distTarget := filepath.Join(distDir, mc.Name)
	return utils.GuardedRemoveAll(distTarget)
}

// TODO there are some differences here that I dont fully groak but I think
// we should push this stuff be more common (dir names, etc) and also use
// typed things where possible like vmfiles
func provisionWSLDist(name string, imagePath string, prompt string) (string, error) {
	vmDataDir, err := env.GetDataDir(vmtype)
	if err != nil {
		return "", err
	}

	distDir := filepath.Join(vmDataDir, "wsldist")
	distTarget := filepath.Join(distDir, name)
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create wsldist directory: %w", err)
	}

	dist := env.WithPodmanPrefix(name)
	fmt.Println(prompt)

	// Run WSL import and analyze output for specific errors.
	// If the 'Virtual Machine Platform' feature is disabled, we expect a failure
	// with HCS service-related errors such as:
	// 1. Wsl/Service/RegisterDistro/CreateVm/HCS/ERROR_NOT_SUPPORTED
	// 2. Wsl/Service/RegisterDistro/CreateVm/HCS/HCS_E_SERVICE_NOT_AVAILABLE
	cmdOutput := &bytes.Buffer{}
	cmd := wutil.NewWSLCommand("--import", dist, distTarget, imagePath, "--version", "2")
	err = runCmdPassThroughTee(cmdOutput, cmd)
	decodedStr := strings.ToLower(cmdOutput.String())
	for _, substr := range []string{"hcs/error_not_supported", "hcs/hcs_e_service_not_available"} {
		if strings.Contains(decodedStr, substr) {
			return "", ErrWslNotSupported
		}
	}
	if err != nil {
		return "", fmt.Errorf("the WSL import of guest OS failed: %w", err)
	}

	// Fixes newuidmap
	if err = wslInvoke(dist, "rpm", "--restore", "shadow-utils"); err != nil {
		return "", fmt.Errorf("package permissions restore of shadow-utils on guest OS failed: %w", err)
	}

	if err = wslInvoke(dist, "mkdir", "-p", "/usr/local/bin"); err != nil {
		return "", fmt.Errorf("could not create /usr/local/bin: %w", err)
	}

	if err = wslInvoke(dist, "ln", "-f", "-s", gvForwarderPath, "/usr/local/bin/vm"); err != nil {
		return "", fmt.Errorf("could not setup compatibility link: %w", err)
	}

	return dist, nil
}

func createKeys(mc *vmconfigs.MachineConfig, dist string) error {
	user := mc.SSH.RemoteUsername

	if err := terminateDist(dist); err != nil {
		return fmt.Errorf("could not cycle WSL dist: %w", err)
	}

	identityPath := mc.SSH.IdentityPath + ".pub"

	// TODO We could audit vmfile reads and see if a 'ReadToString'
	// method makes sense.
	pubKey, err := os.ReadFile(identityPath)
	if err != nil {
		return fmt.Errorf("could not create ssh keys: %w", err)
	}

	key := string(pubKey)

	if err := wslPipe(key+"\n", dist, "sh", "-c", "mkdir -p /root/.ssh;"+
		"cat >> /root/.ssh/authorized_keys; chmod 600 /root/.ssh/authorized_keys"); err != nil {
		return fmt.Errorf("could not create root authorized keys on guest OS: %w", err)
	}

	userAuthCmd := withUser("mkdir -p /home/[USER]/.ssh;"+
		"cat >> /home/[USER]/.ssh/authorized_keys; chown -R [USER]:[USER] /home/[USER]/.ssh;"+
		"chmod 600 /home/[USER]/.ssh/authorized_keys", user)
	if err := wslPipe(key+"\n", dist, "sh", "-c", userAuthCmd); err != nil {
		return fmt.Errorf("could not create '%s' authorized keys on guest OS: %w", user, err)
	}

	return nil
}

func configureSystem(mc *vmconfigs.MachineConfig, dist string, ansibleConfig *vmconfigs.AnsibleConfig) error {
	user := mc.SSH.RemoteUsername
	if err := wslInvoke(dist, "sh", "-c", fmt.Sprintf(appendPort, mc.SSH.Port, mc.SSH.Port)); err != nil {
		return fmt.Errorf("could not configure SSH port for guest OS: %w", err)
	}

	if err := wslPipe(withUser(configServices, user), dist, "sh"); err != nil {
		return fmt.Errorf("could not configure systemd settings for guest OS: %w", err)
	}

	if err := wslPipe(sudoers, dist, "sh", "-c", "cat >> /etc/sudoers"); err != nil {
		return fmt.Errorf("could not add wheel to sudoers: %w", err)
	}

	if err := wslPipe(overrideSysusers, dist, "sh", "-c",
		"cat > /etc/systemd/system/systemd-sysusers.service.d/override.conf"); err != nil {
		return fmt.Errorf("could not generate systemd-sysusers override for guest OS: %w", err)
	}

	if ansibleConfig != nil {
		if err := wslPipe(ansibleConfig.Contents, dist, "sh", "-c", fmt.Sprintf("cat > %s", ansibleConfig.PlaybookPath)); err != nil {
			return fmt.Errorf("could not generate playbook file for guest os: %w", err)
		}
	}

	if err := enableUserLinger(mc, dist); err != nil {
		return err
	}

	if err := wslPipe(containersConf, dist, "sh", "-c", "cat > /etc/containers/containers.conf"); err != nil {
		return fmt.Errorf("could not create containers.conf for guest OS: %w", err)
	}

	if err := configureRegistries(dist); err != nil {
		return err
	}

	if err := setupPodmanDockerSock(dist, mc.HostUser.Rootful); err != nil {
		return err
	}

	if err := wslInvoke(dist, "sh", "-c", "echo wsl > /etc/containers/podman-machine"); err != nil {
		return fmt.Errorf("could not create podman-machine file for guest OS: %w", err)
	}

	if err := configureBindMounts(dist, user); err != nil {
		return err
	}

	return changeDistUserModeNetworking(dist, user, mc.ImagePath.GetPath(), mc.WSLHypervisor.UserModeNetworking)
}

func configureBindMounts(dist string, user string) error {
	if err := wslPipe(fmt.Sprintf(bindMountSystemService, dist), dist, "sh", "-c", "cat > /etc/systemd/system/podman-mnt-bindings.service"); err != nil {
		return fmt.Errorf("could not create podman binding service file for guest OS: %w", err)
	}

	if err := wslPipe(getConfigBindServicesScript(user), dist, "sh"); err != nil {
		return fmt.Errorf("could not configure podman binding services for guest OS: %w", err)
	}

	catUserService := "cat > " + getUserUnitPath(user)
	if err := wslPipe(getBindMountUserService(dist), dist, "sh", "-c", catUserService); err != nil {
		return fmt.Errorf("could not create podman binding user service file for guest OS: %w", err)
	}

	if err := wslPipe(getBindMountFsTab(dist), dist, "sh", "-c", "cat >> /etc/fstab"); err != nil {
		return fmt.Errorf("could not create podman binding fstab entry for guest OS: %w", err)
	}

	catGroupDropin := fmt.Sprintf("cat > %s/%s", podmanSocketDropinPath, "10-group.conf")
	if err := wslPipe(overrideSocketGroup, dist, "sh", "-c", catGroupDropin); err != nil {
		return fmt.Errorf("could not configure podman socket group override: %w", err)
	}

	return nil
}

func getConfigBindServicesScript(user string) string {
	return fmt.Sprintf(configBindServices, user)
}

func getBindMountUserService(dist string) string {
	return fmt.Sprintf(bindMountUserService, dist)
}

func getUserUnitPath(user string) string {
	return fmt.Sprintf(bindUserUnitPath, user)
}

func getBindMountFsTab(dist string) string {
	return fmt.Sprintf(bindMountFsTab, dist)
}

func setupPodmanDockerSock(dist string, rootful bool) error {
	content := ignition.GetPodmanDockerTmpConfig(1000, rootful, true)

	if err := wslPipe(content, dist, "sh", "-c", "cat > "+ignition.PodmanDockerTmpConfPath); err != nil {
		return fmt.Errorf("could not create internal docker sock conf: %w", err)
	}

	return nil
}

func enableUserLinger(mc *vmconfigs.MachineConfig, dist string) error {
	lingerCmd := "mkdir -p /var/lib/systemd/linger; touch /var/lib/systemd/linger/" + mc.SSH.RemoteUsername
	if err := wslInvoke(dist, "sh", "-c", lingerCmd); err != nil {
		return fmt.Errorf("could not enable linger for remote user on guest OS: %w", err)
	}

	return nil
}

func configureRegistries(dist string) error {
	cmd := "cat > /etc/containers/registries.conf.d/999-podman-machine.conf"
	if err := wslPipe(registriesConf, dist, "sh", "-c", cmd); err != nil {
		return fmt.Errorf("could not configure registries on guest OS: %w", err)
	}

	return nil
}

func installScripts(dist string) error {
	if err := wslPipe(enterns, dist, "sh", "-c",
		"cat > /usr/local/bin/enterns; chmod 755 /usr/local/bin/enterns"); err != nil {
		return fmt.Errorf("could not create enterns script for guest OS: %w", err)
	}

	if err := wslPipe(profile, dist, "sh", "-c",
		"cat > /etc/profile.d/enterns.sh"); err != nil {
		return fmt.Errorf("could not create motd profile script for guest OS: %w", err)
	}

	if err := wslPipe(wslmotd, dist, "sh", "-c", "cat > /etc/wslmotd"); err != nil {
		return fmt.Errorf("could not create a WSL MOTD for guest OS: %w", err)
	}

	if err := wslPipe(bootstrap, dist, "sh", "-c",
		"cat > /root/bootstrap; chmod 755 /root/bootstrap"); err != nil {
		return fmt.Errorf("could not create bootstrap script for guest OS: %w", err)
	}

	return nil
}

func writeWslConf(dist string, user string) error {
	if err := wslPipe(withUser(wslConf, user), dist, "sh", "-c", "cat > /etc/wsl.conf"); err != nil {
		return fmt.Errorf("could not configure wsl config for guest OS: %w", err)
	}

	return nil
}

func attemptFeatureInstall(reExec, admin bool) error {
	if !winVersionAtLeast(10, 0, 18362) {
		return errors.New("your version of Windows does not support WSL. Update to Windows 10 Build 19041 or later")
	} else if !winVersionAtLeast(10, 0, 19041) {
		fmt.Fprint(os.Stderr, wslOldVersion)
		return errors.New("the WSL can not be automatically installed")
	}

	message := "WSL is not installed on this system, installing it.\n\n"

	if !admin {
		message += "Since you are not running as admin, a new window will open and " +
			"require you to approve administrator privileges.\n\n"
	}

	message += "NOTE: A system reboot will be required as part of this process. " +
		"If you prefer, you may abort now, and perform a manual installation using the \"wsl --install\" command."

	if !reExec && MessageBox(message, "Podman Machine", false) != 1 {
		return fmt.Errorf("the WSL installation aborted: %w", define.ErrInitRelaunchAttempt)
	}

	if !reExec && !admin {
		return launchElevate("install the Windows WSL Features")
	}
	return installWsl()
}

func launchElevate(operation string) error {
	if err := createOrTruncateElevatedOutputFile(); err != nil {
		return err
	}
	err := relaunchElevatedWait()
	if err != nil {
		if eerr, ok := err.(*ExitCodeError); ok {
			if eerr.code == ErrorSuccessRebootRequired {
				fmt.Println("Reboot is required to continue installation, please reboot at your convenience")
				return define.ErrInitRelaunchAttempt
			}
		}

		fmt.Fprintf(os.Stderr, "Elevated process failed with error: %v\n\n", err)
		dumpOutputFile()
		fmt.Fprintf(os.Stderr, wslInstallError, operation)
		return fmt.Errorf("%w: %w", err, define.ErrInitRelaunchAttempt)
	}
	return define.ErrInitRelaunchAttempt
}

func installWsl() error {
	log, err := getElevatedOutputFileWrite()
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command("dism", "/online", "/enable-feature",
		"/featurename:Microsoft-Windows-Subsystem-Linux", "/all", "/norestart")
	if err := runCmdPassThroughTee(log, cmd); isMsiError(err) {
		return fmt.Errorf("could not enable WSL Feature: %w", err)
	}

	cmd = exec.Command("dism", "/online", "/enable-feature",
		"/featurename:VirtualMachinePlatform", "/all", "/norestart")
	if err = runCmdPassThroughTee(log, cmd); isMsiError(err) {
		return fmt.Errorf("could not enable Virtual Machine Feature: %w", err)
	}

	return reboot()
}

func getElevatedOutputFileName() (string, error) {
	dir, err := homedir.GetDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "podman-elevated-output.log"), nil
}

func dumpOutputFile() {
	file, err := getElevatedOutputFileRead()
	if err != nil {
		logrus.Debug("could not find elevated child output file")
		return
	}
	defer file.Close()
	_, _ = io.Copy(os.Stdout, file)
}

func getElevatedOutputFileRead() (*os.File, error) {
	return getElevatedOutputFile(os.O_RDONLY)
}

func getElevatedOutputFileWrite() (*os.File, error) {
	return getElevatedOutputFile(os.O_WRONLY | os.O_CREATE | os.O_APPEND)
}

func createOrTruncateElevatedOutputFile() error {
	name, err := getElevatedOutputFileName()
	if err != nil {
		return err
	}

	_, err = os.Create(name)
	return err
}

func getElevatedOutputFile(mode int) (*os.File, error) {
	name, err := getElevatedOutputFileName()
	if err != nil {
		return nil, err
	}

	dir, err := homedir.GetDataHome()
	if err != nil {
		return nil, err
	}

	if err = os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	return os.OpenFile(name, mode, 0o644)
}

func isMsiError(err error) bool {
	if err == nil {
		return false
	}

	if eerr, ok := err.(*exec.ExitError); ok {
		switch eerr.ExitCode() {
		case 0:
			fallthrough
		case ErrorSuccessRebootInitiated:
			fallthrough
		case ErrorSuccessRebootRequired:
			return false
		}
	}

	return true
}

func withUser(s string, user string) string {
	return strings.ReplaceAll(s, "[USER]", user)
}

func wslInvoke(dist string, arg ...string) error {
	newArgs := make([]string, 0, 4+len(arg))
	newArgs = append(newArgs, "-u", "root", "-d", dist)
	newArgs = append(newArgs, arg...)
	cmd := wutil.NewWSLCommand(newArgs...)
	return runCmdPassThrough(cmd)
}

func wslPipe(input string, dist string, arg ...string) error {
	newArgs := make([]string, 0, 4+len(arg))
	newArgs = append(newArgs, "-u", "root", "-d", dist)
	newArgs = append(newArgs, arg...)
	cmd := wutil.NewWSLCommand(newArgs...)
	return pipeCmdPassThrough(cmd, input)
}

func runCmdPassThrough(cmd *exec.Cmd) error {
	logrus.Debugf("Running command: %s %v", cmd.Path, cmd.Args)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w", cmd.Path, cmd.Args, err)
	}
	return nil
}

func runCmdPassThroughTee(out io.Writer, cmd *exec.Cmd) error {
	logrus.Debugf("Running command: %s %v", cmd.Path, cmd.Args)

	// TODO - Perhaps improve this with a conpty pseudo console so that
	//        dism installer text bars mirror console behavior (redraw)
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, out)
	cmd.Stderr = io.MultiWriter(os.Stderr, out)
	if err := cmd.Run(); isMsiError(err) {
		return fmt.Errorf("command %s %v failed: %w", cmd.Path, cmd.Args, err)
	}
	return nil
}

func pipeCmdPassThrough(cmd *exec.Cmd, input string) error {
	logrus.Debugf("Running command: %s %v", cmd.Path, cmd.Args)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w", cmd.Path, cmd.Args, err)
	}
	return nil
}

func setupWslProxyEnv() (hasProxy bool) {
	current, _ := os.LookupEnv("WSLENV")
	for _, key := range config.ProxyEnv {
		if value, _ := os.LookupEnv(key); len(value) < 1 {
			continue
		}

		hasProxy = true
		delim := ""
		if len(current) > 0 {
			delim = ":"
		}
		current = fmt.Sprintf("%s%s%s/u", current, delim, key)
	}
	if hasProxy {
		os.Setenv("WSLENV", current)
	}
	return
}

//nolint:unused
func obtainGlobalConfigLock() (*fileLock, error) {
	lockDir, err := env.GetGlobalDataDir()
	if err != nil {
		return nil, err
	}

	// Lock file needs to be above all backends
	// TODO: This should be changed to a common.Config lock mechanism when available
	return lockFile(filepath.Join(lockDir, "podman-config.lck"))
}

func isWSLRunning(dist string) (bool, error) {
	return wslCheckExists(dist, true)
}

func isWSLExist(dist string) (bool, error) {
	return wslCheckExists(dist, false)
}

func wslCheckExists(dist string, running bool) (bool, error) {
	all, err := getAllWSLDistros(running)
	if err != nil {
		return false, err
	}

	_, exists := all[dist]
	return exists, nil
}

func getAllWSLDistros(running bool) (map[string]struct{}, error) {
	args := []string{"-l", "--quiet"}
	if running {
		args = append(args, "--running")
	}
	cmd := wutil.NewWSLCommand(args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s %v: %w", cmd.Path, args, err)
	}

	all := make(map[string]struct{})
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 0 {
			all[fields[0]] = struct{}{}
		}
	}

	err = cmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("command %s %v failed: %w (%s)", cmd.Path, args, err, strings.TrimSpace(stderr.String()))
	}

	return all, nil
}

func isSystemdRunning(dist string) (bool, error) {
	cmd := wutil.NewWSLCommand("-u", "root", "-d", dist, "sh")
	cmd.Stdin = strings.NewReader(sysdpid + "\necho $SYSDPID\n")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err = cmd.Start(); err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(out)
	result := false
	if scanner.Scan() {
		text := scanner.Text()
		i, err := strconv.Atoi(text)
		if err == nil && i > 0 {
			result = true
		}
	}

	err = cmd.Wait()
	if err != nil {
		return false, fmt.Errorf("command %s %v failed: %w (%s)", cmd.Path, cmd.Args[1:], err, strings.TrimSpace(stderr.String()))
	}

	return result, nil
}

func terminateDist(dist string) error {
	cmd := wutil.NewWSLCommand("--terminate", dist)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %s %v failed: %w (%s)", cmd.Path, cmd.Args[1:], err, strings.TrimSpace(string(out)))
	}
	return nil
}

func unregisterDist(dist string) error {
	cmd := wutil.NewWSLCommand("--unregister", dist)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %s %v failed: %w (%s)", cmd.Path, cmd.Args[1:], err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isRunning(name string) (bool, error) {
	dist := env.WithPodmanPrefix(name)
	wsl, err := isWSLRunning(dist)
	if err != nil {
		return false, err
	}

	sysd := false
	if wsl {
		sysd, err = isSystemdRunning(dist)

		if err != nil {
			return false, err
		}
	}

	return sysd, err
}

//nolint:unused
func getDiskSize(name string) strongunits.GiB {
	vmDataDir, err := env.GetDataDir(vmtype)
	if err != nil {
		return 0
	}
	distDir := filepath.Join(vmDataDir, "wsldist")
	disk := filepath.Join(distDir, name, "ext4.vhdx")
	info, err := os.Stat(disk)
	if err != nil {
		return 0
	}
	return strongunits.ToGiB(strongunits.B(info.Size()))
}

//nolint:unused
func getCPUs(name string) (uint64, error) {
	dist := env.WithPodmanPrefix(name)
	if run, _ := isWSLRunning(dist); !run {
		return 0, nil
	}
	cmd := wutil.NewWSLCommand("-u", "root", "-d", dist, "nproc")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	cmd.Env = []string{"WSL_UTF8=1"}
	if err = cmd.Start(); err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(out)
	var result string
	for scanner.Scan() {
		result = scanner.Text()
	}
	err = cmd.Wait()
	if err != nil {
		return 0, fmt.Errorf("command %s %v failed: %w (%s)", cmd.Path, cmd.Args[1:], err, strings.TrimSpace(strings.TrimSpace(stderr.String())))
	}

	ret, err := strconv.Atoi(result)
	return uint64(ret), err
}

//nolint:unused
func getMem(name string) (strongunits.MiB, error) {
	dist := env.WithPodmanPrefix(name)
	if run, _ := isWSLRunning(dist); !run {
		return 0, nil
	}
	cmd := wutil.NewWSLCommand("-u", "root", "-d", dist, "cat", "/proc/meminfo")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err = cmd.Start(); err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(out)
	var (
		total, available uint64
		t, a             int
	)
	for scanner.Scan() {
		// fields are in kB so div to mb
		fields := strings.Fields(scanner.Text())
		if strings.HasPrefix(fields[0], "MemTotal") && len(fields) >= 2 {
			t, err = strconv.Atoi(fields[1])
			total = uint64(t) / 1024
		} else if strings.HasPrefix(fields[0], "MemAvailable") && len(fields) >= 2 {
			a, err = strconv.Atoi(fields[1])
			available = uint64(a) / 1024
		}
		if err != nil {
			break
		}
	}
	err = cmd.Wait()
	if err != nil {
		return 0, fmt.Errorf("command %s %v failed: %w (%s)", cmd.Path, cmd.Args[1:], err, strings.TrimSpace(stderr.String()))
	}

	return strongunits.MiB(total - available), err
}

//nolint:unused
func getResources(mc *vmconfigs.MachineConfig) (resources vmconfigs.ResourceConfig) {
	resources.CPUs, _ = getCPUs(mc.Name)
	resources.Memory, _ = getMem(mc.Name)
	resources.DiskSize = getDiskSize(mc.Name)
	return
}
