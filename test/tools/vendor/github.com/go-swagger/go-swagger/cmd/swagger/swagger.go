// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"log"
	"os"

	flags "github.com/jessevdk/go-flags"

	"github.com/go-swagger/go-swagger/cmd/swagger/commands"
)

var opts struct {
	// General options applicable to all commands
	Quiet   func()       `description:"silence logs"          long:"quiet"      short:"q"`
	LogFile func(string) `description:"redirect logs to file" long:"log-output" value-name:"LOG-FILE"`
	// Version bool `long:"version" short:"v" description:"print the version of the command"`
}

func main() {
	parser := flags.NewParser(&opts, flags.Default)
	parser.ShortDescription = "helps you keep your API well described"
	parser.LongDescription = `
Swagger tries to support you as best as possible when building APIs.

It aims to represent the contract of your API with a language agnostic description of your application in json or yaml.
`
	_, err := parser.AddCommand("validate", "validate the swagger document", "validate the provided swagger document against a swagger spec", &commands.ValidateSpec{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("init", "initialize a spec document", "initialize a swagger spec document", &commands.InitCmd{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("version", "print the version", "print the version of the swagger command", &commands.PrintVersion{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("serve", "serve spec and docs", "serve a spec and swagger or redoc documentation ui", &commands.ServeCmd{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("expand", "expand $ref fields in a swagger spec", "expands the $refs in a swagger document to inline schemas", &commands.ExpandSpec{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("flatten", "flattens a swagger document", "expand the remote references in a spec and move inline schemas to definitions, after flattening there are no complex inlined anymore", &commands.FlattenSpec{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("mixin", "merge swagger documents", "merge additional specs into first/primary spec by copying their paths and definitions", &commands.MixinSpec{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("diff", "diff swagger documents", "diff specs showing which changes will break existing clients", &commands.DiffCommand{})
	if err != nil {
		log.Fatal(err)
	}

	genpar, err := parser.AddCommand("generate", "generate go code", "generate go code for the swagger spec file", &commands.Generate{})
	if err != nil {
		log.Fatalln(err)
	}
	for _, cmd := range genpar.Commands() {
		switch cmd.Name {
		case "spec":
			cmd.ShortDescription = "generate a swagger spec document from a go application"
			cmd.LongDescription = cmd.ShortDescription
		case "client":
			cmd.ShortDescription = "generate all the files for a client library"
			cmd.LongDescription = cmd.ShortDescription
		case "server":
			cmd.ShortDescription = "generate all the files for a server application"
			cmd.LongDescription = cmd.ShortDescription
		case "model":
			cmd.ShortDescription = "generate one or more models from the swagger spec"
			cmd.LongDescription = cmd.ShortDescription
		case "support":
			cmd.ShortDescription = "generate supporting files like the main function and the api builder"
			cmd.LongDescription = cmd.ShortDescription
		case "operation":
			cmd.ShortDescription = "generate one or more server operations from the swagger spec"
			cmd.LongDescription = cmd.ShortDescription
		case "markdown":
			cmd.ShortDescription = "generate a markdown representation from the swagger spec"
			cmd.LongDescription = cmd.ShortDescription
		case "cli":
			cmd.ShortDescription = "generate a command line client tool from the swagger spec"
			cmd.LongDescription = cmd.ShortDescription
		}
	}

	opts.Quiet = func() {
		log.SetOutput(io.Discard)
	}

	opts.LogFile = func(logfile string) {
		f, err := os.Create(logfile)
		if err != nil {
			log.Fatalf("cannot write to file %s: %v", logfile, err)
		}
		log.SetOutput(f)
	}

	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}
