package e2e_test

import "context"

type restartMachine struct{}

func (r restartMachine) buildCmd(_ context.Context, m *machineTestBuilder) []string {
	cmd := []string{"machine", "restart"}
	if len(m.name) > 0 {
		cmd = append(cmd, m.name)
	}
	return cmd
}
