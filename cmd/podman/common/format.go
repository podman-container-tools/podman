package common

import (
	"slices"
	"strings"
)

// FormatLabels converts a map of labels to a sorted, comma-separated list
// of key=value pairs, matching Docker CLI output format.
func FormatLabels(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	list := make([]string, 0, len(keys))
	for _, k := range keys {
		list = append(list, k+"="+labels[k])
	}
	return strings.Join(list, ",")
}
