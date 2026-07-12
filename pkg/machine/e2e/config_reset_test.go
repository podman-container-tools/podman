package e2e_test

type resetMachine struct {
	/*
	  -f, --force           Stop and do not prompt before resetting
	*/

	force bool

	cmd []string
}

func (i *resetMachine) buildCmd(_ *machineTestBuilder) []string {
	cmd := []string{"machine", "reset"}
	if i.force {
		cmd = append(cmd, "--force")
	}
	i.cmd = cmd
	return cmd
}

func (i *resetMachine) withForce() *resetMachine {
	i.force = true
	return i
}
