package e2e_test

import (
	"fmt"
	"strconv"
)

type startMachine struct {
	/*
		No command line args other than a machine vm name (also not required)
	*/
	quiet            bool
	noInfo           bool
	updateConnection *bool
}

func (s *startMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "start"}
	if len(m.name) > 0 {
		cmd = append(cmd, m.name)
	}
	if s.quiet {
		cmd = append(cmd, "--quiet")
	}
	if s.noInfo {
		cmd = append(cmd, "--no-info")
	}
	if s.updateConnection != nil {
		cmd = append(cmd, fmt.Sprintf("--update-connection=%s", strconv.FormatBool(*s.updateConnection)))
	}
	return cmd
}

func (s *startMachine) withQuiet() *startMachine {
	s.quiet = true
	return s
}

func (s *startMachine) withNoInfo() *startMachine {
	s.noInfo = true
	return s
}

func (s *startMachine) withUpdateConnection(value *bool) *startMachine {
	s.updateConnection = value
	return s
}

func ptrBool(v bool) *bool {
	return &v
}
