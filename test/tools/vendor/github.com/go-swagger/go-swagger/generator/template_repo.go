// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"errors"
	"fmt"
	"path/filepath"
	"text/template"

	golangfuncs "github.com/go-swagger/go-swagger/generator/internal/funcmaps/golang"
	"github.com/go-swagger/go-swagger/generator/internal/language"
)

var errInternal = errors.New("internal error detected in templates")

var docFormat = map[string]string{
	binary: "binary (byte stream)",
	b64:    "byte (base64 string)",
}

// embeddedAssets adapts the package-level AssetNames/MustAsset functions
// to the [templatesrepo.AssetProvider] interface.
type embeddedAssets struct{}

func (embeddedAssets) AssetNames() []string         { return AssetNames() }
func (embeddedAssets) MustAsset(name string) []byte { return MustAsset(name) }

// DefaultFuncMap yields a map with default functions for use in the templates.
// These are available in every template.
func DefaultFuncMap(lang *language.Options) template.FuncMap {
	f := golangfuncs.FuncMap(lang.Mangler)
	pascalize, ok := f["pascalize"].(func(string) string)
	if !ok {
		panic("internal error: expected pascalize to be func(string) string")
	}

	// Language-specific entries that depend on *LanguageOpts.
	f["varname"] = lang.MangleVarName
	f["snakize"] = lang.MangleFileName
	f["toPackagePath"] = func(name string) string {
		return filepath.FromSlash(lang.ManglePackagePath(name, ""))
	}
	f["toPackage"] = func(name string) string {
		return lang.ManglePackagePath(name, "")
	}
	f["toPackageName"] = func(name string) string {
		return lang.ManglePackageName(name, "")
	}
	f["arrayInitializer"] = lang.ArrayInitializer
	f["imports"] = lang.Imports

	// Generator-type-dependent entries.
	f["paramDocType"] = func(param GenParameter) string {
		return resolvedDocType(param.SwaggerType, param.SwaggerFormat, param.Child)
	}
	f["headerDocType"] = func(header GenHeader) string {
		return resolvedDocType(header.SwaggerType, header.SwaggerFormat, header.Child)
	}
	f["schemaDocType"] = func(in any) string {
		switch schema := in.(type) {
		case GenSchema:
			return resolvedDocSchemaType(schema.SwaggerType, schema.SwaggerFormat, schema.Items)
		case *GenSchema:
			if schema == nil {
				return ""
			}
			return resolvedDocSchemaType(schema.SwaggerType, schema.SwaggerFormat, schema.Items)
		case GenDefinition:
			return resolvedDocSchemaType(schema.SwaggerType, schema.SwaggerFormat, schema.Items)
		case *GenDefinition:
			if schema == nil {
				return ""
			}
			return resolvedDocSchemaType(schema.SwaggerType, schema.SwaggerFormat, schema.Items)
		default:
			panic("dev error: schemaDocType should be called with GenSchema or GenDefinition")
		}
	}
	f["schemaDocMapType"] = func(schema GenSchema) string {
		return resolvedDocElemType("object", schema.SwaggerFormat, &schema.resolvedType)
	}
	f["docCollectionFormat"] = resolvedDocCollectionFormat
	f["path"] = errorPath

	// CLI command helpers that depend on generator types.
	f["cmdName"] = func(in any) (string, error) {
		op, isOperation := in.(GenOperation)
		if !isOperation {
			ptr, ok := in.(*GenOperation)
			if !ok {
				return "", fmt.Errorf("cmdName should be called on a GenOperation, but got: %T", in)
			}
			op = *ptr
		}
		name := "Operation" + pascalize(op.Package) + pascalize(op.Name) + "Cmd"

		return name, nil
	}
	f["cmdGroupName"] = func(in any) (string, error) {
		opGroup, ok := in.(GenOperationGroup)
		if !ok {
			return "", fmt.Errorf("cmdGroupName should be called on a GenOperationGroup, but got: %T", in)
		}
		name := "GroupOfOperations" + pascalize(opGroup.Name) + "Cmd"

		return name, nil
	}

	// assert is used to inject into templates and check for inconsistent/invalid data.
	f["assert"] = func(msg string, assertion bool) (string, error) {
		if !assertion {
			return "", fmt.Errorf("%v: %w", msg, errInternal)
		}

		return "", nil
	}

	return f
}

func defaultAssets() map[string][]byte {
	return map[string][]byte{
		// schema validation templates
		"validation/primitive.gotmpl":    MustAsset("templates/validation/primitive.gotmpl"),
		"validation/customformat.gotmpl": MustAsset("templates/validation/customformat.gotmpl"),
		"validation/structfield.gotmpl":  MustAsset("templates/validation/structfield.gotmpl"),
		"structfield.gotmpl":             MustAsset("templates/structfield.gotmpl"),
		"schemavalidator.gotmpl":         MustAsset("templates/schemavalidator.gotmpl"),
		"schemapolymorphic.gotmpl":       MustAsset("templates/schemapolymorphic.gotmpl"),
		"schemaembedded.gotmpl":          MustAsset("templates/schemaembedded.gotmpl"),
		"validation/minimum.gotmpl":      MustAsset("templates/validation/minimum.gotmpl"),
		"validation/maximum.gotmpl":      MustAsset("templates/validation/maximum.gotmpl"),
		"validation/multipleOf.gotmpl":   MustAsset("templates/validation/multipleOf.gotmpl"),

		// schema serialization templates
		"additionalpropertiesserializer.gotmpl": MustAsset("templates/serializers/additionalpropertiesserializer.gotmpl"),
		"aliasedserializer.gotmpl":              MustAsset("templates/serializers/aliasedserializer.gotmpl"),
		"allofserializer.gotmpl":                MustAsset("templates/serializers/allofserializer.gotmpl"),
		"basetypeserializer.gotmpl":             MustAsset("templates/serializers/basetypeserializer.gotmpl"),
		"marshalbinaryserializer.gotmpl":        MustAsset("templates/serializers/marshalbinaryserializer.gotmpl"),
		"schemaserializer.gotmpl":               MustAsset("templates/serializers/schemaserializer.gotmpl"),
		"subtypeserializer.gotmpl":              MustAsset("templates/serializers/subtypeserializer.gotmpl"),
		"tupleserializer.gotmpl":                MustAsset("templates/serializers/tupleserializer.gotmpl"),

		// schema generation template
		"docstring.gotmpl":  MustAsset("templates/docstring.gotmpl"),
		"schematype.gotmpl": MustAsset("templates/schematype.gotmpl"),
		"schemabody.gotmpl": MustAsset("templates/schemabody.gotmpl"),
		"schema.gotmpl":     MustAsset("templates/schema.gotmpl"),
		"model.gotmpl":      MustAsset("templates/model.gotmpl"),
		"header.gotmpl":     MustAsset("templates/header.gotmpl"),

		// simple schema generation helpers templates
		"simpleschema/defaultsvar.gotmpl":  MustAsset("templates/simpleschema/defaultsvar.gotmpl"),
		"simpleschema/defaultsinit.gotmpl": MustAsset("templates/simpleschema/defaultsinit.gotmpl"),

		"swagger_json_embed.gotmpl": MustAsset("templates/swagger_json_embed.gotmpl"),

		// server templates
		"server/parameter.gotmpl":        MustAsset("templates/server/parameter.gotmpl"),
		"server/urlbuilder.gotmpl":       MustAsset("templates/server/urlbuilder.gotmpl"),
		"server/responses.gotmpl":        MustAsset("templates/server/responses.gotmpl"),
		"server/operation.gotmpl":        MustAsset("templates/server/operation.gotmpl"),
		"server/builder.gotmpl":          MustAsset("templates/server/builder.gotmpl"),
		"server/server.gotmpl":           MustAsset("templates/server/server.gotmpl"),
		"server/configureapi.gotmpl":     MustAsset("templates/server/configureapi.gotmpl"),
		"server/autoconfigureapi.gotmpl": MustAsset("templates/server/autoconfigureapi.gotmpl"),
		"server/main.gotmpl":             MustAsset("templates/server/main.gotmpl"),
		"server/doc.gotmpl":              MustAsset("templates/server/doc.gotmpl"),

		// client templates
		"client/parameter.gotmpl": MustAsset("templates/client/parameter.gotmpl"),
		"client/response.gotmpl":  MustAsset("templates/client/response.gotmpl"),
		"client/client.gotmpl":    MustAsset("templates/client/client.gotmpl"),
		"client/facade.gotmpl":    MustAsset("templates/client/facade.gotmpl"),

		"markdown/docs.gotmpl": MustAsset("templates/markdown/docs.gotmpl"),

		// cli templates
		"cli/cli.gotmpl":           MustAsset("templates/cli/cli.gotmpl"),
		"cli/main.gotmpl":          MustAsset("templates/cli/main.gotmpl"),
		"cli/modelcli.gotmpl":      MustAsset("templates/cli/modelcli.gotmpl"),
		"cli/operation.gotmpl":     MustAsset("templates/cli/operation.gotmpl"),
		"cli/registerflag.gotmpl":  MustAsset("templates/cli/registerflag.gotmpl"),
		"cli/retrieveflag.gotmpl":  MustAsset("templates/cli/retrieveflag.gotmpl"),
		"cli/schema.gotmpl":        MustAsset("templates/cli/schema.gotmpl"),
		"cli/completion.gotmpl":    MustAsset("templates/cli/completion.gotmpl"),
		"cli/documentation.gotmpl": MustAsset("templates/cli/documentation.gotmpl"),
	}
}

func defaultProtectedTemplates() map[string]bool {
	return map[string]bool{
		"dereffedSchemaType":          true,
		"docstring":                   true,
		"header":                      true,
		"mapvalidator":                true,
		"model":                       true,
		"modelvalidator":              true,
		"objectvalidator":             true,
		"primitivefieldvalidator":     true,
		"privstructfield":             true,
		"privtuplefield":              true,
		"propertyValidationDocString": true,
		"propertyvalidator":           true,
		"schema":                      true,
		"schemaBody":                  true,
		"schemaType":                  true,
		"schemabody":                  true,
		"schematype":                  true,
		"schemavalidator":             true,
		"serverDoc":                   true,
		"slicevalidator":              true,
		"structfield":                 true,
		"structfieldIface":            true,
		"subTypeBody":                 true,
		"swaggerJsonEmbed":            true,
		"tuplefield":                  true,
		"tuplefieldIface":             true,
		"typeSchemaType":              true,
		"simpleschemaDefaultsvar":     true,
		"simpleschemaDefaultsinit":    true,

		// validation helpers
		"validationCustomformat": true,
		"validationPrimitive":    true,
		"validationStructfield":  true,
		"withBaseTypeBody":       true,
		"withoutBaseTypeBody":    true,
		"validationMinimum":      true,
		"validationMaximum":      true,
		"validationMultipleOf":   true,

		// all serializers
		"additionalPropertiesSerializer": true,
		"tupleSerializer":                true,
		"schemaSerializer":               true,
		"hasDiscriminatedSerializer":     true,
		"discriminatedSerializer":        true,
	}
}
