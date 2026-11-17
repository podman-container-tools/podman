// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"unicode"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag/mangling"

	golangfuncs "github.com/go-swagger/go-swagger/generator/internal/funcmaps/golang"
)

// exportGoName returns the Go identifier to emit in generated code.
func exportGoName(rawName string, explicit bool, mangler mangling.NameMangler) string {
	if rawName == "" {
		return "Empty"
	}
	if explicit {
		runes := []rune(rawName)
		switch runes[0] {
		case '+', '-', '#', '_', '*', '/', '=':
			return golangfuncs.PrefixForName(rawName)
		}
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}
	// Double ToGoName matches legacy behavior.
	return mangler.ToGoName(mangler.ToGoName(rawName))
}

func schemaGoName(sch *spec.Schema, fallback string, mangler mangling.NameMangler) string {
	return extensionGoName(sch.Extensions, fallback, mangler)
}

// extensionGoName returns the exported Go identifier for an object that may carry x-go-name.
// When x-go-name is present but invalid, the fallback is mangled normally (no error).
func extensionGoName(ext spec.Extensions, fallback string, mangler mangling.NameMangler) string {
	name, err := extensionGoNameOrError(ext, fallback, mangler)
	if err != nil {
		return exportGoName(fallback, false, mangler)
	}
	return name
}

func extensionGoNameOrError(ext spec.Extensions, fallback string, mangler mangling.NameMangler) (string, error) {
	if raw, exists := ext[xGoName]; exists {
		gn, ok := raw.(string)
		if !ok {
			return "", fmt.Errorf(`"x-go-name" field must be a string, not a %T`, raw)
		}
		name := exportGoName(gn, true, mangler)
		// Input sanitization (prevents invalid codegen and code injection from an
		// untrusted spec). The explicit x-go-name only has its first rune upper-cased
		// and is otherwise emitted verbatim as a Go identifier. Reject any value that
		// does not yield a plain identifier so it cannot break out of its syntactic
		// slot. Callers using extensionGoName fall back to the mangled name; callers
		// using this function directly (e.g. parameters) surface the error.
		if err := validateGoIdentifierExtension(name); err != nil {
			return "", err
		}

		return name, nil
	}
	return exportGoName(fallback, false, mangler), nil
}
