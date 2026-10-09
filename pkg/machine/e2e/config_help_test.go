package e2e_test

import "context"

type helpMachine struct {
	cmd []string
}

func (i *helpMachine) buildCmd(_ context.Context, _ *machineTestBuilder) []string {
	cmd := []string{"help"}
	i.cmd = cmd
	return cmd
}
