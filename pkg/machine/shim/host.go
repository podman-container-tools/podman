package shim

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/certificates"
	"go.podman.io/podman/v6/pkg/machine/connection"
	machineDefine "go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/podman/v6/pkg/machine/ignition"
	"go.podman.io/podman/v6/pkg/machine/lock"
	"go.podman.io/podman/v6/pkg/machine/provider"
	"go.podman.io/podman/v6/pkg/machine/proxyenv"
	"go.podman.io/podman/v6/pkg/machine/shim/diskpull"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/podman/v6/utils"
	"golang.org/x/term"
)

// List is done at the host level to allow for a *possible* future where
// more than one provider is used
func List(vmstubbers []vmconfigs.VMProvider, _ machine.ListOptions) ([]*machine.ListResponse, error) {
	lrs := make([]*machine.ListResponse, 0)
	mcs, err := getMCsOverProviders(vmstubbers)
	if err != nil {
		return nil, err
	}
	for name, mc := range mcs {
		// set bypass=true so it doesn't fail the entire list if one provider can't determine state
		state, err := mc.Provider.State(mc.MachineConfig, true)
		if err != nil {
			return nil, err
		}
		lr := machine.ListResponse{
			Name:               name,
			CreatedAt:          mc.Created,
			LastUp:             mc.LastUp,
			Running:            state == machineDefine.Running,
			Starting:           mc.Starting,
			VMType:             mc.Provider.VMType().String(),
			CPUs:               mc.Resources.CPUs,
			Memory:             mc.Resources.Memory,
			Swap:               mc.Swap,
			DiskSize:           mc.Resources.DiskSize,
			Port:               mc.SSH.Port,
			RemoteUsername:     mc.SSH.RemoteUsername,
			IdentityPath:       mc.SSH.IdentityPath,
			UserModeNetworking: mc.Provider.UserModeNetworkEnabled(mc.MachineConfig),
		}

		lrs = append(lrs, &lr)
	}
	return lrs, nil
}

func Init(opts machineDefine.InitOptions, mp vmconfigs.VMProvider) error {
	var (
		err            error
		imageExtension string
		imagePath      *machineDefine.VMFile
	)

	callbackFuncs := machine.CleanUp()
	defer func() {
		// ErrRelaunchSucceeded is not a real error: it signals that
		// an elevated child process completed the operation successfully.
		// Skip cleanup so we don't remove resources (e.g. the disk image)
		// that the child process created and that are now in use.
		if !errors.Is(err, machineDefine.ErrRelaunchSucceeded) {
			callbackFuncs.CleanIfErr(&err)
		}
	}()
	go callbackFuncs.CleanOnSignal()

	dirs, err := env.GetMachineDirs(mp.VMType())
	if err != nil {
		return err
	}

	sshIdentityPath, err := env.GetSSHIdentityPath(machineDefine.DefaultIdentityName)
	if err != nil {
		return err
	}
	sshKey, err := machine.GetSSHKeys(sshIdentityPath)
	if err != nil {
		return err
	}

	machineLock, err := lock.GetMachineLock(opts.Name, dirs.ConfigDir.GetPath())
	if err != nil {
		return err
	}

	// If the machine is being re-launched, the lock is already held
	if !opts.ReExec {
		machineLock.Lock()
		defer machineLock.Unlock()
	}

	mc, err := vmconfigs.NewMachineConfig(opts, dirs, sshIdentityPath, mp.VMType(), machineLock)
	if err != nil {
		return err
	}

	mc.Version = vmconfigs.MachineConfigVersion
	mc.ImportNativeCA = opts.ImportNativeCA

	createOpts := machineDefine.CreateVMOpts{
		Name:   opts.Name,
		Dirs:   dirs,
		ReExec: opts.ReExec,
	}

	if umn := opts.UserModeNetworking; umn != nil {
		createOpts.UserModeNetworking = *umn
	}

	// Mounts
	if mp.VMType() != machineDefine.WSLVirt {
		mc.Mounts = CmdLineVolumesToMounts(opts.Volumes, mp.MountType())
	}

	// Issue #18230 ... do not mount over important directories at the / level (subdirs are fine)
	for _, mnt := range mc.Mounts {
		if err := validateDestinationPaths(mnt.Target); err != nil {
			return err
		}
	}

	// Get Image
	// TODO This needs rework bigtime; my preference is most of below of not living in here.
	// ideally we could get a func back that pulls the image, and only do so IF everything works because
	// image stuff is the slowest part of the operation

	// This is a break from before.  New images are named vmname-ARCH.
	// It turns out that Windows/HyperV will not accept a disk that
	// is not suffixed as ".vhdx". Go figure
	switch mp.VMType() {
	case machineDefine.QemuVirt:
		imageExtension = ".qcow2"
	case machineDefine.AppleHvVirt, machineDefine.LibKrun:
		imageExtension = ".raw"
	case machineDefine.HyperVVirt:
		imageExtension = ".vhdx"
	case machineDefine.WSLVirt:
		imageExtension = ""
	default:
		return fmt.Errorf("unknown VM type: %s", mp.VMType())
	}

	imagePath, err = dirs.DataDir.AppendToNewVMFile(fmt.Sprintf("%s-%s%s", opts.Name, runtime.GOARCH, imageExtension), nil)
	if err != nil {
		return err
	}
	mc.ImagePath = imagePath

	// If the process was re-executed with elevation, the image has already been pulled
	// in the parent process, so skip disk pulling here.
	if !opts.ReExec {
		// TODO The following stanzas should be re-written in a differeent place.  It should have a custom
		// parser for our image pulling.  It would be nice if init just got an error and mydisk back.
		//
		// Eventual valid input:
		// "" <- means take the default
		// "http|https://path"
		// "/path
		// "docker://quay.io/something/someManifest
		if err := diskpull.GetDisk(opts.Image, dirs, mc.ImagePath, mp.VMType(), mc.Name, opts.SkipTlsVerify); err != nil {
			return err
		}
		callbackFuncs.Add(mc.ImagePath.Delete)
	}

	logrus.Debugf("imagePath is %q", imagePath.GetPath())

	ignitionFile, err := mc.IgnitionFile()
	if err != nil {
		return err
	}

	uid := os.Getuid()
	if uid == -1 { // windows compensation
		uid = 1000
	}

	// TODO the definition of "user" should go into
	// common for WSL
	userName := opts.Username
	if mp.VMType() == machineDefine.WSLVirt {
		if opts.Username == "core" {
			userName = "user"
			mc.SSH.RemoteUsername = "user"
		}
	}

	ignBuilder := ignition.NewIgnitionBuilder(ignition.DynamicIgnition{
		Name:      userName,
		Key:       sshKey,
		TimeZone:  opts.TimeZone,
		UID:       uid,
		VMName:    opts.Name,
		VMType:    mp.VMType(),
		WritePath: ignitionFile.GetPath(),
		Rootful:   opts.Rootful,
		Swap:      opts.Swap,
	})

	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		err = ignBuilder.BuildWithIgnitionFile(opts.IgnitionPath)
		if err != nil {
			return err
		}
	} else {
		err = ignBuilder.GenerateIgnitionConfig()
		if err != nil {
			return err
		}
	}

	if len(opts.PlaybookPath) > 0 {
		f, err := os.Open(opts.PlaybookPath)
		if err != nil {
			return err
		}
		defer f.Close()
		s, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("read playbook: %w", err)
		}

		playbookDest := fmt.Sprintf("/home/%s/%s", userName, "playbook.yaml")

		if mp.VMType() != machineDefine.WSLVirt {
			err = ignBuilder.AddPlaybook(string(s), playbookDest, userName)
			if err != nil {
				return err
			}
		}

		mc.Ansible = &vmconfigs.AnsibleConfig{
			PlaybookPath: playbookDest,
			Contents:     string(s),
			User:         userName,
		}
	}

	readyIgnOpts, err := mp.PrepareIgnition(mc, &ignBuilder)
	if err != nil {
		return err
	}

	readyUnitFile, err := ignition.CreateReadyUnitFile(mp.VMType(), readyIgnOpts)
	if err != nil {
		return err
	}

	readyUnit := ignition.Unit{
		Enabled:  ignition.BoolToPtr(true),
		Name:     "ready.service",
		Contents: ignition.StrToPtr(readyUnitFile),
	}
	ignBuilder.WithUnit(readyUnit)

	// CreateVM could cause the init command to be re-launched in some cases (e.g. wsl)
	// so we need to avoid creating the machine config or connections before this check happens.
	// when relaunching, the invoked 'init' command will be responsible to set up the machine
	err = mp.CreateVM(createOpts, mc, &ignBuilder)
	if err != nil {
		return err
	}

	// TODO AddSSHConnectionToPodmanSocket could take an machineconfig instead
	if err := connection.AddSSHConnectionsToPodmanSocket(mc.HostUser.UID, mc.SSH.Port, mc.SSH.IdentityPath, mc.Name, mc.SSH.RemoteUsername, opts); err != nil {
		return err
	}

	cleanup := func() error {
		machines, err := provider.GetAllMachinesAndRootfulness()
		if err != nil {
			return err
		}
		return connection.RemoveConnections(machines, mc.Name, mc.Name+"-root")
	}
	callbackFuncs.Add(cleanup)

	if len(opts.IgnitionPath) == 0 {
		if err := ignBuilder.Build(); err != nil {
			return err
		}
	}

	return mc.Write()
}

type MultiProvider []vmconfigs.VMProvider

// VMExists looks across given providers for a machine's existence.  returns the actual config and found bool
func VMExists(name string) (*vmconfigs.MachineConfig, vmconfigs.VMProvider, error) {
	// Look on disk first
	mcs, err := getMCsOverProviders(provider.GetAll())
	if err != nil {
		return nil, nil, err
	}
	if mc, found := mcs[name]; found {
		vmType := mc.Provider
		return mc.MachineConfig, vmType, nil
	}

	return nil, nil, &machineDefine.ErrVMDoesNotExist{Name: name}
}

// VMExistsOnHyperVisor actually checks the managing hypervisor (like WSL, HyperV)
// to make sure a VM with the same name does not happen to exist.
func VMExistsOnHyperVisor(name string) (bool, error) {
	providers := provider.GetAll()
	for _, p := range providers {
		exists, err := p.Exists(name)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

// checkExclusiveActiveVM checks if any of the machines are already running
func checkExclusiveActiveVM(currentProvider vmconfigs.VMProvider, mc *vmconfigs.MachineConfig) error {
	providers := provider.GetAll()
	// Check if any other machines are running; if so, we error
	localMachines, err := getMCsOverProviders(providers)
	if err != nil {
		return err
	}

	for name, localMachine := range localMachines {
		state, err := localMachine.Provider.State(localMachine.MachineConfig, false)
		if err != nil {
			return err
		}
		if state == machineDefine.Running || state == machineDefine.Starting {
			if mc.Name == name {
				return fmt.Errorf("unable to start %q: already running", mc.Name)
			}

			// A machine is running in the current provider
			if currentProvider.VMType() == localMachine.Provider.VMType() {
				fail := machineDefine.ErrMultipleActiveVM{Name: name}
				return fmt.Errorf("unable to start %q: %w", mc.Name, &fail)
			}
			// A machine is running in an alternate provider
			fail := machineDefine.ErrMultipleActiveVM{Name: name, Provider: localMachine.Provider.VMType().String()}
			return fmt.Errorf("unable to start: %w", &fail)
		}
	}
	return nil
}

type knownMachineConfig struct {
	Provider vmconfigs.VMProvider
	*vmconfigs.MachineConfig
}

// getMCsOverProviders loads machineconfigs from a config dir derived from the "provider".  it returns only what is known on
// disk so things like status may be incomplete or inaccurate
func getMCsOverProviders(vmstubbers []vmconfigs.VMProvider) (map[string]knownMachineConfig, error) {
	mcs := make(map[string]knownMachineConfig)
	for _, stubber := range vmstubbers {
		dirs, err := env.GetMachineDirs(stubber.VMType())
		if err != nil {
			return nil, err
		}
		stubberMCs, err := vmconfigs.LoadMachinesInDir(dirs)
		if err != nil {
			return nil, err
		}
		for mcName, mc := range stubberMCs {
			if _, ok := mcs[mcName]; !ok {
				mcs[mcName] = knownMachineConfig{Provider: stubber, MachineConfig: mc}
			}
		}
	}
	return mcs, nil
}

// Stop stops the machine as well as supporting binaries/processes
func Stop(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, hardStop bool) error {
	dirs, err := env.GetMachineDirs(mp.VMType())
	if err != nil {
		return err
	}
	// state is checked here instead of earlier because stopping a stopped vm is not considered
	// an error.  so putting in one place instead of sprinkling all over.
	mc.Lock()
	defer mc.Unlock()
	if err := mc.Refresh(); err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	return stopLocked(mc, mp, dirs, hardStop)
}

// stopLocked stops the machine and expects the caller to hold the machine's lock.
func stopLocked(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, dirs *machineDefine.MachineDirs, hardStop bool) error {
	state, err := mp.State(mc, false)
	if err != nil {
		return err
	}
	// stopping a stopped machine is NOT an error
	if state == machineDefine.Stopped {
		return nil
	}
	if state != machineDefine.Running {
		return machineDefine.ErrWrongState
	}

	// Provider stops the machine
	if err := mp.StopVM(mc, hardStop); err != nil {
		return err
	}

	// Remove Ready Socket
	readySocket, err := mc.ReadySocket()
	if err != nil {
		return err
	}
	if err := readySocket.Delete(); err != nil {
		return err
	}

	// Stop GvProxy and remove PID file
	if !mp.UseProviderNetworkSetup() {
		gvproxyPidFile, err := dirs.RuntimeDir.AppendToNewVMFile("gvproxy.pid", nil)
		if err != nil {
			return err
		}
		if err := machine.CleanupGVProxy(*gvproxyPidFile); err != nil {
			return fmt.Errorf("unable to clean up gvproxy: %w", err)
		}
	}

	// Update last time up
	mc.LastUp = time.Now()
	return mc.Write()
}

func Start(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, opts machine.StartOptions, updateSystemConn *bool) error {
	var updateDefaultConnection bool

	defaultBackoff := 500 * time.Millisecond
	maxBackoffs := 6
	signalChanClosed := false

	dirs, err := env.GetMachineDirs(mp.VMType())
	if err != nil {
		return err
	}
	if !opts.ReExec {
		mc.Lock()
		defer mc.Unlock()
	}
	if err := mc.Refresh(); err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	connName := mc.Name
	if mc.HostUser.Rootful {
		connName += "-root"
	}
	conn, err := registry.PodmanConfig().ContainersConfDefaultsRO.GetConnection(connName, false)
	if err != nil {
		return err
	}

	// Don't check if provider supports parallel running machines
	if mp.RequireExclusiveActive() {
		startLock, err := lock.GetMachineStartLock()
		if err != nil {
			return err
		}
		startLock.Lock()
		defer startLock.Unlock()

		if err := checkExclusiveActiveVM(mp, mc); err != nil {
			return err
		}
	} else {
		// still should make sure we do not start the same machine twice
		state, err := mp.State(mc, false)
		if err != nil {
			return err
		}

		if state == machineDefine.Running || state == machineDefine.Starting {
			return fmt.Errorf("unable to start %q: already running", mc.Name)
		}
	}

	// Do not do anything with the system connection if its already
	// the default system connection.
	if !conn.Default {
		if updateSystemConn != nil {
			updateDefaultConnection = *updateSystemConn
		} else if term.IsTerminal(int(os.Stdin.Fd())) {
			// Prompt for system connection update if there is a terminal
			// on stdin
			response, err := promptUpdateSystemConn()
			if err != nil {
				return err
			}
			// This might be kind of lame but when using the command, but if you don't
			// provide some sort of visual cue back to the user, it's unclear what is
			// going on because the machine startup is going and it looks like things
			// are frozen
			if response {
				fmt.Printf("\nDefault system connection will be changed to %q\n ", connName)
			} else {
				fmt.Println("Default system connection will remain unchanged")
			}
			updateDefaultConnection = response
		}
	}

	// if the machine cannot continue starting due to a signal, ensure the state
	// reflects the machine is no longer starting
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGPIPE)
	go func() {
		sig, ok := <-signalChan
		if ok {
			mc.Starting = false
			logrus.Error("signal received when starting the machine: ", sig)

			if err := mc.Write(); err != nil {
				logrus.Error(err)
			}

			os.Exit(1)
		}
	}()

	// Set starting to true
	mc.Starting = true
	if err := mc.Write(); err != nil {
		logrus.Error(err)
	}

	// Set starting to false on exit
	defer func() {
		mc.Starting = false
		if err := mc.Write(); err != nil {
			logrus.Error(err)
		}

		if !signalChanClosed {
			signal.Stop(signalChan)
			close(signalChan)
		}
	}()

	gvproxyPidFile, err := dirs.RuntimeDir.AppendToNewVMFile("gvproxy.pid", nil)
	if err != nil {
		return err
	}

	// start gvproxy and set up the API socket forwarding
	forwardSocketPath, forwardingState, err := startNetworking(mc, mp)
	if err != nil {
		return err
	}

	callBackFuncs := machine.CleanUp()
	defer callBackFuncs.CleanIfErr(&err)
	go callBackFuncs.CleanOnSignal()

	// Clean up gvproxy if start fails
	cleanGV := func() error {
		return machine.CleanupGVProxy(*gvproxyPidFile)
	}
	callBackFuncs.Add(cleanGV)

	// if there are generic things that need to be done, a preStart function could be added here
	// should it be extensive

	// releaseFunc is if the provider starts a vm using a go command
	// and we still need control of it while it is booting until the ready
	// socket is tripped
	releaseCmd, WaitForReady, err := mp.StartVM(mc)
	if err != nil {
		return err
	}

	if WaitForReady == nil {
		return errors.New("no valid wait function returned")
	}

	if err := WaitForReady(); err != nil {
		return err
	}

	if releaseCmd != nil && releaseCmd() != nil { // some providers can return nil here (hyperv)
		if err := releaseCmd(); err != nil {
			// I think it is ok for a "light" error?
			logrus.Error(err)
		}
	}

	if !opts.NoInfo && !mc.HostUser.Rootful {
		machine.PrintRootlessWarning(mc.Name)
	}

	err = mp.PostStartNetworking(mc, opts.NoInfo)
	if err != nil {
		return err
	}

	stateF := func() (machineDefine.Status, error) {
		return mp.State(mc, true)
	}

	connected, sshError, err := conductVMReadinessCheck(mc, maxBackoffs, defaultBackoff, stateF)
	if err != nil {
		return err
	}

	if !connected {
		msg := "machine did not transition into running state"
		if sshError != nil {
			return fmt.Errorf("%s: ssh error: %v", msg, sshError)
		}
		return errors.New(msg)
	}

	// now that the machine has transitioned into the running state, we don't need a goroutine listening for SIGINT or SIGTERM to handle state
	signal.Stop(signalChan)
	close(signalChan)
	signalChanClosed = true

	if err := proxyenv.ApplyProxies(mc); err != nil {
		return err
	}

	// mount the volumes to the VM
	if err := mp.MountVolumesToVM(mc, opts.Quiet); err != nil {
		return err
	}

	// Import native CA certificates if enabled (must run after volumes are
	// mounted so the certificate file is accessible via the mounted path)
	if mc.ImportNativeCA {
		if err := certificates.ImportNativeCertificates(mc, mp.VMType()); err != nil {
			// Warn the user but continue the machine startup process
			logrus.Warnf("Failed to import native CA certificates: %v", err)
			fmt.Println("Warning: Failed to import host trusted CA certificates. The machine will start without them.")
		} else if !opts.Quiet {
			fmt.Println("The host trusted CA certificates have been imported successfully")
		}
	}

	// update the podman/docker socket service if the host user has been modified at all (UID or Rootful)
	if mc.HostUser.Modified {
		if machine.UpdatePodmanDockerSockService(mc) == nil {
			// Reset modification state if there are no errors, otherwise ignore errors
			// which are already logged
			mc.HostUser.Modified = false
			if err := mc.Write(); err != nil {
				logrus.Error(err)
			}
		}
	}

	if mp.VMType() == machineDefine.WSLVirt && mc.Ansible != nil && mc.IsFirstBoot() {
		if err := machine.LocalhostSSHSilent(mc.Ansible.User, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, []string{"ansible-playbook", mc.Ansible.PlaybookPath}); err != nil {
			logrus.Error(err)
		}
	}

	// embedded function for handling if we should update the default connection
	// and the actual update
	updateConnectionFunc := func() error {
		if !updateDefaultConnection {
			return nil
		}
		return config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
			logrus.Infof("Setting default Podman connection to %s", connName)
			cfg.Connection.Default = connName
			return nil
		})
	}
	// Provider is responsible for waiting
	if mp.UseProviderNetworkSetup() {
		return updateConnectionFunc()
	}

	noInfo := opts.NoInfo

	machine.WaitAPIAndPrintInfo(
		forwardingState,
		mc.Name,
		findClaimHelper(),
		forwardSocketPath,
		noInfo,
		mc.HostUser.Rootful,
	)
	return updateConnectionFunc()
}

func Set(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, opts machineDefine.SetOptions) error {
	mc.Lock()
	defer mc.Unlock()

	if err := mc.Refresh(); err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	if opts.CPUs != nil {
		mc.Resources.CPUs = *opts.CPUs
	}

	if opts.Memory != nil {
		mc.Resources.Memory = *opts.Memory
	}

	if opts.DiskSize != nil {
		if *opts.DiskSize <= mc.Resources.DiskSize {
			return fmt.Errorf("new disk size must be larger than %d GB", mc.Resources.DiskSize)
		}
		mc.Resources.DiskSize = *opts.DiskSize
	}

	if opts.ImportNativeCA != nil {
		mc.ImportNativeCA = *opts.ImportNativeCA
	}

	if err := mp.SetProviderAttrs(mc, opts); err != nil {
		return err
	}

	// Update the configuration file last if everything earlier worked
	return mc.Write()
}

func Remove(mc *vmconfigs.MachineConfig, mp vmconfigs.VMProvider, opts machine.RemoveOptions) error {
	dirs, err := env.GetMachineDirs(mp.VMType())
	if err != nil {
		return err
	}
	if !opts.ReExec {
		mc.Lock()
		defer mc.Unlock()
	}
	if err := mc.Refresh(); err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	state, err := mp.State(mc, false)
	if err != nil {
		return err
	}

	if state == machineDefine.Running {
		if !opts.Force {
			return &machineDefine.ErrVMRunningCannotDestroyed{Name: mc.Name}
		}
	}

	machines, err := provider.GetAllMachinesAndRootfulness()
	if err != nil {
		return err
	}

	rmFiles, genericRm, err := mc.Remove(machines, opts.SaveIgnition, opts.SaveImage)
	if err != nil {
		return err
	}

	providerFiles, providerRm, err := mp.Remove(mc)
	if err != nil {
		return err
	}

	// Add provider specific files to the list
	rmFiles = append(rmFiles, providerFiles...)

	// Important!
	// Nothing can be removed at this point.  The user can still opt out below
	//

	if !opts.Force {
		// Warn user
		confirmationMessage(rmFiles)
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}

	if state == machineDefine.Running {
		if err := stopLocked(mc, mp, dirs, true); err != nil {
			return err
		}
	}

	//
	// All actual removal of files and vms should occur after this
	//

	if err := providerRm(); err != nil {
		logrus.Errorf("failed to remove virtual machine from provider for %q: %v", mc.Name, err)
	}

	if err := genericRm(); err != nil {
		return fmt.Errorf("failed to remove machines files: %v", err)
	}
	return nil
}

func confirmationMessage(files []string) {
	fmt.Printf("The following files will be deleted:\n\n\n")
	for _, msg := range files {
		fmt.Println(msg)
	}
}

func Reset(mps []vmconfigs.VMProvider, _ machine.ResetOptions) error {
	var resetErrors *multierror.Error
	removeDirs := []*machineDefine.MachineDirs{}

	for _, p := range mps {
		d, err := env.GetMachineDirs(p.VMType())
		if err != nil {
			resetErrors = multierror.Append(resetErrors, err)
			continue
		}
		mcs, err := vmconfigs.LoadMachinesInDir(d)
		if err != nil {
			resetErrors = multierror.Append(resetErrors, err)
			continue
		}
		removeDirs = append(removeDirs, d)

		machines, err := provider.GetAllMachinesAndRootfulness()
		if err != nil {
			return err
		}

		for _, mc := range mcs {
			err := Stop(mc, p, true)
			if err != nil {
				resetErrors = multierror.Append(resetErrors, err)
			}
			_, genericRm, err := mc.Remove(machines, false, false)
			if err != nil {
				resetErrors = multierror.Append(resetErrors, err)
			}
			_, providerRm, err := p.Remove(mc)
			if err != nil {
				resetErrors = multierror.Append(resetErrors, err)
			}

			if genericRm != nil {
				if err := genericRm(); err != nil {
					resetErrors = multierror.Append(resetErrors, err)
				}
			}
			// We must check if the returned providerRm function is not nil before executing it.
			// If a provider (like Hyper-V) encountered an error during the removal setup
			// phase (e.g., a cancelled 'runas' elevation), it returns nil for the function.
			if providerRm != nil {
				if err := providerRm(); err != nil {
					resetErrors = multierror.Append(resetErrors, err)
				}
			}
		}
	}

	// Delete the various directories
	// We do this after all the provider rm's, since providers may still share the base machine dir.
	// Note: we cannot delete the machine run dir blindly like this because
	// other things live there like the podman.socket and so forth.
	for _, dir := range removeDirs {
		// in linux this ~/.local/share/containers/podman/machine
		dataDirErr := utils.GuardedRemoveAll(filepath.Dir(dir.DataDir.GetPath()))
		if !errors.Is(dataDirErr, os.ErrNotExist) {
			resetErrors = multierror.Append(resetErrors, dataDirErr)
		}
		// in linux this ~/.config/containers/podman/machine
		confDirErr := utils.GuardedRemoveAll(filepath.Dir(dir.ConfigDir.GetPath()))
		if !errors.Is(confDirErr, os.ErrNotExist) {
			resetErrors = multierror.Append(resetErrors, confDirErr)
		}
	}
	return resetErrors.ErrorOrNil()
}

func validateDestinationPaths(dest string) error {
	// illegalMounts are locations at the / level of the podman machine where we do want users mounting directly over
	illegalMounts := map[string]struct{}{
		"/bin":                        {},
		"/boot":                       {},
		"/dev":                        {},
		"/etc":                        {},
		"/home":                       {},
		"/proc":                       {},
		"/root":                       {},
		"/run":                        {},
		"/sbin":                       {},
		"/sys":                        {},
		"/tmp":                        {},
		"/usr":                        {},
		"/var":                        {},
		"/var/lib/containers/storage": {},
	}
	mountTarget := path.Clean(dest)
	if _, ok := illegalMounts[mountTarget]; ok {
		return fmt.Errorf("machine mount destination cannot be %q: consider another location or a subdirectory of an existing location", mountTarget)
	}
	return nil
}

func promptUpdateSystemConn() (bool, error) {
	fmt.Println("Warning: The machine being started is not set as your default Podman connection.")
	fmt.Println("As such, Podman commands may not work correctly.")
	fmt.Print(`Set the default Podman connection to this machine? [y/N] `)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	return strings.ToLower(answer)[0] == 'y', nil
}
