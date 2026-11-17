/*
	The purpose of this test is to keep a consistent
	and great shell autocompletion experience.

	This test ensures that each command and flag has a shell completion
	function set. (except boolean, hidden and deprecated flags)

	Shell completion functions are defined in:
	- "go.podman.io/podman/v6/cmd/podman/common/completion.go"
	- "go.podman.io/common/pkg/completion"
	and are called Autocomplete...

	To apply such function to a command use the ValidArgsFunction field.
	To apply such function to a flag use cmd.RegisterFlagCompletionFunc(name,func)

	If there are any questions/problems please tag Luap99.
*/

package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestShellCompletionFunctions(t *testing.T) {
	rootCmd := parseCommands()
	checkCommand(t, rootCmd)
}

func checkCommand(t *testing.T, cmd *cobra.Command) {
	if cmd.HasSubCommands() {
		for _, childCmd := range cmd.Commands() {
			if !childCmd.Hidden {
				checkCommand(t, childCmd)
			}
		}

		// if not check if completion for that command is provided
	} else if cmd.ValidArgsFunction == nil && cmd.ValidArgs == nil {
		t.Errorf("%s command has no shell completion function set", cmd.CommandPath())
	}

	// Verify Example strings are flush-left (no leading whitespace).
	// The indentExamples template function adds the 2-space indent at
	// render time, so source examples must not contain their own indentation.
	for i, line := range strings.Split(cmd.Example, "\n") {
		if line != "" && line != strings.TrimLeft(line, " \t") {
			t.Errorf("%s: Example line %d has leading whitespace: %q",
				cmd.CommandPath(), i+1, line)
		}
	}

	// loop over all local flags
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		// an error means that there is a completion function for this flag
		err := cmd.RegisterFlagCompletionFunc(flag.Name, func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveDefault
		})

		switch {
		case flag.Value.Type() == "bool" && err != nil:
			// make sure bool flags don't have a completion function
			t.Errorf(`%s --%s is a bool flag but has a shell completion function set.
You have to remove this shell completion function.`, cmd.CommandPath(), flag.Name)
			return

		case flag.Value.Type() == "bool" || flag.Hidden || len(flag.Deprecated) > 0:
			// skip bool, hidden and deprecated flags
			return

		case err == nil:
			// there is no shell completion function
			t.Errorf("%s --%s flag has no shell completion function set", cmd.CommandPath(), flag.Name)
		}
	})
}
