package validate

import (
	"fmt"
	"slices"
	"strings"
)

// Honors cobra.Value interface
type ChoiceValue struct {
	value   *string
	choices []string
}

// Value may be used in cobra FlagSet methods Var/VarP/VarPF() to select from a set of values
//
// Example:
//
//	created := validate.ChoiceValue(&opts.Sort, "command", "created", "id", "image", "names", "runningfor", "size", "status")
//	flags.Var(created, "sort", "Sort output by: "+created.Choices())
func Value(p *string, choices ...string) *ChoiceValue {
	return &ChoiceValue{
		value:   p,
		choices: choices,
	}
}

func (c *ChoiceValue) String() string {
	return *c.value
}

func (c *ChoiceValue) Set(value string) error {
	if slices.Contains(c.choices, value) {
		*c.value = value
		return nil
	}
	return fmt.Errorf("%q is not a valid value.  Choose from: %q", value, c.Choices())
}

func (c *ChoiceValue) Choices() string {
	return strings.Join(c.choices, ", ")
}

func (c *ChoiceValue) Type() string {
	return "choice"
}
