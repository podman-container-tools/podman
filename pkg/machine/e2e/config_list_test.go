package e2e_test

type listMachine struct {
	/*
		--format string   Format volume output using JSON or a Go template (default "{{.Name}}\t{{.VMType}}\t{{.Created}}\t{{.LastUp}}\t{{.CPUs}}\t{{.Memory}}\t{{.DiskSize}}\n")
		--noheading       Do not print headers
		-q, --quiet           Show only machine names
	*/

	format       string
	noHeading    bool
	quiet        bool
	allProviders bool

	cmd []string
}

func (i *listMachine) buildCmd(_ *machineTestBuilder) []string {
	cmd := []string{"machine", "list"}
	if len(i.format) > 0 {
		cmd = append(cmd, "--format", i.format)
	}
	if i.noHeading {
		cmd = append(cmd, "--noheading")
	}
	if i.quiet {
		cmd = append(cmd, "--quiet")
	}
	if i.allProviders {
		cmd = append(cmd, "--all-providers")
	}

	i.cmd = cmd
	return cmd
}

func (i *listMachine) withNoHeading() *listMachine {
	i.noHeading = true
	return i
}

func (i *listMachine) withQuiet() *listMachine {
	i.quiet = true
	return i
}

func (i *listMachine) withFormat(format string) *listMachine {
	i.format = format
	return i
}
