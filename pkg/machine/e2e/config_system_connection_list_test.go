package e2e_test

type listSystemConnection struct {
	/*
		--format string   Custom Go template for printing connections
	*/

	format string
}

func (l *listSystemConnection) buildCmd(_ *machineTestBuilder) []string {
	cmd := []string{"system", "connection", "list"}
	if len(l.format) > 0 {
		cmd = append(cmd, "--format", l.format)
	}

	return cmd
}

func (l *listSystemConnection) withFormat(format string) *listSystemConnection {
	l.format = format
	return l
}
