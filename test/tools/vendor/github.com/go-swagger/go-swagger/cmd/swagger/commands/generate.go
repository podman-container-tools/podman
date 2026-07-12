// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package commands

import "github.com/go-swagger/go-swagger/cmd/swagger/commands/generate"

// Generate command to group all generator commands together.
type Generate struct {
	Model     *generate.Model     `command:"model"`
	Operation *generate.Operation `command:"operation"`
	Support   *generate.Support   `command:"support"`
	Server    *generate.Server    `command:"server"`
	Spec      *generate.SpecFile  `command:"spec"`
	Client    *generate.Client    `command:"client"`
	Cli       *generate.Cli       `command:"cli"`
	Markdown  *generate.Markdown  `command:"markdown"`
}
