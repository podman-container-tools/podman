package vmconfigs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/strongunits"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/podman/v6/pkg/machine/connection"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/lock"
	"go.podman.io/podman/v6/pkg/machine/ports"
	"go.podman.io/storage/pkg/fileutils"
	"go.podman.io/storage/pkg/ioutils"
	"go.podman.io/storage/pkg/lockfile"
)

/*
  info        Display machine host info					common
  init        Initialize a virtual machine				specific
  inspect     Inspect an existing machine				specific
  list        List machines								specific
  os          Manage a Podman virtual machine's OS		common
  rm          Remove an existing machine				specific
  set         Set a virtual machine setting				specific
  ssh         SSH into an existing machine				common
  start       Start an existing machine					specific
  stop        Stop an existing machine					specific
*/

var (
	SSHRemoteConnection     RemoteConnectionType = "ssh"
	DefaultIgnitionUserName                      = "core"
	ForwarderBinaryName                          = "gvproxy"
)

type RemoteConnectionType string

// NewMachineConfig creates the initial machine configuration file from cli options.
func NewMachineConfig(opts define.InitOptions, dirs *define.MachineDirs, sshIdentityPath string, vmtype define.VMType, machineLock *lockfile.LockFile) (*MachineConfig, error) {
	mc := new(MachineConfig)
	mc.Name = opts.Name
	mc.dirs = dirs
	mc.lock = machineLock

	// Assign Dirs
	cf, err := define.NewMachineFile(filepath.Join(dirs.ConfigDir.GetPath(), fmt.Sprintf("%s.json", opts.Name)), nil)
	if err != nil {
		return nil, err
	}
	mc.configPath = cf
	// Given that we are locked now and check again that the config file does not exists,
	// if it does it means the VM was already created and we should error.
	if err := fileutils.Exists(cf.Path); err == nil {
		return nil, &define.ErrVMAlreadyExists{Name: opts.Name}
	}

	if vmtype != define.QemuVirt && len(opts.USBs) > 0 {
		return nil, fmt.Errorf("USB host passthrough not supported for %s machines", vmtype)
	}

	usbs, err := define.ParseUSBs(opts.USBs)
	if err != nil {
		return nil, err
	}

	// System Resources
	mrc := ResourceConfig{
		CPUs:     opts.CPUS,
		DiskSize: strongunits.GiB(opts.DiskSize),
		Memory:   strongunits.MiB(opts.Memory),
		USBs:     usbs,
	}
	mc.Resources = mrc

	if opts.Swap > 0 {
		mc.Swap = strongunits.MiB(opts.Swap)
	}

	sshPort, err := ports.AllocateMachinePort()
	if err != nil {
		return nil, err
	}

	sshConfig := SSHConfig{
		IdentityPath:   sshIdentityPath,
		Port:           sshPort,
		RemoteUsername: opts.Username,
	}

	mc.SSH = sshConfig
	mc.Created = time.Now()

	mc.HostUser = HostUser{UID: getHostUID(), Rootful: opts.Rootful}

	return mc, nil
}

// Lock creates a lock on the machine for single access
func (mc *MachineConfig) Lock() {
	mc.lock.Lock()
}

// Unlock removes an existing lock
func (mc *MachineConfig) Unlock() {
	mc.lock.Unlock()
}

// Refresh reloads the config file from disk
func (mc *MachineConfig) Refresh() error {
	content, err := os.ReadFile(mc.configPath.GetPath())
	if err != nil {
		return err
	}
	return json.Unmarshal(content, mc)
}

// write is a non-locking way to write the machine configuration file to disk
func (mc *MachineConfig) Write() error {
	if mc.configPath == nil {
		return fmt.Errorf("no configuration file associated with vm %q", mc.Name)
	}
	b, err := json.Marshal(mc)
	if err != nil {
		return err
	}
	logrus.Debugf("writing configuration file %q", mc.configPath.Path)
	return ioutils.AtomicWriteFile(mc.configPath.GetPath(), b, define.DefaultFilePerm)
}

func (mc *MachineConfig) SetRootful(rootful bool) error {
	if err := connection.UpdateConnectionIfDefault(rootful, mc.Name, mc.Name+"-root"); err != nil {
		return err
	}
	mc.HostUser.Rootful = rootful
	mc.HostUser.Modified = true
	return nil
}

func (mc *MachineConfig) Remove(machines map[string]bool, saveIgnition, saveImage bool) ([]string, func() error, error) {
	ignitionFile, err := mc.IgnitionFile()
	if err != nil {
		return nil, nil, err
	}

	readySocket, err := mc.ReadySocket()
	if err != nil {
		return nil, nil, err
	}

	gvProxySocket, err := mc.GVProxySocket()
	if err != nil {
		return nil, nil, err
	}

	apiSocket, err := mc.APISocket()
	if err != nil {
		return nil, nil, err
	}

	logPath, err := mc.LogFile()
	if err != nil {
		return nil, nil, err
	}

	rmFiles := []string{
		mc.configPath.GetPath(),
		readySocket.GetPath(),
		gvProxySocket.GetPath(),
		apiSocket.GetPath(),
		logPath.GetPath(),
	}
	if !saveImage {
		mc.ImagePath.GetPath()
	}
	if !saveIgnition {
		ignitionFile.GetPath()
	}

	mcRemove := func() error {
		var errs []error
		if err := connection.RemoveConnections(machines, mc.Name, mc.Name+"-root"); err != nil {
			errs = append(errs, err)
		}

		if !saveIgnition {
			if err := ignitionFile.Delete(); err != nil {
				errs = append(errs, err)
			}
		}
		if !saveImage {
			if err := mc.ImagePath.Delete(); err != nil {
				errs = append(errs, err)
			}
		}
		if err := readySocket.Delete(); err != nil {
			errs = append(errs, err)
		}
		if err := gvProxySocket.Delete(); err != nil {
			errs = append(errs, err)
		}
		if err := apiSocket.Delete(); err != nil {
			errs = append(errs, err)
		}
		if err := logPath.Delete(); err != nil {
			errs = append(errs, err)
		}

		if err := mc.configPath.Delete(); err != nil {
			errs = append(errs, err)
		}

		if err := ports.ReleaseMachinePort(mc.SSH.Port); err != nil {
			errs = append(errs, err)
		}

		return errorhandling.JoinErrors(errs)
	}

	return rmFiles, mcRemove, nil
}

// ConfigDir is a simple helper to obtain the machine config dir
func (mc *MachineConfig) ConfigDir() (*define.VMFile, error) {
	if mc.dirs == nil || mc.dirs.ConfigDir == nil {
		return nil, errors.New("no configuration directory set")
	}
	return mc.dirs.ConfigDir, nil
}

// DataDir is a simple helper function to obtain the machine data dir
func (mc *MachineConfig) DataDir() (*define.VMFile, error) {
	if mc.dirs == nil || mc.dirs.DataDir == nil {
		return nil, errors.New("no data directory set")
	}
	return mc.dirs.DataDir, nil
}

// RuntimeDir is simple helper function to obtain the runtime dir
func (mc *MachineConfig) RuntimeDir() (*define.VMFile, error) {
	if mc.dirs == nil || mc.dirs.RuntimeDir == nil {
		return nil, errors.New("no runtime directory set")
	}
	return mc.dirs.RuntimeDir, nil
}

func (mc *MachineConfig) SetDirs(dirs *define.MachineDirs) {
	mc.dirs = dirs
}

func (mc *MachineConfig) IgnitionFile() (*define.VMFile, error) {
	configDir, err := mc.ConfigDir()
	if err != nil {
		return nil, err
	}
	return configDir.AppendToNewVMFile(mc.Name+".ign", nil)
}

func (mc *MachineConfig) ReadySocket() (*define.VMFile, error) {
	rtDir, err := mc.RuntimeDir()
	if err != nil {
		return nil, err
	}
	return readySocket(mc.Name, rtDir)
}

func (mc *MachineConfig) GVProxySocket() (*define.VMFile, error) {
	machineRuntimeDir, err := mc.RuntimeDir()
	if err != nil {
		return nil, err
	}
	return gvProxySocket(mc.Name, machineRuntimeDir)
}

func (mc *MachineConfig) APISocket() (*define.VMFile, error) {
	machineRuntimeDir, err := mc.RuntimeDir()
	if err != nil {
		return nil, err
	}
	return apiSocket(mc.Name, machineRuntimeDir)
}

func (mc *MachineConfig) LogFile() (*define.VMFile, error) {
	rtDir, err := mc.RuntimeDir()
	if err != nil {
		return nil, err
	}
	return rtDir.AppendToNewVMFile(mc.Name+".log", nil)
}

func (mc *MachineConfig) IsFirstBoot() bool {
	return mc.LastUp.IsZero()
}

func (mc *MachineConfig) ConnectionInfo(_ define.VMType) (*define.VMFile, *define.VMFile, error) {
	socket, err := mc.APISocket()
	return socket, getPipe(mc.Name), err
}

// LoadMachineByName returns a machine config based on the vm name and provider
func LoadMachineByName(name string, dirs *define.MachineDirs) (*MachineConfig, error) {
	fullPath, err := dirs.ConfigDir.AppendToNewVMFile(name+".json", nil)
	if err != nil {
		return nil, err
	}
	mc, err := loadMachineFromFQPath(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &define.ErrVMDoesNotExist{Name: name}
		}
		return nil, err
	}
	mc.dirs = dirs
	mc.configPath = fullPath

	// If we find an incompatible configuration, we return a hard
	// error because the user wants to deal directly with this
	// machine
	if mc.Version == 0 {
		return mc, &define.ErrIncompatibleMachineConfig{
			Name: name,
			Path: fullPath.GetPath(),
		}
	}
	return mc, nil
}

// loadMachineFromFQPath stub function for loading a JSON configuration file and returning
// a machineconfig.  this should only be called if you know what you are doing.
func loadMachineFromFQPath(path *define.VMFile) (*MachineConfig, error) {
	mc := new(MachineConfig)
	b, err := path.Read()
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(b, mc); err != nil {
		return nil, fmt.Errorf("unable to load machine config file: %q", err)
	}
	lock, err := lock.GetMachineLock(mc.Name, filepath.Dir(path.GetPath()))
	mc.lock = lock
	return mc, err
}

// LoadMachinesInDir returns all the machineconfigs located in given dir
func LoadMachinesInDir(dirs *define.MachineDirs) (map[string]*MachineConfig, error) {
	mcs := make(map[string]*MachineConfig)
	if err := filepath.WalkDir(dirs.ConfigDir.GetPath(), func(_ string, d fs.DirEntry, _ error) error {
		if strings.HasSuffix(d.Name(), ".json") {
			fullPath, err := dirs.ConfigDir.AppendToNewVMFile(d.Name(), nil)
			if err != nil {
				return err
			}
			mc, err := loadMachineFromFQPath(fullPath)
			if err != nil {
				return err
			}
			// if we find an incompatible machine configuration file, we emit and error
			//
			if mc.Version == 0 {
				tmpErr := &define.ErrIncompatibleMachineConfig{
					Name: mc.Name,
					Path: fullPath.GetPath(),
				}
				logrus.Error(tmpErr)
				return nil
			}
			mc.configPath = fullPath
			mc.dirs = dirs
			mcs[mc.Name] = mc
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return mcs, nil
}
