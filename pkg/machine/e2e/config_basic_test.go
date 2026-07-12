package e2e_test

type basicMachine struct {
	args []string
}

func (s *basicMachine) buildCmd(_ *machineTestBuilder) []string {
	cmd := []string{"-r"}
	if len(s.args) > 0 {
		cmd = append(cmd, s.args...)
	}
	return cmd
}

func (s *basicMachine) withPodmanCommand(args []string) *basicMachine {
	s.args = args
	return s
}
