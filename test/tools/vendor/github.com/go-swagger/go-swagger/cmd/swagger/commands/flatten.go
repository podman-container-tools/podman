// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"errors"

	flags "github.com/jessevdk/go-flags"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"

	"github.com/go-swagger/go-swagger/cmd/swagger/commands/generate"
)

// FlattenSpec is a command that flattens a swagger document
// which will expand the remote references in a spec and move inline schemas to definitions
// after flattening there are no complex inlined anymore.
type FlattenSpec struct {
	generate.FlattenCmdOptions

	Compact bool           `description:"applies to JSON formatted specs. When present, doesn't prettify the json" long:"compact"`
	Output  flags.Filename `description:"the file to write to"                                                     long:"output"  short:"o"`
	Format  string         `choice:"yaml"                                                                          choice:"json"  default:"json" description:"the format for the spec document" long:"format"`
}

// Execute flattens the spec.
func (c *FlattenSpec) Execute(args []string) error {
	if len(args) != 1 {
		return errors.New("flatten command requires the single swagger document url to be specified")
	}

	swaggerDoc := args[0]
	specDoc, err := loads.Spec(swaggerDoc)
	if err != nil {
		return err
	}

	flattenOpts := c.SetFlattenOptions(&analysis.FlattenOpts{
		// defaults
		Minimal:      true,
		Verbose:      true,
		Expand:       false,
		RemoveUnused: false,
	})
	flattenOpts.BasePath = specDoc.SpecFilePath()
	flattenOpts.Spec = analysis.New(specDoc.Spec())
	if err := analysis.Flatten(*flattenOpts); err != nil {
		return err
	}

	return writeToFile(specDoc.Spec(), !c.Compact, c.Format, string(c.Output))
}
