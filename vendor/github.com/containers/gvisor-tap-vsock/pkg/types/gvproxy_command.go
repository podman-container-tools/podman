package types

import (
	"os/exec"
	"strconv"
)

type GvproxyCommand struct {
	// Print packets on stderr
	Debug bool

	// Length of packet
	// Larger packets means less packets to exchange for the same amount of data (and less protocol overhead)
	MTU int

	// Values passed in by forward-xxx flags in commandline (forward-xxx:info)
	forwardInfo map[string][]string

	// List of endpoints the user wants to listen to
	endpoints []string

	// List of service endpoints the user wants to listen to
	servicesEndpoints []string

	// Map of different sockets provided by user (socket-type flag:socket)
	sockets map[string]string

	// Logfile where gvproxy should redirect logs
	LogFile string

	// File where gvproxy's pid is stored
	PidFile string

	// SSHPort to access the guest VM
	SSHPort int
}

func NewGvproxyCommand() GvproxyCommand {
	return GvproxyCommand{
		MTU:         1500,
		SSHPort:     2222,
		endpoints:   []string{},
		forwardInfo: map[string][]string{},
		sockets:     map[string]string{},
	}
}

func (c *GvproxyCommand) checkSocketsInitialized() {
	if len(c.sockets) < 1 {
		c.sockets = map[string]string{}
	}
}

func (c *GvproxyCommand) checkForwardInfoInitialized() {
	if len(c.forwardInfo) < 1 {
		c.forwardInfo = map[string][]string{}
	}
}

func (c *GvproxyCommand) AddEndpoint(endpoint string) {
	if len(c.endpoints) < 1 {
		c.endpoints = []string{}
	}

	c.endpoints = append(c.endpoints, endpoint)
}

func (c *GvproxyCommand) AddServiceEndpoint(endpoint string) {
	if len(c.servicesEndpoints) < 1 {
		c.servicesEndpoints = []string{}
	}

	c.servicesEndpoints = append(c.servicesEndpoints, endpoint)
}

func (c *GvproxyCommand) AddVpnkitSocket(socket string) {
	c.checkSocketsInitialized()
	c.sockets["listen-vpnkit"] = socket
}

func (c *GvproxyCommand) AddQemuSocket(socket string) {
	c.checkSocketsInitialized()
	c.sockets["listen-qemu"] = socket
}

func (c *GvproxyCommand) AddBessSocket(socket string) {
	c.checkSocketsInitialized()
	c.sockets["listen-bess"] = socket
}

func (c *GvproxyCommand) AddStdioSocket(socket string) {
	c.checkSocketsInitialized()
	c.sockets["listen-stdio"] = socket
}

func (c *GvproxyCommand) AddVfkitSocket(socket string) {
	c.checkSocketsInitialized()
	c.sockets["listen-vfkit"] = socket
}

func (c *GvproxyCommand) addForwardInfo(flag, value string) {
	c.forwardInfo[flag] = append(c.forwardInfo[flag], value)
}

func (c *GvproxyCommand) AddForwardSock(socket string) {
	c.checkForwardInfoInitialized()
	c.addForwardInfo("forward-sock", socket)
}

func (c *GvproxyCommand) AddForwardDest(dest string) {
	c.checkForwardInfoInitialized()
	c.addForwardInfo("forward-dest", dest)
}

func (c *GvproxyCommand) AddForwardUser(user string) {
	c.checkForwardInfoInitialized()
	c.addForwardInfo("forward-user", user)
}

func (c *GvproxyCommand) AddForwardIdentity(identity string) {
	c.checkForwardInfoInitialized()
	c.addForwardInfo("forward-identity", identity)
}

// socketsToCmdline converts Command.sockets to a commandline format
func (c *GvproxyCommand) socketsToCmdline() []string {
	args := []string{}

	for socketFlag, socket := range c.sockets {
		if socket != "" {
			args = append(args, "-"+socketFlag, socket)
		}
	}

	return args
}

// forwardInfoToCmdline converts Command.forwardInfo to a commandline format
func (c *GvproxyCommand) forwardInfoToCmdline() []string {
	args := []string{}

	for forwardInfoFlag, forwardInfo := range c.forwardInfo {
		for _, i := range forwardInfo {
			if i != "" {
				args = append(args, "-"+forwardInfoFlag, i)
			}
		}
	}

	return args
}

// endpointsToCmdline converts Command.endpoints to a commandline format
func (c *GvproxyCommand) endpointsToCmdline() []string {
	args := []string{}

	for _, endpoint := range c.endpoints {
		if endpoint != "" {
			args = append(args, "-listen", endpoint)
		}
	}

	return args
}

func (c *GvproxyCommand) servicesEndpointsToCmdline() []string {
	args := []string{}

	for _, endpoint := range c.servicesEndpoints {
		if endpoint != "" {
			args = append(args, "-services", endpoint)
		}
	}

	return args
}

// ToCmdline converts Command to a properly formatted command for gvproxy based
// on its fields
func (c *GvproxyCommand) ToCmdline() []string {
	args := []string{}

	// listen (endpoints)
	args = append(args, c.endpointsToCmdline()...)

	args = append(args, c.servicesEndpointsToCmdline()...)

	// debug
	if c.Debug {
		args = append(args, "-debug")
	}

	// mtu
	args = append(args, "-mtu", strconv.Itoa(c.MTU))

	// ssh-port
	args = append(args, "-ssh-port", strconv.Itoa(c.SSHPort))

	// sockets
	args = append(args, c.socketsToCmdline()...)

	// forward info
	args = append(args, c.forwardInfoToCmdline()...)

	// pid-file
	if c.PidFile != "" {
		args = append(args, "-pid-file", c.PidFile)
	}

	// log-file
	if c.LogFile != "" {
		args = append(args, "-log-file", c.LogFile)
	}

	return args
}

// Cmd converts Command to a commandline format and returns an exec.Cmd which
// can be executed by os/exec
func (c *GvproxyCommand) Cmd(gvproxyPath string) *exec.Cmd {
	return exec.Command(gvproxyPath, c.ToCmdline()...) // #nosec G204
}
