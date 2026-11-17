package common

import (
	"os"

	"golang.org/x/term"
)

// ClearScreen clears the screen and puts the cursor back to position 1,1
// Useful when printing output in an interval like podman stats.
// When the stdout is not a terminal this is a NOP.
func ClearScreen() {
	// Only write escape sequences when the output is a terminal.
	if term.IsTerminal(int(os.Stdout.Fd())) {
		// terminal escape control sequence to clear screen ([2J)
		// followed by putting the cursor to position 1,1 ([1;1H)
		os.Stdout.WriteString("\033[2J\033[1;1H")
	}
}
