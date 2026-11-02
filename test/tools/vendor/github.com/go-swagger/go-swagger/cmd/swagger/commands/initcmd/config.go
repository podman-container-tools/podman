// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package initcmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/go-openapi/swag/jsonutils"
	"github.com/go-swagger/go-swagger/generator"
)

// Config represents a command for initializing a new config file with defaults.
type Config struct {
	Format      string `choice:"json"           choice:"toml"                                         choice:"yaml" choice:"yml" default:"yaml" description:"When present, writes output as json" long:"format" short:"f"` //nolint:staticcheck // duplicate tags are the way to specify options
	Destination string `default:"./config.yaml" description:"Output destination file or - for stdout" long:"dest"   short:"d"`
}

func (c Config) Usage() string {
	return "[config-OPTIONS] [config]"
}

// Execute this command.
func (c *Config) Execute(args []string) error {
	dest, err := resolveDest(c.Destination, args, c.Format, filepath.Join(".", "config"))
	if err != nil {
		return err
	}

	opts := generator.NewGenOpts()
	if err = opts.Seed(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	var conf map[string]any
	if err = jsonutils.FromDynamicJSON(opts, &conf); err != nil {
		return fmt.Errorf("conversion: %w", err)
	}

	v := viper.New()
	v.SetConfigType(c.Format)

	if err = v.MergeConfigMap(conf); err != nil {
		return err
	}

	var file *os.File
	if dest == "-" {
		file = os.Stdout
	} else {
		file, err = os.Create(dest)
		if err != nil {
			return err
		}

		defer file.Close()
	}

	log.Println("creating default config file in", dest)
	if err = v.WriteConfigTo(file); err != nil {
		return err
	}

	return nil
}
