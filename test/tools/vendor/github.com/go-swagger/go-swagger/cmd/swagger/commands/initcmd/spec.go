// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package initcmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

// Spec represents a command for initializing a new swagger application.
type Spec struct {
	Format      string   `choice:"yaml"                            choice:"json"                                                                   default:"yaml"  description:"the format for the spec document" long:"format"` //nolint:staticcheck // false positive detecting duplicate tags (it works fine on other files with the same pattern)
	Title       string   `description:"the title of the API"       long:"title"`
	Description string   `description:"the description of the API" long:"description"`
	Version     string   `default:"0.1.0"                          description:"the version of the API"                                            long:"version"`
	Terms       string   `description:"the terms of services"      long:"terms"`
	Consumes    []string `default:"application/json"               description:"add a content type to the global consumes definitions, can repeat" long:"consumes"`
	Produces    []string `default:"application/json"               description:"add a content type to the global produces definitions, can repeat" long:"produces"`
	Schemes     []string `default:"http"                           description:"add a scheme to the global schemes definition, can repeat"         long:"scheme"`
	Contact     struct {
		Name  string `description:"name of the primary contact for the API"  long:"contact.name"`
		URL   string `description:"url of the primary contact for the API"   long:"contact.url"`
		Email string `description:"email of the primary contact for the API" long:"contact.email"`
	}
	License struct {
		Name string `description:"name of the license for the API" long:"license.name"`
		URL  string `description:"url of the license for the API"  long:"license.url"`
	}
}

// Execute this command.
func (s *Spec) Execute(args []string) error {
	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}
	realPath, err := filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	var file *os.File
	switch s.Format {
	case "json":
		file, err = os.Create(filepath.Join(realPath, "swagger.json"))
		if err != nil {
			return err
		}
	case "yaml", "yml":
		file, err = os.Create(filepath.Join(realPath, "swagger.yml"))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid format: %s", s.Format)
	}
	defer file.Close()
	log.Println("creating specification document in", filepath.Join(targetPath, file.Name()))

	var doc spec.Swagger
	info := new(spec.Info)
	doc.Info = info

	doc.Swagger = "2.0"
	doc.Paths = new(spec.Paths)
	doc.Definitions = make(spec.Definitions)

	info.Title = s.Title
	if info.Title == "" {
		info.Title = swag.ToHumanNameTitle(filepath.Base(realPath))
	}
	info.Description = s.Description
	info.Version = s.Version
	info.TermsOfService = s.Terms
	if s.Contact.Name != "" || s.Contact.Email != "" || s.Contact.URL != "" {
		var contact spec.ContactInfo
		contact.Name = s.Contact.Name
		contact.Email = s.Contact.Email
		contact.URL = s.Contact.URL
		info.Contact = &contact
	}
	if s.License.Name != "" || s.License.URL != "" {
		var license spec.License
		license.Name = s.License.Name
		license.URL = s.License.URL
		info.License = &license
	}

	doc.Consumes = append(doc.Consumes, s.Consumes...)
	doc.Produces = append(doc.Produces, s.Produces...)
	doc.Schemes = append(doc.Schemes, s.Schemes...)

	if s.Format == "json" {
		enc := json.NewEncoder(file)
		return enc.Encode(doc)
	}

	b, err := yaml.Marshal(swag.ToDynamicJSON(doc))
	if err != nil {
		return err
	}
	if _, err := file.Write(b); err != nil {
		return err
	}
	return nil
}
