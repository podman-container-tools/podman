package e2e_test

type inspectMachine struct {
	/*
		--format string   Format volume output using JSON or a Go template
	*/
	cmd    []string
	format string
}

func (i *inspectMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "inspect"}
	if len(i.format) > 0 {
		cmd = append(cmd, "--format", i.format)
	}
	cmd = append(cmd, m.name)
	i.cmd = cmd
	return cmd
}

func (i *inspectMachine) withFormat(format string) *inspectMachine {
	i.format = format
	return i
}
