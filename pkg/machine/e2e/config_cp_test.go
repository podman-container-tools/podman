package e2e_test

type cpMachine struct {
	quiet bool
	src   string
	dest  string

	cmd []string
}

func (c *cpMachine) buildCmd(_ *machineTestBuilder) []string {
	cmd := []string{"machine", "cp"}

	if c.quiet {
		cmd = append(cmd, "--quiet")
	}

	cmd = append(cmd, c.src, c.dest)

	c.cmd = cmd
	return cmd
}

func (c *cpMachine) withQuiet() *cpMachine {
	c.quiet = true
	return c
}

func (c *cpMachine) withSrc(src string) *cpMachine {
	c.src = src
	return c
}

func (c *cpMachine) withDest(dest string) *cpMachine {
	c.dest = dest
	return c
}
