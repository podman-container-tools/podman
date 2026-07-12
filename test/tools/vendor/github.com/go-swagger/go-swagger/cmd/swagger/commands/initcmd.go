// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package commands

import "github.com/go-swagger/go-swagger/cmd/swagger/commands/initcmd"

// InitCmd is a command namespace for initializing things like a swagger spec.
type InitCmd struct {
	Model *initcmd.Spec `command:"spec"`
}

// Execute provides default empty implementation.
func (i *InitCmd) Execute(_ []string) error {
	return nil
}
