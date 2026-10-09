package e2e_test

import "context"

type fakeCompose struct {
	cmd []string
}

func (f *fakeCompose) buildCmd(_ context.Context, _ *machineTestBuilder) []string {
	cmd := []string{"compose"}
	cmd = append(cmd, "env")
	f.cmd = cmd
	return cmd
}
