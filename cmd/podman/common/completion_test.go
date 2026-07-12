package common_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/cmd/podman/common"
)

//nolint:recvcheck // We like to test mixed pointer receiver and non-pointer receiver
type Car struct {
	Brand string
	Stats struct {
		HP           *int
		Displacement int
	}
	Extras map[string]Extra
	// also ensure it will work with pointers
	Extras2 map[string]*Extra
}

type Extra struct {
	Name1 string
	Name2 string
}

type Anonymous struct {
	Hello string
	// The name should match the testStruct Name below. This is used to make
	// sure the logic uses the actual struct fields before the embedded ones.
	Name struct {
		Suffix string
		Prefix string
	}
}

// The name should match the testStruct Age name below.
func (a Anonymous) Age() int {
	return 0
}

func (c Car) Type() string {
	return ""
}

// Note: It is important that this function is *Car and the Type one is just Car.
// The reflect logic behaves differently for these cases so we have to test both.
func (c *Car) Color() string {
	return ""
}

// This is for reflect testing required.
//
//nolint:unused
func (c Car) internal() int {
	return 0
}

func (c Car) TwoOut() (string, string) {
	return "", ""
}

func (c Car) Struct() Car {
	return Car{}
}

func TestAutocompleteFormat(t *testing.T) {
	testStruct := struct {
		Name string
		Age  int
		Car  *Car
		Car2 *Car
		*Anonymous
		private int
	}{}

	testStruct.Car = &Car{}

	tests := []struct {
		name       string
		toComplete string
		expected   []string
	}{
		{
			"empty completion",
			"",
			[]string{"json"},
		},
		{
			"json completion",
			"json",
			[]string{"json"},
		},
		{
			"invalid completion",
			"blahblah",
			nil,
		},
		{
			"invalid completion",
			"{{",
			nil,
		},
		{
			"invalid completion",
			"{{  ",
			nil,
		},
		{
			"invalid completion",
			"{{  ..",
			[]string{},
		},
		{
			"fist level struct field name",
			"{{.",
			[]string{"{{.Name}}", "{{.Age}}", "{{.Car.", "{{.Car2.", "{{.Anonymous.", "{{.Hello}}"},
		},
		{
			"fist level struct field name",
			"{{ .",
			[]string{"{{ .Name}}", "{{ .Age}}", "{{ .Car.", "{{ .Car2.", "{{ .Anonymous.", "{{ .Hello}}"},
		},
		{
			"fist level struct field name",
			"{{ .N",
			[]string{"{{ .Name}}"},
		},
		{
			"second level struct field name",
			"{{ .Car.",
			[]string{"{{ .Car.Color}}", "{{ .Car.Struct.", "{{ .Car.Type}}", "{{ .Car.Brand}}", "{{ .Car.Stats.", "{{ .Car.Extras.", "{{ .Car.Extras2."},
		},
		{
			"second level struct field name",
			"{{ .Car.B",
			[]string{"{{ .Car.Brand}}"},
		},
		{
			"second level nil struct field name",
			"{{ .Car2.",
			[]string{"{{ .Car2.Color}}", "{{ .Car2.Struct.", "{{ .Car2.Type}}", "{{ .Car2.Brand}}", "{{ .Car2.Stats.", "{{ .Car2.Extras.", "{{ .Car2.Extras2."},
		},
		{
			"three level struct field name",
			"{{ .Car.Stats.",
			[]string{"{{ .Car.Stats.HP}}", "{{ .Car.Stats.Displacement}}"},
		},
		{
			"three level struct field name",
			"{{ .Car.Stats.D",
			[]string{"{{ .Car.Stats.Displacement}}"},
		},
		{
			"second level struct field name",
			"{{ .Car.B",
			[]string{"{{ .Car.Brand}}"},
		},
		{
			"invalid field name",
			"{{ .Ca.B",
			[]string{},
		},
		{
			"map key names don't work",
			"{{ .Car.Extras.",
			[]string{},
		},
		{
			"map values work",
			"{{ .Car.Extras.somekey.",
			[]string{"{{ .Car.Extras.somekey.Name1}}", "{{ .Car.Extras.somekey.Name2}}"},
		},
		{
			"map values work with ptr",
			"{{ .Car.Extras2.somekey.",
			[]string{"{{ .Car.Extras2.somekey.Name1}}", "{{ .Car.Extras2.somekey.Name2}}"},
		},
		{
			"two variables struct field name",
			"{{ .Car.Brand }} {{ .Car.",
			[]string{
				"{{ .Car.Brand }} {{ .Car.Color}}", "{{ .Car.Brand }} {{ .Car.Struct.", "{{ .Car.Brand }} {{ .Car.Type}}",
				"{{ .Car.Brand }} {{ .Car.Brand}}", "{{ .Car.Brand }} {{ .Car.Stats.", "{{ .Car.Brand }} {{ .Car.Extras.",
				"{{ .Car.Brand }} {{ .Car.Extras2.",
			},
		},
		{
			"only dot without variable",
			".",
			nil,
		},
		{
			"access embedded nil struct field",
			"{{.Hello.",
			[]string{},
		},
	}

	for _, test := range tests {
		completion, directive := common.AutocompleteFormat(&testStruct)(nil, nil, test.toComplete)
		// directive should always be greater than ShellCompDirectiveNoFileComp
		assert.GreaterOrEqual(t, directive, cobra.ShellCompDirectiveNoFileComp, "unexpected ShellCompDirective")
		assert.Equal(t, test.expected, completion, test.name)
	}
}
