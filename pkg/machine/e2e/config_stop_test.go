package e2e_test

type stopMachine struct {
	/*
		No command line args other than a machine vm name (also not required)
	*/
}

func (s stopMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "stop"}
	if len(m.name) > 0 {
		cmd = append(cmd, m.name)
	}
	return cmd
}
