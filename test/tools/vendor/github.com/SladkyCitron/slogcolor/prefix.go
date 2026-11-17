package slogcolor

import (
	"strings"

	"github.com/fatih/color"
)

// Prefix prepends a colored prefix to msg.
func Prefix(prefix string, msg ...string) string {
	if len(msg) == 0 {
		return color.New(color.BgHiWhite, color.FgBlack).Sprint(prefix)
	}

	return color.New(color.BgHiWhite, color.FgBlack).Sprint(prefix) + " " + strings.Join(msg, " ")
}
