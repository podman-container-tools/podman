//go:build linux

package main

import (
	"testing"

	"github.com/rootless-containers/rootlesskit/v2/pkg/port"
	"github.com/stretchr/testify/assert"
	"go.podman.io/common/pkg/machine"
)

type SpecData struct {
	mach        string
	sourceProto string
	sourceIP    string
	expectCount int
	expectProto string
	expectIP    string
	secondProto string
	secondIP    string
}

func TestDualStackSplit(t *testing.T) {
	const (
		IP4_ALL = "0.0.0.0"
		IP4__LO = "127.0.0.1"
		IP6_ALL = "::"
		IP6__LO = "::1"
		TCP_    = "tcp"
		TCP4    = "tcp4"
		TCP6    = "tcp6"
		WSL     = "wsl"
		___     = ""
		IP6_REG = "2001:0db8:85a3:0000:0000:8a2e:0370:7334"
		IP4_REG = "10.0.0.1"
	)

	tests := []SpecData{
		// Split cases
		{WSL, TCP_, IP4_ALL, 2, TCP4, IP4_ALL, TCP6, IP6_ALL},
		{WSL, TCP_, IP6_ALL, 2, TCP4, IP4_ALL, TCP6, IP6_ALL},
		{WSL, TCP_, IP6__LO, 2, TCP4, IP4__LO, TCP6, IP6__LO},

		// Non-Split
		{WSL, TCP_, IP4__LO, 1, TCP_, IP4__LO, "", ""},
		{WSL, TCP4, IP4_ALL, 1, TCP4, IP4_ALL, "", ""},
		{WSL, TCP6, IP6__LO, 1, TCP6, IP6__LO, "", ""},
		{WSL, TCP_, IP4_REG, 1, TCP_, IP4_REG, "", ""},
		{WSL, TCP_, IP6_REG, 1, TCP_, IP6_REG, "", ""},
		{___, TCP_, IP4_ALL, 1, TCP_, IP4_ALL, "", ""},
		{___, TCP_, IP6_ALL, 1, TCP_, IP6_ALL, "", ""},
		{___, TCP_, IP4__LO, 1, TCP_, IP4__LO, "", ""},
		{___, TCP_, IP6__LO, 1, TCP_, IP6__LO, "", ""},
	}

	for _, data := range tests {
		verifySplit(t, data)
	}
}

func verifySplit(t *testing.T, data SpecData) {
	machine := machine.GetMachineMarker()
	oldEnable, oldType := machine.Enabled, machine.Type
	machine.Enabled, machine.Type = len(data.mach) > 0, data.mach

	source := port.Spec{
		Proto:      data.sourceProto,
		ParentIP:   data.sourceIP,
		ParentPort: 100,
		ChildIP:    "1.1.1.1",
		ChildPort:  200,
	}
	expect, second := source, source
	specs := splitDualStackSpecIfWsl(source)

	assert.Equal(t, data.expectCount, len(specs))

	expect.Proto = data.expectProto
	expect.ParentIP = data.expectIP
	assert.Equal(t, expect, specs[0])

	if data.expectCount > 1 {
		second.Proto = data.secondProto
		second.ParentIP = data.secondIP
		assert.Equal(t, second, specs[1])
	}

	machine.Enabled, machine.Type = oldEnable, oldType
}
