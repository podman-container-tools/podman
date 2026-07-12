// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"fmt"
	"runtime/debug"
)

var (
	// Version for the swagger command.
	Version string
	// Commit for the swagger command.
	Commit string
)

// PrintVersion the command.
type PrintVersion struct{}

// Execute this command.
//
//nolint:forbidigo // this commands is allowed to use fmt.Println
func (p *PrintVersion) Execute(_ []string) error {
	if Version == "" {
		if info, available := debug.ReadBuildInfo(); available && info.Main.Version != "(devel)" {
			// built from source, with module (e.g. go get)
			fmt.Println("version:", info.Main.Version)
			fmt.Println("commit:", fmt.Sprintf("(unknown, mod sum: %q)", info.Main.Sum))
			return nil
		}
		// built from source, local repo
		fmt.Println("dev")
		return nil
	}
	// released version
	fmt.Println("version:", Version)
	fmt.Println("commit:", Commit)

	return nil
}
