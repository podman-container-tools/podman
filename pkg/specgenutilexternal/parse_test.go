package specgenutilexternal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindMountType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		mountType string
		tokens    []string
	}{
		{
			name:      "bind with options",
			input:     "type=bind,src=/foo,target=/bar",
			mountType: "bind",
			tokens:    []string{"src=/foo", "target=/bar"},
		},
		{
			name:      "tmpfs",
			input:     "type=tmpfs,target=/tmp",
			mountType: "tmpfs",
			tokens:    []string{"target=/tmp"},
		},
		{
			name:      "no type defaults to volume",
			input:     "src=/foo,target=/bar",
			mountType: "volume",
			tokens:    []string{"src=/foo", "target=/bar"},
		},
		{
			name:      "type does not need to be first",
			input:     "src=/foo,type=bind,target=/bar",
			mountType: "bind",
			tokens:    []string{"src=/foo", "target=/bar"},
		},
		{
			name:      "only the first type is used",
			input:     "type=bind,type=tmpfs",
			mountType: "bind",
			tokens:    []string{"type=tmpfs"},
		},
		{
			name:      "token with multiple equals is preserved",
			input:     "type=bind,opt=a=b",
			mountType: "bind",
			tokens:    []string{"opt=a=b"},
		},
		{
			name:      "empty type value",
			input:     "type=,target=/bar",
			mountType: "",
			tokens:    []string{"target=/bar"},
		},
		{
			name:      "whitespace around type is not recognized",
			input:     "type = a,target=/bar",
			mountType: "volume",
			tokens:    []string{"type = a", "target=/bar"},
		},
		{
			name:      "literal backslash-n is not a newline",
			input:     "type=bind\\nfoo=bar",
			mountType: "volume",
			tokens:    []string{"type=bind\\nfoo=bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mountType, tokens, err := FindMountType(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.mountType, mountType)
			assert.Equal(t, tt.tokens, tokens)
		})
	}
}

func TestFindMountTypeErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty input", input: ""},
		{name: "newline creates multiple records", input: "type=bind\nfoo=bar"},
		{name: "bare quote in value", input: `type=bind,src="/path,with,commas",target=/bar`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := FindMountType(tt.input)
			assert.Error(t, err)
		})
	}
}
