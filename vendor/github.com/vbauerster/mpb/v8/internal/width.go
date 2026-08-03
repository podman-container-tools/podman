package internal

import "cmp"

// CheckRequestedWidth checks that requested width doesn't overflow available width
func CheckRequestedWidth(requested, available int) int {
	return min(cmp.Or(requested, available), available)
}
