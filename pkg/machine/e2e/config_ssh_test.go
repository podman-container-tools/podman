package e2e_test

type sshMachine struct {
	/*
		--username string   Username to use when ssh-ing into the VM.
	*/

	username   string //nolint:unused
	sshCommand []string
}

func (s *sshMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "ssh"}
	if len(m.name) > 0 {
		cmd = append(cmd, m.name)
	}
	if len(s.sshCommand) > 0 {
		cmd = append(cmd, s.sshCommand...)
	}
	return cmd
}

func (s *sshMachine) withUsername(name string) *sshMachine { //nolint:unused
	s.username = name
	return s
}

func (s *sshMachine) withSSHCommand(sshCommand []string) *sshMachine {
	s.sshCommand = sshCommand
	return s
}
