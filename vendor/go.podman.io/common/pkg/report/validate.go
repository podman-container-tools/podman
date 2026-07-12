package report

import (
	"go.podman.io/storage/pkg/regexp"
)

// Check for json, {{json }} and {{ json. }} which are not valid go template,
// {{json .}} is valid and thus not matched to let the template handle it like docker does.
var jsonRegex = regexp.Delayed(`^\s*(json|{{\s*json\.?\s*}})\s*$`)

// IsJSON check if CLI --format string is a JSON request
//
//	if report.IsJSON(cmd.Flag("format").Value.String()) {
//	  ... process JSON and output
//	}
func IsJSON(s string) bool {
	return jsonRegex.MatchString(s)
}
