// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"

	flags "github.com/jessevdk/go-flags"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag/yamlutils"
)

const readableMode fs.FileMode = 0o644 & fs.ModePerm

// ExpandSpec is a command that expands the $refs in a swagger document.
//
// There are no specific options for this expansion.
type ExpandSpec struct {
	Compact bool           `description:"applies to JSON formatted specs. When present, doesn't prettify the json" long:"compact"`
	Output  flags.Filename `description:"the file to write to"                                                     long:"output"  short:"o"`
	Format  string         `choice:"yaml"                                                                          choice:"json"  default:"json" description:"the format for the spec document" long:"format"`
}

// Execute expands the spec.
func (c *ExpandSpec) Execute(args []string) error {
	if len(args) != 1 {
		return errors.New("expand command requires the single swagger document url to be specified")
	}

	swaggerDoc := args[0]
	specDoc, err := loads.Spec(swaggerDoc)
	if err != nil {
		return err
	}

	exp, err := specDoc.Expanded()
	if err != nil {
		return err
	}

	return writeToFile(exp.Spec(), !c.Compact, c.Format, string(c.Output))
}

var defaultWriter io.Writer = os.Stdout

func writeToFile(swspec *spec.Swagger, pretty bool, format string, output string) error {
	var (
		b   []byte
		err error
	)
	asJSON := format == "json"
	log.Println("format = ", format)

	switch {
	case pretty && asJSON:
		b, err = json.MarshalIndent(swspec, "", "  ")
	case asJSON:
		b, err = json.Marshal(swspec)
	default:
		b, err = marshalAsYAML(swspec)
	}

	if err != nil {
		return err
	}

	switch output {
	case "", "-":
		_, e := fmt.Fprintf(defaultWriter, "%s\n", b)
		return e
	default:
		return os.WriteFile(output, b, readableMode)
	}
}

func marshalAsYAML(swspec *spec.Swagger) ([]byte, error) {
	b, err := json.Marshal(swspec)
	if err != nil {
		return nil, err
	}

	var data yamlutils.YAMLMapSlice
	err = json.Unmarshal(b, &data)
	if err != nil {
		return nil, err
	}

	var bb any
	bb, err = data.MarshalYAML()
	if err != nil {
		return nil, err
	}

	var ok bool
	b, ok = bb.([]byte)
	if !ok {
		return nil, fmt.Errorf("expected MarshalYAML to return bytes, but got: %T", bb)
	}

	return b, err
}
