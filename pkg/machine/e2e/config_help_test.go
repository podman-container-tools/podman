package e2e_test

type helpMachine struct {
	cmd []string
}

func (i *helpMachine) buildCmd(_ *machineTestBuilder) []string {
	cmd := []string{"help"}
	i.cmd = cmd
	return cmd
}
