package e2e_test

type applyMachineOS struct {
	imageName string
	restart   bool

	cmd []string
}

func (a *applyMachineOS) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "os", "apply"}
	if a.restart {
		cmd = append(cmd, "--restart")
	}
	cmd = append(cmd, a.imageName, m.name)
	a.cmd = cmd
	return cmd
}

func (a *applyMachineOS) withRestart() *applyMachineOS {
	a.restart = true
	return a
}

func (a *applyMachineOS) withImage(i string) *applyMachineOS {
	a.imageName = i
	return a
}
