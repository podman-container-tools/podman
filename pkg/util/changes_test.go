package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeChanges(t *testing.T) {
	testCases := []struct {
		description string
		input       string
		output      []string
	}{
		{
			description: "nothing",
			input:       "",
			output:      []string{},
		},
		{
			description: "space",
			input:       `CMD=/bin/bash`,
			output:      []string{"CMD /bin/bash"},
		},
		{
			description: "equal",
			input:       `CMD=/bin/bash`,
			output:      []string{"CMD /bin/bash"},
		},
		{
			description: "both-but-right-first",
			input:       `LABEL A=B`,
			output:      []string{"LABEL A=B"},
		},
		{
			description: "both-but-right-second",
			input:       `LABEL A=B C=D`,
			output:      []string{"LABEL A=B C=D"},
		},
		{
			description: "both-but-wrong",
			input:       `LABEL=A=B C=D`,
			output:      []string{"LABEL A=B C=D"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			output := DecodeChanges([]string{testCase.input})
			assert.Equalf(t, testCase.output, output, "decoded value was not what we expected")
		})
	}
}
