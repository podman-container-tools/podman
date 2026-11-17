package completion

import (
	"strings"

	"github.com/spf13/cobra"
)

/* Autocomplete Functions for cobra ValidArgsFunction */

// AutocompleteNamespaceFlag - Autocomplete the userns flag.
// -> host, private, container, ns:[path], [path]
func AutocompleteNamespaceFlag(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var completions []string
	// If we don't filter on "toComplete", zsh and fish will not do file completion
	// even if the prefix typed by the user does not match the returned completions
	for _, comp := range []string{"host", "private", "container", "ns:"} {
		if strings.HasPrefix(comp, toComplete) {
			completions = append(completions, comp)
		}
	}
	return completions, cobra.ShellCompDirectiveDefault
}
