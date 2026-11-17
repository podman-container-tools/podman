// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Command swagger is a CLI tool to work with Swagger specifications.
package main

import (
	"cmp"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strings"

	flags "github.com/jessevdk/go-flags"
)

type docCommand struct {
	Destination string `default:"./docs"                                                description:"Output destination folder"                        long:"dest"  short:"d"`
	Width       uint16 `default:"132"                                                   description:"Desired width in columns of the formatted output" long:"width" short:"w"`
	Include     bool   `description:"Adds a hugo shortcode include line after the help" long:"include-shortcode"                                       short:"i"`
	parser      *flags.Parser
}

func (d *docCommand) Execute(_ []string) error {
	return d.gendoc()
}

func (d *docCommand) gendoc() (err error) {
	if d.Destination == "" {
		const defaultFolder = "docs"
		d.Destination = defaultFolder
	}

	const writableMode = 0o755
	if err = os.MkdirAll(d.Destination, writableMode); err != nil {
		return err
	}

	for documented := range documentCLI(d.parser) {
		if err = d.documentCommand(documented); err != nil {
			return err
		}
	}

	return nil
}

func (d *docCommand) documentCommand(documented doc) (err error) {
	file, err := os.Create(filepath.Join(d.Destination, documented.Target))
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	stdout := os.Stdout
	stderr := os.Stderr

	// front matter
	fmt.Fprintln(file, "---")
	fmt.Fprintf(file, "title: %q\n", documented.Title)
	fmt.Fprintf(file, "description: %q\n", documented.Description)
	fmt.Fprintf(file, "weight: %d\n", documented.Index)
	fmt.Fprintln(file, "---")
	fmt.Fprintln(file, "")

	// markdown formatting
	fmt.Fprintf(file, "## %s\n", documented.Title)
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "```cmd")

	// go-flags formats a help message by polling the width of the stdin terminal: we give it a pseudo-tty, sized at 132 cols.
	const (
		defaultTermWidth = 132
		builtinTermWidth = 80
	)
	if d.Width < builtinTermWidth {
		d.Width = defaultTermWidth
	}
	restore, err := setTermsize(d.Width)
	if err != nil {
		return err
	}
	defer restore()

	// go-flags writes help messages to stdout: redirecting here
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
	}()

	os.Stdout = file
	os.Stderr = file

	// run the command with --help flag; swallow the error systematically returned by go-flag on --help
	_ = run(d.parser, documented.Args)

	fmt.Fprintln(file, "```")
	if d.Include {
		fmt.Fprintln(file, "")
		stem := fileStem(documented.Target)
		fmt.Fprintf(file, `{{%% include file="%s_include.md" %%}}`, stem)
		fmt.Fprintf(file, "\n")
	}

	return nil
}

func fileStem(name string) string {
	ext := filepath.Ext(name)

	return strings.TrimSuffix(name, ext)
}

type doc struct {
	Title       string
	Description string
	Target      string
	Args        []string
	Index       int // the index in the tree of commands: used to weight the produced pages
}

// documentCLI yields an iterator over the tree of commands and their subcommands.
func documentCLI(parser *flags.Parser) iter.Seq[doc] {
	return func(yield func(doc) bool) {
		i := 1
		rootDoc := doc{
			Title:       "All commands",
			Description: "All swagger commands",
			Index:       i,
			Target:      "commands.md",
			Args:        []string{"--help"},
		}
		if !yield(rootDoc) {
			return
		}

		i++
		for cmdDoc := range documentCommands([]string{}, &i, parser.Commands()) {
			if !yield(cmdDoc) {
				return
			}
		}
	}
}

func documentCommands(parents []string, index *int, commands []*flags.Command) iter.Seq[doc] {
	var prefix, parent string
	if len(parents) > 0 {
		prefix = strings.Join(parents, "_") + "_"
		parent = strings.Join(parents, " ") + " "
	}
	slices.SortStableFunc(commands, func(a, b *flags.Command) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return func(yield func(doc) bool) {
		for _, cmd := range commands {
			const addedArgs = 2
			args := make([]string, 0, len(parents)+addedArgs)
			args = append(args, parents...)
			args = append(args, cmd.Name, "--help")

			cmdDoc := doc{
				Title:       parent + cmd.Name,
				Description: cmd.ShortDescription,
				Target:      prefix + cmd.Name + ".md",
				Args:        args,
				Index:       *index,
			}
			if !yield(cmdDoc) {
				return
			}
			*index++

			children := slices.Clone(parents)
			children = slices.Grow(children, 1)
			children = append(children, cmd.Name)
			for subCmd := range documentCommands(children, index, cmd.Commands()) {
				if !yield(subCmd) {
					return
				}
				*index++
			}
		}
	}
}
