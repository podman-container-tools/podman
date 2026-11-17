package e2e_test

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

type initMachine struct {
	playbook           string
	provider           string
	cpus               *uint
	diskSize           *uint
	swap               *uint
	ignitionPath       string
	username           string
	image              string
	memory             *uint
	now                bool
	timezone           string
	rootful            bool
	volumes            []string
	updateConnection   *bool
	userModeNetworking bool
	tlsVerify          *bool
	importNativeCA     bool

	cmd []string
}

func (i *initMachine) buildCmd(m *machineTestBuilder) []string {
	diskSize := defaultDiskSize
	cmd := []string{"machine", "init"}
	if i.cpus != nil {
		cmd = append(cmd, "--cpus", strconv.Itoa(int(*i.cpus)))
	}
	if i.diskSize != nil {
		diskSize = *i.diskSize
	}
	cmd = append(cmd, "--disk-size", strconv.Itoa(int(diskSize)))
	if l := len(i.ignitionPath); l > 0 {
		cmd = append(cmd, "--ignition-path", i.ignitionPath)
	}
	if l := len(i.username); l > 0 {
		cmd = append(cmd, "--username", i.username)
	}
	if l := len(i.image); l > 0 {
		cmd = append(cmd, "--image", i.image)
	}
	if i.memory != nil {
		cmd = append(cmd, "--memory", strconv.Itoa(int(*i.memory)))
	}
	if l := len(i.timezone); l > 0 {
		cmd = append(cmd, "--timezone", i.timezone)
	}
	for _, v := range i.volumes {
		cmd = append(cmd, "--volume", v)
	}
	if i.now {
		cmd = append(cmd, "--now")
	}
	if i.rootful {
		cmd = append(cmd, "--rootful")
	}
	if l := len(i.playbook); l > 0 {
		cmd = append(cmd, "--playbook", i.playbook)
	}
	if l := len(i.provider); l > 0 {
		cmd = append(cmd, "--provider", i.provider)
	}
	if i.userModeNetworking {
		cmd = append(cmd, "--user-mode-networking")
	}
	if i.swap != nil {
		cmd = append(cmd, "--swap", strconv.Itoa(int(*i.swap)))
	}
	if i.tlsVerify != nil {
		cmd = append(cmd, "--tls-verify="+strconv.FormatBool(*i.tlsVerify))
	}
	if i.updateConnection != nil {
		cmd = append(cmd, fmt.Sprintf("--update-connection=%s", strconv.FormatBool(*i.updateConnection)))
	}
	if i.importNativeCA {
		cmd = append(cmd, "--import-native-ca")
	}

	name := m.name
	cmd = append(cmd, name)

	// when we create a new VM remove it again as cleanup
	DeferCleanup(func() {
		r := new(rmMachine)
		session, err := m.setName(name).setCmd(r.withForce()).run()
		Expect(err).ToNot(HaveOccurred(), "error occurred rm'ing machine")
		// Some test create a invalid VM so the VM does not exists in this case we have to ignore the error.
		// It would be much better if rm -f would behave like other commands and ignore not exists errors.
		if session.ExitCode() == 125 {
			if strings.Contains(session.errorToString(), "VM does not exist") {
				return
			}

			// FIXME:#24344 work-around for custom ignition removal
			if strings.Contains(session.errorToString(), "failed to remove machines files: unable to find connection named") {
				return
			}
		}
		Expect(session).To(Exit(0))
	})

	i.cmd = cmd
	return cmd
}

func (i *initMachine) withCPUs(num uint) *initMachine {
	i.cpus = &num
	return i
}

func (i *initMachine) withDiskSize(size uint) *initMachine {
	i.diskSize = &size
	return i
}

func (i *initMachine) withSwap(size uint) *initMachine {
	i.swap = &size
	return i
}

func (i *initMachine) withIgnitionPath(path string) *initMachine {
	i.ignitionPath = path
	return i
}

func (i *initMachine) withUsername(username string) *initMachine {
	i.username = username
	return i
}

func (i *initMachine) withImage(path string) *initMachine {
	i.image = path
	return i
}

func (i *initMachine) withMemory(num uint) *initMachine {
	i.memory = &num
	return i
}

func (i *initMachine) withNow() *initMachine {
	i.now = true
	return i
}

func (i *initMachine) withTimezone(tz string) *initMachine {
	i.timezone = tz
	return i
}

func (i *initMachine) withVolume(v string) *initMachine {
	i.volumes = append(i.volumes, v)
	return i
}

func (i *initMachine) withRootful(r bool) *initMachine {
	i.rootful = r
	return i
}

func (i *initMachine) withRunPlaybook(p string) *initMachine {
	i.playbook = p
	return i
}

func (i *initMachine) withProvider(p string) *initMachine {
	i.provider = p
	return i
}

func (i *initMachine) withTlsVerify(tlsVerify *bool) *initMachine {
	i.tlsVerify = tlsVerify
	return i
}

func (i *initMachine) withUpdateConnection(value *bool) *initMachine {
	i.updateConnection = value
	return i
}

func (i *initMachine) withUserModeNetworking(r bool) *initMachine { //nolint:unused,nolintlint
	i.userModeNetworking = r
	return i
}

func (i *initMachine) withImportNativeCA(r bool) *initMachine {
	i.importNativeCA = r
	return i
}
