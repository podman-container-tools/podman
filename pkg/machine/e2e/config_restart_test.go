package e2e_test

type restartMachine struct{}

func (r restartMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "restart"}
	if len(m.name) > 0 {
		cmd = append(cmd, m.name)
	}
	return cmd
}
