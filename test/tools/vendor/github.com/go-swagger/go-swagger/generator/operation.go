// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/spec"
)

const (
	timeoutName = "timeout"
	contextName = "context"
)

var (
	timeoutVarNamePreferences = []string{
		timeoutName,
		"requestTimeout",
		"httpRequestTimeout",
		"swaggerTimeout",
		"operationTimeout",
		"opTimeout",
		"operTimeout",
	}

	contextVarNamePreferences = []string{
		contextName,
		"requestContext",
		"httpRequestContext",
		"swaggerContext",
		"operationContext",
		"opContext",
		"operContext",
	}

	multipartFormNamePreferences = []string{
		"MultipartForm",
		"RequestMultipartForm",
		"HTTPMultipartForm",
		"SwaggerMultipartForm",
		"OperationMultipartForm",
		"OpMultipartForm",
	}
)

// GenerateServerOperation generates a parameter model, parameter validator, http handler implementations for a given operation.
//
// It also generates an operation handler interface that uses the parameter model for handling a valid request.
// Allows for specifying a list of tags to include only certain tags for the generation.
func GenerateServerOperation(operationNames []string, opts *GenOpts) error {
	if err := opts.Prepare(); err != nil {
		return err
	}

	specDoc, analyzed, err := newSpecAnalyzer(opts).analyzeSpec()
	if err != nil {
		return err
	}

	ops := gatherOperations(opts, analyzed, operationNames)
	if len(ops) == 0 {
		return errors.New("no operations were selected")
	}

	for operationName, opRef := range ops {
		method, path, operation := opRef.Method, opRef.Path, opRef.Op
		serverPackage := opts.LanguageOpts.ManglePackagePath(opts.ServerPackage, defaultServerTarget)
		generator := operationGenerator{
			Name:                 operationName,
			Method:               method,
			Path:                 path,
			BasePath:             specDoc.BasePath(),
			ServerPackage:        serverPackage,
			Operation:            *operation,
			SecurityRequirements: analyzed.SecurityRequirementsFor(operation),
			SecurityDefinitions:  analyzed.SecurityDefinitionsFor(operation),
			Target:               filepath.Join(opts.Target, filepath.FromSlash(serverPackage)),
			Doc:                  specDoc,
			Analyzed:             analyzed,
		}
		// injects inherited global options into the generator.
		generator.applyOptions(opts)

		// build the data model and render the operation.
		if err := generator.Generate(); err != nil {
			return err
		}
	}

	return nil
}

type operationGenerator struct {
	Authorized        bool
	IncludeHandler    bool
	IncludeParameters bool
	IncludeResponses  bool
	IncludeValidator  bool
	DumpData          bool

	Principal            string
	Target               string
	Base                 string
	Name                 string
	Method               string
	Path                 string
	BasePath             string
	APIPackage           string
	ModelsPackage        string
	ServerPackage        string
	ClientPackage        string
	Operation            spec.Operation
	SecurityRequirements [][]analysis.SecurityRequirement
	SecurityDefinitions  map[string]spec.SecurityScheme
	Tags                 []string
	DefaultScheme        string
	DefaultProduces      string
	DefaultConsumes      string
	Doc                  *loads.Document
	Analyzed             *analysis.Spec
	GenOpts              *GenOpts
}

// Generate a single operation.
func (o *operationGenerator) Generate() error {
	defaultImports, err := newImportsBuilder(o.GenOpts).defaultImports()
	if err != nil {
		return err
	}

	apiPackage := o.GenOpts.LanguageOpts.ManglePackagePath(o.GenOpts.APIPackage, defaultOperationsTarget)
	imports, err := newImportsBuilder(o.GenOpts).initImports(
		filepath.Join(o.GenOpts.LanguageOpts.ManglePackagePath(o.GenOpts.ServerPackage, defaultServerTarget), apiPackage),
	)
	if err != nil {
		return err
	}

	if err = ensureDedupedImports(defaultImports, imports); err != nil {
		// check the imports map against internal mistakes, e.g. 2 aliases pointing to the same package, same alias
		// pointing to different package. Errors detected here are necessarily go-swagger bugs.
		return err
	}

	bldr := codeGenOpBuilder{
		DefaultImports: defaultImports,
		Imports:        imports,
	}
	// The operation builder inherits most of its settings from the parent generator.
	bldr.applyGeneratorSettings(o)
	_, tags, _ := bldr.analyzeTags()

	op, err := bldr.MakeOperation()
	if err != nil {
		return err
	}

	op.Tags = tags
	operations := make(GenOperations, 0, 1)
	operations = append(operations, op)
	sort.Sort(operations)

	if o.GenOpts.DumpData {
		// short-circuit when dumping the data model for debugging or introspection purpose
		// (e.g. for users trying to construct custom templates).
		return dumpOperations(operations)
	}

	renderer := newRenderer(o.GenOpts)
	for _, pp := range operations {
		op := pp
		if err := renderer.renderOperation(&op); err != nil {
			return err
		}
	}

	return nil
}

// applyOptions propagates all relevant top-level options to the generator.
func (o *operationGenerator) applyOptions(opts *GenOpts) {
	o.APIPackage = opts.LanguageOpts.ManglePackagePath(opts.APIPackage, defaultOperationsTarget)
	o.ModelsPackage = opts.LanguageOpts.ManglePackagePath(opts.ModelPackage, defaultModelsTarget)
	o.ClientPackage = opts.LanguageOpts.ManglePackagePath(opts.ClientPackage, defaultClientTarget)
	o.Principal = principalAlias(opts.Principal)
	o.Base = opts.Target
	o.Tags = opts.Tags
	o.IncludeHandler = opts.IncludeHandler
	o.IncludeParameters = opts.IncludeParameters
	o.IncludeResponses = opts.IncludeResponses
	o.IncludeValidator = opts.IncludeValidator
	o.DumpData = opts.DumpData
	o.DefaultScheme = opts.DefaultScheme
	o.DefaultProduces = opts.DefaultProduces
	o.DefaultConsumes = opts.DefaultConsumes
	o.GenOpts = opts
}

type codeGenOpBuilder struct {
	Authed           bool
	IncludeValidator bool

	Name                string
	Method              string
	Path                string
	BasePath            string
	APIPackage          string
	APIPackageAlias     string
	RootAPIPackage      string
	ModelsPackage       string
	Principal           string
	Target              string
	Operation           spec.Operation
	Doc                 *loads.Document
	PristineDefs        *loads.Document
	Analyzed            *analysis.Spec
	DefaultImports      map[string]string
	Imports             map[string]string
	DefaultScheme       string
	DefaultProduces     string
	DefaultConsumes     string
	Security            [][]analysis.SecurityRequirement
	SecurityDefinitions map[string]spec.SecurityScheme
	ExtraSchemas        map[string]GenSchema
	GenOpts             *GenOpts
}

func (b *codeGenOpBuilder) MakeOperation() (GenOperation, error) {
	// NOTE: we assume flatten is enabled by default (i.e. complex constructs are resolved from the models package),
	// but do not assume the spec is necessarily fully flattened (i.e. all schemas moved to definitions).
	//
	// Fully flattened means that all complex constructs are present as
	// definitions and models produced accordingly in ModelsPackage,
	// whereas minimal flatten simply ensures that there are no weird $ref's in the spec.
	//
	// When some complex anonymous constructs are specified, extra schemas are produced in the operations package.
	//
	// In all cases, resetting definitions to the _original_ (untransformed) spec is not an option:
	// we take from there the spec possibly already transformed by the GenDefinitions stage.
	resolver := newTypeResolver(
		b.GenOpts.LanguageOpts.ManglePackageName(b.ModelsPackage, defaultModelsTarget),
		b.Doc,
		b.GenOpts,
	)
	receiver := "o"
	operation := b.Operation

	// handle the parameters for this operation
	paramsForOperation := b.Analyzed.ParamsFor(b.Method, b.Path)

	// sanitize & deconflict names
	idMapping, timeoutName, ctxName, err := b.paramMappings(paramsForOperation)
	if err != nil {
		return GenOperation{}, err
	}

	// categorize all parameters with all characteristics that matter to code generation.
	splitParams := newParamFlags(b, receiver, resolver, idMapping, len(paramsForOperation))
	err = splitParams.handleParameters(paramsForOperation)
	if err != nil {
		return GenOperation{}, err
	}

	// handle responses: categorize responses with all characteristics that matter to code generation.
	splitResponses := newResponseFlags(b, receiver, resolver)
	if errResp := splitResponses.handleResponses(operation.Responses); errResp != nil {
		return GenOperation{}, errResp
	}

	swaggerSpec := resolver.Doc.Spec()
	schemes, extraSchemes := gatherURISchemes(swaggerSpec, operation)
	originalSchemes := operation.Schemes
	originalExtraSchemes := getExtraSchemes(operation.Extensions)
	produces := producesOrDefault(operation.Produces, swaggerSpec.Produces, b.DefaultProduces)
	consumes := producesOrDefault(operation.Consumes, swaggerSpec.Consumes, b.DefaultConsumes)
	importTarget, err := b.GenOpts.LanguageOpts.BaseImport(b.GenOpts.Target)
	if err != nil {
		return GenOperation{}, errTarget(b.GenOpts.Target, err)
	}

	return GenOperation{
		GenCommon: GenCommon{
			Copyright:        b.GenOpts.Copyright,
			TargetImportPath: importTarget,
		},
		Package:              b.GenOpts.LanguageOpts.ManglePackageName(b.APIPackage, defaultOperationsTarget),
		PackageAlias:         b.APIPackageAlias,
		RootPackage:          b.RootAPIPackage,
		Name:                 b.Name,
		Method:               b.Method,
		Path:                 b.Path,
		BasePath:             b.BasePath,
		Tags:                 operation.Tags,
		UseTags:              len(operation.Tags) > 0 && !b.GenOpts.SkipTagPackages,
		Description:          trimBOM(operation.Description),
		ReceiverName:         receiver,
		DefaultImports:       b.DefaultImports,
		Imports:              b.Imports,
		Params:               splitParams.params,
		ServerParams:         splitParams.serverParams,
		Summary:              trimBOM(operation.Summary),
		QueryParams:          splitParams.qp,
		PathParams:           splitParams.pp,
		HeaderParams:         splitParams.hp,
		FormParams:           splitParams.fp,
		HasQueryParams:       splitParams.hasQueryParams,
		HasPathParams:        splitParams.hasPathParams,
		HasHeaderParams:      splitParams.hasHeaderParams,
		HasFormParams:        splitParams.hasFormParams,
		HasFormValueParams:   splitParams.hasFormValueParams,
		HasFileParams:        splitParams.hasFileParams,
		HasBodyParams:        splitParams.hasBodyParams,
		HasStreamingForm:     splitParams.hasStreamingForm,
		HasStreamingResponse: splitResponses.hasStreamingResponse,
		MultipartFormName:    splitParams.multipartFormName,
		Authorized:           b.Authed,
		Security:             b.makeSecurityRequirements(receiver), // resolved security requirements, for codegen
		SecurityDefinitions:  b.makeSecuritySchemes(receiver),
		SecurityRequirements: securityRequirements(operation.Security), // raw security requirements, for doc
		Principal:            b.Principal,
		Responses:            splitResponses.responses,
		DefaultResponse:      splitResponses.defaultResponse,
		SuccessResponse:      splitResponses.successResponse,
		SuccessResponses:     splitResponses.successResponses,
		ExtraSchemas:         gatherExtraSchemas(b.ExtraSchemas),
		Schemes:              schemeOrDefault(schemes, b.DefaultScheme),
		SchemeOverrides:      originalSchemes,      // raw operation schemes, for doc
		ProducesMediaTypes:   produces,             // resolved produces, for codegen
		ConsumesMediaTypes:   consumes,             // resolved consumes, for codegen
		Produces:             operation.Produces,   // for doc
		Consumes:             operation.Consumes,   // for doc
		ExtraSchemes:         extraSchemes,         // resolved schemes, for codegen
		ExtraSchemeOverrides: originalExtraSchemes, // raw operation extra schemes, for doc
		TimeoutName:          timeoutName,          // deconflicted names for internal fields (and methods)
		ContextName:          ctxName,
		Extensions:           operation.Extensions,
		StrictResponders:     b.GenOpts.StrictResponders,
		PrincipalIsNullable:  principalIsNullable(b.GenOpts.Principal, b.GenOpts.PrincipalCustomIface),
		ExternalDocs:         trimExternalDoc(operation.ExternalDocs),
		ReturnErrors:         b.GenOpts.ReturnErrors,
		WantsGetters:         b.GenOpts.WantsGetters,
	}, nil
}

func (b *codeGenOpBuilder) MakeResponse(receiver, name string, isSuccess bool, resolver *typeResolver, code int, resp spec.Response) (GenResponse, error) {
	// assume minimal flattening has been carried on, so there is not $ref in response (but some may remain in response schema)
	examples := make(GenResponseExamples, 0, len(resp.Examples))
	for k, v := range resp.Examples {
		examples = append(examples, GenResponseExample{MediaType: k, Example: v})
	}
	sort.Sort(examples)

	res := GenResponse{
		Package:          b.GenOpts.LanguageOpts.ManglePackageName(b.APIPackage, defaultOperationsTarget),
		ModelsPackage:    b.ModelsPackage,
		ReceiverName:     receiver,
		Name:             name,
		Description:      trimBOM(resp.Description),
		DefaultImports:   b.DefaultImports,
		Imports:          b.Imports,
		IsSuccess:        isSuccess,
		Code:             code,
		Method:           b.Method,
		Path:             b.Path,
		Extensions:       resp.Extensions,
		StrictResponders: b.GenOpts.StrictResponders,
		OperationName:    b.Name,
		Examples:         examples,
		ReturnErrors:     b.GenOpts.ReturnErrors,
	}

	mangle := b.GenOpts.LanguageOpts.Mangler.ToGoName

	// prepare response headers
	for hName, header := range resp.Headers {
		hdr, err := b.MakeHeader(receiver, hName, header)
		if err != nil {
			return GenResponse{}, err
		}
		res.Headers = append(res.Headers, hdr)
	}
	sort.Sort(res.Headers)

	if resp.Schema != nil {
		// resolve schema model
		schema, ers := b.buildOperationSchema(fmt.Sprintf("%q", name), name+"Body", mangle(name+"Body"), receiver, "i", resp.Schema, resolver)
		if ers != nil {
			return GenResponse{}, ers
		}
		res.Schema = &schema
	}

	if headersNeedStrfmt(res.Headers) {
		// Register the strfmt import on a response-scoped copy of the imports
		// map. Mutating the shared b.Imports would leak the import into the
		// parameters template, which already declares strfmt on its own (#1769).
		imports := maps.Clone(b.Imports)
		if imports == nil {
			imports = make(map[string]string)
		}
		imports["strfmt"] = "github.com/go-openapi/strfmt"
		res.Imports = imports
	}

	return res, nil
}

func (b *codeGenOpBuilder) MakeHeader(receiver, name string, hdr spec.Header) (GenHeader, error) {
	tpe := simpleResolvedType(hdr.Type, hdr.Format, hdr.Items, &hdr.CommonValidations)

	id := extensionGoName(hdr.Extensions, name, b.GenOpts.LanguageOpts.Mangler)
	res := GenHeader{
		sharedValidations: sharedValidations{
			Required:          true,
			SchemaValidations: hdr.Validations(), // NOTE: Required is not defined by the Swagger schema for header. Set arbitrarily to true for convenience in templates.
		},
		resolvedType:     tpe,
		Package:          b.GenOpts.LanguageOpts.ManglePackageName(b.APIPackage, defaultOperationsTarget),
		ReceiverName:     receiver,
		ID:               id,
		GoName:           id,
		Name:             name,
		Path:             fmt.Sprintf("%q", name),
		ValueExpression:  fmt.Sprintf("%s.%s", receiver, id),
		Description:      trimBOM(hdr.Description),
		Default:          hdr.Default,
		HasDefault:       hdr.Default != nil,
		Converter:        stringConverters[tpe.GoType],
		Formatter:        stringFormatters[tpe.GoType],
		ZeroValue:        tpe.Zero(),
		CollectionFormat: hdr.CollectionFormat,
		IndexVar:         "i",
	}
	res.HasValidations, res.HasSliceValidations = b.HasValidations(hdr.CommonValidations, res.resolvedType)

	hasChildValidations := false
	if hdr.Items != nil {
		pi, err := b.MakeHeaderItem(receiver, name+" "+res.IndexVar, res.IndexVar+"i", "fmt.Sprintf(\"%s.%v\", \"header\", "+res.IndexVar+")", res.Name+"I", hdr.Items, nil)
		if err != nil {
			return GenHeader{}, err
		}
		res.Child = &pi
		hasChildValidations = pi.HasValidations
	}
	// we feed the GenHeader structure the same way as we do for
	// GenParameter, even though there is currently no actual validation
	// for response headers.
	res.HasValidations = res.HasValidations || hasChildValidations

	return res, nil
}

func (b *codeGenOpBuilder) MakeHeaderItem(receiver, paramName, indexVar, path, valueExpression string, items, _ *spec.Items) (GenItems, error) {
	var res GenItems
	mangler := b.GenOpts.LanguageOpts.Mangler
	res.resolvedType = simpleResolvedType(items.Type, items.Format, items.Items, &items.CommonValidations)
	res.sharedValidations = sharedValidations{
		Required:          false,
		SchemaValidations: items.Validations(),
	}
	res.Name = paramName
	res.Path = path
	res.Location = "header"
	res.ValueExpression = mangler.ToVarName(valueExpression)
	res.CollectionFormat = items.CollectionFormat
	res.Converter = stringConverters[res.GoType]
	res.Formatter = stringFormatters[res.GoType]
	res.IndexVar = indexVar
	res.HasValidations, res.HasSliceValidations = b.HasValidations(items.CommonValidations, res.resolvedType)
	res.IsEnumCI = b.GenOpts.AllowEnumCI || hasEnumCI(items.Extensions)

	if items.Items == nil {
		return res, nil
	}

	// resolve items in header

	// Recursively follows nested arrays
	// IMPORTANT! transmitting a ValueExpression consistent with the parent's one
	hi, err := b.MakeHeaderItem(receiver, paramName+" "+indexVar, indexVar+"i", "fmt.Sprintf(\"%s.%v\", \"header\", "+indexVar+")", res.ValueExpression+"I", items.Items, items)
	if err != nil {
		return GenItems{}, err
	}
	res.Child = &hi
	hi.Parent = &res
	// Propagates HasValidations flag to outer Items definition (currently not in use: done to remain consistent with parameters)
	res.HasValidations = res.HasValidations || hi.HasValidations

	return res, nil
}

// HasValidations resolves the validation status for simple schema objects.
func (b *codeGenOpBuilder) HasValidations(sh spec.CommonValidations, rt resolvedType) (hasValidations bool, hasSliceValidations bool) {
	hasSliceValidations = sh.HasArrayValidations() || sh.HasEnum()
	hasValidations = sh.HasNumberValidations() || sh.HasStringValidations() || hasSliceValidations || hasFormatValidation(rt)

	return hasValidations, hasSliceValidations
}

func (b *codeGenOpBuilder) MakeParameterItem(receiver, paramName, indexVar, path, valueExpression, location string, resolver *typeResolver, items, _ *spec.Items) (GenItems, error) {
	var res GenItems
	mangler := b.GenOpts.LanguageOpts.Mangler
	res.resolvedType = simpleResolvedType(items.Type, items.Format, items.Items, &items.CommonValidations)

	res.sharedValidations = sharedValidations{
		Required:          false,
		SchemaValidations: items.Validations(),
	}
	res.Name = paramName
	res.Path = path
	res.Location = location
	res.ValueExpression = mangler.ToVarName(valueExpression)
	res.CollectionFormat = items.CollectionFormat
	res.Converter = stringConverters[res.GoType]
	res.Formatter = stringFormatters[res.GoType]
	res.IndexVar = indexVar

	res.HasValidations, res.HasSliceValidations = b.HasValidations(items.CommonValidations, res.resolvedType)
	res.IsEnumCI = b.GenOpts.AllowEnumCI || hasEnumCI(items.Extensions)
	res.NeedsIndex = res.HasValidations || res.Converter != "" || (res.IsCustomFormatter && !res.SkipParse)

	if items.Items == nil {
		return res, nil
	}

	// recurse over items in parameter

	// Recursively follows nested arrays
	// IMPORTANT! transmitting a ValueExpression consistent with the parent's one
	pi, err := b.MakeParameterItem(receiver, paramName+" "+indexVar, indexVar+"i", "fmt.Sprintf(\"%s.%v\", "+path+", "+indexVar+")", res.ValueExpression+"I", location, resolver, items.Items, items)
	if err != nil {
		return GenItems{}, err
	}
	res.Child = &pi
	pi.Parent = &res
	// Propagates HasValidations flag to outer Items definition
	res.HasValidations = res.HasValidations || pi.HasValidations
	res.NeedsIndex = res.NeedsIndex || pi.NeedsIndex

	return res, nil
}

func (b *codeGenOpBuilder) MakeParameter(receiver string, resolver *typeResolver, param spec.Parameter, idMapping map[string]map[string]string) (GenParameter, error) {
	var child *GenItems
	// id, err:= b.inferParameterID(param, idMapping)
	// if err != nil {
	//	return GenParameter{},err
	// }
	var id string
	if len(idMapping) > 0 {
		var ok bool
		id, ok = idMapping[param.In][param.Name]
		if !ok {
			return GenParameter{}, fmt.Errorf(`%s %s, %q has an invalid parameter definition`, b.Method, b.Path, param.Name)
		}
	} else {
		var err error
		id, err = extensionGoNameOrError(param.Extensions, param.Name, b.GenOpts.LanguageOpts.Mangler)
		if err != nil {
			return GenParameter{}, fmt.Errorf(`%s %s, parameter %q: %w`, b.Method, b.Path, param.Name, err)
		}
	}

	res := GenParameter{
		ID:               id,
		GoName:           id,
		Name:             param.Name,
		ModelsPackage:    b.ModelsPackage,
		Path:             fmt.Sprintf("%q", param.Name),
		ValueExpression:  fmt.Sprintf("%s.%s", receiver, id),
		IndexVar:         "i",
		Default:          param.Default,
		HasDefault:       param.Default != nil,
		Description:      trimBOM(param.Description),
		ReceiverName:     receiver,
		CollectionFormat: param.CollectionFormat,
		Child:            child,
		Location:         param.In,
		AllowEmptyValue:  (param.In == "query" || param.In == "formData") && param.AllowEmptyValue,
		Extensions:       param.Extensions,
	}

	// res.CustomTag, err = b.inferCustomTag(param)
	// if err != nil { ...}
	if goCustomTag, ok := param.Extensions["x-go-custom-tag"]; ok {
		customTag, ok := goCustomTag.(string)
		if !ok {
			return GenParameter{}, fmt.Errorf(`%s %s, parameter %q: "x-go-custom-tag" field must be a string, not a %T`,
				b.Method, b.Path, param.Name, goCustomTag)
		}

		res.CustomTag = customTag
	}

	if param.In == "body" {
		// Process parameters declared in body (i.e. have a Schema)
		res.Required = param.Required
		if err := b.MakeBodyParameter(&res, resolver, param.Schema); err != nil {
			return GenParameter{}, err
		}
	} else {
		// Process parameters declared in other inputs: path, query, header (SimpleSchema)
		res.resolvedType = simpleResolvedType(param.Type, param.Format, param.Items, &param.CommonValidations)
		res.sharedValidations = sharedValidations{
			Required:          param.Required,
			SchemaValidations: param.Validations(),
		}

		res.ZeroValue = res.Zero()

		hasChildValidations := false
		if param.Items != nil {
			// Follow Items definition for array parameters
			pi, err := b.MakeParameterItem(receiver, param.Name+" "+res.IndexVar, res.IndexVar+"i", "fmt.Sprintf(\"%s.%v\", "+res.Path+", "+res.IndexVar+")", res.Name+"I", param.In, resolver, param.Items, nil)
			if err != nil {
				return GenParameter{}, err
			}
			res.Child = &pi
			// Propagates HasValidations from child array
			hasChildValidations = pi.HasValidations
		}
		res.IsNullable = !param.Required && !param.AllowEmptyValue
		res.HasValidations, res.HasSliceValidations = b.HasValidations(param.CommonValidations, res.resolvedType)
		res.HasValidations = res.HasValidations || hasChildValidations
		res.IsEnumCI = b.GenOpts.AllowEnumCI || hasEnumCI(param.Extensions)
	}

	// Select codegen strategy for body param validation
	res.Converter = stringConverters[res.GoType]
	res.Formatter = stringFormatters[res.GoType]
	b.setBodyParamValidation(&res)

	return res, nil
}

// MakeBodyParameter constructs a body parameter schema.
func (b *codeGenOpBuilder) MakeBodyParameter(res *GenParameter, resolver *typeResolver, sch *spec.Schema) error {
	mangle := b.GenOpts.LanguageOpts.Mangler.ToGoName
	// resolve schema model
	schema, ers := b.buildOperationSchema(res.Path, b.Operation.ID+"ParamsBody", mangle(b.Operation.ID+" Body"), res.ReceiverName, res.IndexVar, sch, resolver)
	if ers != nil {
		return ers
	}
	res.Schema = &schema
	res.Schema.Required = res.Required // Required in body is managed independently from validations

	// build Child items for nested slices and maps
	var items *GenItems
	res.KeyVar = "k"
	res.Schema.KeyVar = "k"
	switch {
	case schema.IsMap && !schema.IsInterface:
		items = b.MakeBodyParameterItemsAndMaps(res, res.Schema.AdditionalProperties)
	case schema.IsArray:
		items = b.MakeBodyParameterItemsAndMaps(res, res.Schema.Items)
	default:
		items = new(GenItems)
	}

	// templates assume at least one .Child != nil
	res.Child = items
	schema.HasValidations = schema.HasValidations || items.HasValidations
	res.resolvedType = schema.resolvedType

	// simple and schema views share the same validations
	res.sharedValidations = schema.sharedValidations
	res.ZeroValue = schema.Zero()

	return nil
}

// MakeBodyParameterItemsAndMaps clones the .Items schema structure (resp. .AdditionalProperties) as a .GenItems structure
// for compatibility with simple param templates.
//
// Constructed children assume simple structures: any complex object is assumed to be resolved by a model or extra schema definition.
//
//nolint:gocognit // TODO(fredbi): refactor
func (b *codeGenOpBuilder) MakeBodyParameterItemsAndMaps(res *GenParameter, it *GenSchema) *GenItems {
	mangler := b.GenOpts.LanguageOpts.Mangler
	items := new(GenItems)

	if it != nil {
		var prev *GenItems
		next := items
		if res.Schema.IsArray {
			next.Path = "fmt.Sprintf(\"%s.%v\", " + res.Path + ", " + res.IndexVar + ")"
		} else if res.Schema.IsMap {
			next.Path = "fmt.Sprintf(\"%s.%v\", " + res.Path + ", " + res.KeyVar + ")"
		}

		next.Name = res.Name + " " + res.Schema.IndexVar
		next.IndexVar = res.Schema.IndexVar + "i"
		next.KeyVar = res.Schema.KeyVar + "k"
		next.ValueExpression = mangler.ToVarName(res.Name + "I")
		next.Location = "body"

		for it != nil {
			next.resolvedType = it.resolvedType
			next.sharedValidations = it.sharedValidations
			next.Formatter = stringFormatters[it.SwaggerFormat]
			next.Converter = stringConverters[res.GoType]
			next.Parent = prev
			_, next.IsCustomFormatter = customFormatters[it.GoType]
			next.IsCustomFormatter = next.IsCustomFormatter && !it.IsStream

			// special instruction to avoid using CollectionFormat for body params
			next.SkipParse = true

			if prev != nil {
				if prev.IsArray {
					next.Path = "fmt.Sprintf(\"%s.%v\", " + prev.Path + ", " + prev.IndexVar + ")"
				} else if prev.IsMap {
					next.Path = "fmt.Sprintf(\"%s.%v\", " + prev.Path + ", " + prev.KeyVar + ")"
				}
				next.Name = prev.Name + prev.IndexVar
				next.IndexVar = prev.IndexVar + "i"
				next.KeyVar = prev.KeyVar + "k"
				next.ValueExpression = mangler.ToVarName(prev.ValueExpression + "I")
				prev.Child = next
			}

			// found a complex or aliased thing
			// hide details from the aliased type and stop recursing
			if next.IsAliased || next.IsComplexObject {
				next.IsArray = false
				next.IsMap = false
				next.IsCustomFormatter = false
				next.IsComplexObject = true
				next.IsAliased = true
				break
			}
			if next.IsInterface || next.IsStream || next.IsBase64 {
				next.HasValidations = false
			}
			next.NeedsIndex = next.HasValidations || next.Converter != "" || (next.IsCustomFormatter && !next.SkipParse)
			prev = next
			next = new(GenItems)

			switch {
			case it.Items != nil:
				it = it.Items
			case it.AdditionalProperties != nil:
				it = it.AdditionalProperties
			default:
				it = nil
			}
		}

		// propagate HasValidations
		var propag func(child *GenItems) (bool, bool)
		propag = func(child *GenItems) (bool, bool) {
			if child == nil {
				return false, false
			}
			cValidations, cIndex := propag(child.Child)
			child.HasValidations = child.HasValidations || cValidations
			child.NeedsIndex = child.HasValidations || child.Converter != "" || (child.IsCustomFormatter && !child.SkipParse) || cIndex
			return child.HasValidations, child.NeedsIndex
		}
		items.HasValidations, items.NeedsIndex = propag(items)

		// resolve nullability conflicts when declaring body as a map of array of an anonymous complex object
		// (e.g. refer to an extra schema type, which is nullable, but not rendered as a pointer in arrays or maps)
		// Rule: outer type rules (with IsMapNullOverride), inner types are fixed
		var fixNullable func(child *GenItems) string
		fixNullable = func(child *GenItems) string {
			if !child.IsArray && !child.IsMap {
				if child.IsComplexObject {
					return child.GoType
				}
				return ""
			}
			if innerType := fixNullable(child.Child); innerType != "" {
				if child.IsMapNullOverride && child.IsArray {
					child.GoType = "[]" + innerType
					return child.GoType
				}
			}
			return ""
		}
		fixNullable(items)
	}

	return items
}

// applyGeneratorSettings inherits the generator settings into the operation builder.
func (b *codeGenOpBuilder) applyGeneratorSettings(o *operationGenerator) {
	b.ModelsPackage = o.ModelsPackage
	b.Principal = principalAlias(o.GenOpts.Principal)
	b.Target = o.Target
	b.DefaultScheme = o.DefaultScheme
	b.Doc = o.Doc
	b.PristineDefs = o.Doc.Pristine()
	b.Analyzed = o.Analyzed
	b.BasePath = o.BasePath
	b.GenOpts = o.GenOpts
	b.Name = o.Name
	b.Operation = o.Operation
	b.Method = o.Method
	b.Path = o.Path
	b.IncludeValidator = o.IncludeValidator
	b.APIPackage = o.APIPackage // defaults to main operations package
	b.DefaultProduces = o.DefaultProduces
	b.DefaultConsumes = o.DefaultConsumes
	b.Authed = len(o.Analyzed.SecurityRequirementsFor(&o.Operation)) > 0
	b.Security = o.Analyzed.SecurityRequirementsFor(&o.Operation)
	b.SecurityDefinitions = o.Analyzed.SecurityDefinitionsFor(&o.Operation)
	b.RootAPIPackage = o.GenOpts.LanguageOpts.ManglePackageName(o.ServerPackage, defaultServerTarget)
}

// paramMappings yields a map of safe parameter names for an operation.
func (b *codeGenOpBuilder) paramMappings(params map[string]spec.Parameter) (map[string]map[string]string, string, string, error) {
	idMapping := map[string]map[string]string{
		"query":    make(map[string]string, len(params)),
		"path":     make(map[string]string, len(params)),
		"formData": make(map[string]string, len(params)),
		"header":   make(map[string]string, len(params)),
		"body":     make(map[string]string, len(params)),
	}

	// In order to avoid unstable generation, adopt same naming convention
	// for all parameters with same name across locations.
	mangler := b.GenOpts.LanguageOpts.Mangler

	seenIDs := make(map[string]any, len(params))
	for id, p := range params {
		// guard against possible validation failures and/or skipped issues
		if _, found := idMapping[p.In]; !found {
			log.Printf(`warning: parameter named %q has an invalid "in": %q. Skipped`, p.Name, p.In)
			continue
		}
		if p.Name == "" {
			log.Printf(`warning: unnamed parameter (%+v). Skipped`, p)
			continue
		}

		if val, ok := seenIDs[p.Name]; ok {
			previous, ok := val.(struct{ id, in string })
			if !ok {
				panic(fmt.Errorf("internal error: invalid paramMapping: got %T", val))
			}

			prevParam := params[previous.id]

			goID, err := extensionGoNameOrError(p.Extensions, id, mangler)
			if err != nil {
				return nil, "", "", fmt.Errorf("%s %s, parameter %q: %w", b.Method, b.Path, p.Name, err)
			}
			prevGoID, err := extensionGoNameOrError(prevParam.Extensions, previous.id, mangler)
			if err != nil {
				return nil, "", "", fmt.Errorf("%s %s, parameter %q: %w", b.Method, b.Path, prevParam.Name, err)
			}

			idMapping[p.In][p.Name] = goID
			// rewrite the previously found one
			idMapping[previous.in][p.Name] = prevGoID
		} else {
			goID, err := extensionGoNameOrError(p.Extensions, p.Name, mangler)
			if err != nil {
				return nil, "", "", fmt.Errorf("%s %s, parameter %q: %w", b.Method, b.Path, p.Name, err)
			}
			idMapping[p.In][p.Name] = goID
		}
		seenIDs[strings.ToLower(idMapping[p.In][p.Name])] = struct{ id, in string }{id: id, in: p.In}
	}

	// pick a deconflicted private name for timeout for this operation
	timeoutName := rename(timeoutVarNamePreferences)(seenIDs, timeoutVarNamePreferences[0], 0)
	ctxName := rename(contextVarNamePreferences)(seenIDs, contextVarNamePreferences[0], 0)

	return idMapping, timeoutName, ctxName, nil
}

func (b *codeGenOpBuilder) setBodyParamValidation(p *GenParameter) {
	// Determine validation strategy for body param.
	//
	// Here are the distinct strategies:
	// - the body parameter is a model object => delegates
	// - the body parameter is an array of model objects => carry on slice validations, then iterate and delegate
	// - the body parameter is a map of model objects => iterate and delegate
	// - the body parameter is an array of simple objects (including maps)
	// - the body parameter is a map of simple objects (including arrays)
	if !p.IsBodyParam() {
		return
	}

	var hasSimpleBodyParams, hasSimpleBodyItems, hasSimpleBodyMap, hasModelBodyParams, hasModelBodyItems, hasModelBodyMap bool
	s := p.Schema
	if s != nil {
		doNot := s.IsInterface || s.IsStream || s.IsBase64
		// composition of primitive fields must be properly identified: hack this through
		_, isPrimitive := primitives[s.GoType]
		_, isFormatter := customFormatters[s.GoType]
		isComposedPrimitive := s.IsPrimitive && !isPrimitive && !isFormatter

		hasSimpleBodyParams = !s.IsComplexObject && !s.IsAliased && !isComposedPrimitive && !doNot
		hasModelBodyParams = (s.IsComplexObject || s.IsAliased || isComposedPrimitive) && !doNot

		if s.IsArray && s.Items != nil {
			it := s.Items
			doNot = it.IsInterface || it.IsStream || it.IsBase64
			hasSimpleBodyItems = !it.IsComplexObject && !it.IsAliased && !doNot
			hasModelBodyItems = (it.IsComplexObject || it.IsAliased) && !doNot
		}
		if s.IsMap && s.AdditionalProperties != nil {
			it := s.AdditionalProperties
			hasSimpleBodyMap = !it.IsComplexObject && !it.IsAliased && !doNot
			hasModelBodyMap = !hasSimpleBodyMap && !doNot
		}
	}
	// set validation strategy for body param
	p.HasSimpleBodyParams = hasSimpleBodyParams
	p.HasSimpleBodyItems = hasSimpleBodyItems
	p.HasModelBodyParams = hasModelBodyParams
	p.HasModelBodyItems = hasModelBodyItems
	p.HasModelBodyMap = hasModelBodyMap
	p.HasSimpleBodyMap = hasSimpleBodyMap
}

// makeSecuritySchemes produces a sorted list of security schemes for this operation.
func (b *codeGenOpBuilder) makeSecuritySchemes(receiver string) GenSecuritySchemes {
	return gatherSecuritySchemes(b.SecurityDefinitions, b.Name, b.Principal, receiver, principalIsNullable(b.GenOpts.Principal, b.GenOpts.PrincipalCustomIface))
}

// makeSecurityRequirements produces a sorted list of security requirements for this operation.
// As for current, these requirements are not used by codegen (sec. requirement is determined at runtime).
// We keep the order of the slice from the original spec, but sort the inner slice which comes from a map,
// as well as the map of scopes.
func (b *codeGenOpBuilder) makeSecurityRequirements(_ string) []GenSecurityRequirements {
	if b.Security == nil {
		// nil (default requirement) is different than [] (no requirement)
		return nil
	}

	securityRequirements := make([]GenSecurityRequirements, 0, len(b.Security))
	for _, req := range b.Security {
		jointReq := make(GenSecurityRequirements, 0, len(req))
		for _, j := range req {
			scopes := j.Scopes
			sort.Strings(scopes)
			jointReq = append(jointReq, GenSecurityRequirement{
				Name:   j.Name,
				Scopes: scopes,
			})
		}

		// sort joint requirements (come from a map in spec)
		sort.Sort(jointReq)
		securityRequirements = append(securityRequirements, jointReq)
	}

	return securityRequirements
}

// cloneSchema returns a deep copy of a schema.
func (b *codeGenOpBuilder) cloneSchema(schema *spec.Schema) *spec.Schema {
	savedSchema := &spec.Schema{}
	schemaRep, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Errorf("internal error: cannot json marshal a schema: %w", err))
	}

	err = json.Unmarshal(schemaRep, savedSchema)
	if err != nil {
		panic(fmt.Errorf("internal error: cannot json unmarshal a schema: %w", err))
	}

	return savedSchema
}

// saveResolveContext keeps a copy of known definitions and schema to properly roll back on a makeGenSchema() call
// This uses a deep clone the spec document to construct a type resolver which knows about definitions when the making of this operation started,
// and only these definitions. We are not interested in the "original spec", but in the already transformed spec.
func (b *codeGenOpBuilder) saveResolveContext(resolver *typeResolver, schema *spec.Schema) (*typeResolver, *spec.Schema) {
	rslv := newTypeResolver(
		b.GenOpts.LanguageOpts.ManglePackageName(resolver.ModelsPackage, defaultModelsTarget),
		b.PristineDefs,
		b.GenOpts,
	)

	return rslv, b.cloneSchema(schema)
}

// liftExtraSchemas constructs the schema for an anonymous construct with some ExtraSchemas.
//
// When some ExtraSchemas are produced from something else than a definition,
// this indicates we are not running in fully flattened mode and we need to render
// these ExtraSchemas in the operation's package.
// We need to rebuild the schema with a new type resolver to reflect this change in the
// models package.
func (b *codeGenOpBuilder) liftExtraSchemas(resolver, rslv *typeResolver, bs *spec.Schema, sc *schemaGenContext) (schema *GenSchema, err error) {
	// restore resolving state before previous call to makeGenSchema()
	sc.Schema = *bs

	pg := sc.shallowClone()
	pkg := b.GenOpts.LanguageOpts.ManglePackageName(resolver.ModelsPackage, defaultModelsTarget)

	// make a resolver for current package (i.e. operations)
	pg.TypeResolver = newTypeResolver(
		"",
		rslv.Doc,
		b.GenOpts,
	).
		withKeepDefinitionsPackage(pkg).
		withDefinitionPackage(b.APIPackageAlias) // all new extra schemas are going to be in api pkg
	pg.ExtraSchemas = make(map[string]GenSchema, len(sc.ExtraSchemas))
	pg.UseContainerInName = true

	// rebuild schema within local package
	if err = pg.makeGenSchema(); err != nil {
		return nil, err
	}

	// lift nested extra schemas (inlined types)
	if b.ExtraSchemas == nil {
		b.ExtraSchemas = make(map[string]GenSchema, len(pg.ExtraSchemas))
	}

	for _, v := range pg.ExtraSchemas {
		vv := v
		if !v.IsStream {
			b.ExtraSchemas[vv.Name] = vv
		}
	}

	schema = &pg.GenSchema

	return schema, nil
}

// buildOperationSchema constructs a schema for an operation (for body params or responses).
//
// It determines if the schema is readily available from the models package,
// or if a schema has to be generated in the operations package (i.e. is anonymous).
// Whenever an anonymous schema needs some extra schemas, we also determine if these extras are
// available from models or must be generated alongside the schema in the operations package.
//
// Duplicate extra schemas are pruned later on, when operations grouping in packages (e.g. from tags) takes place.
func (b *codeGenOpBuilder) buildOperationSchema(schemaPath, containerName, schemaName, receiverName, indexVar string, sch *spec.Schema, resolver *typeResolver) (GenSchema, error) {
	var schema GenSchema

	if sch == nil {
		sch = &spec.Schema{}
	}
	shallowClonedResolver := *resolver
	shallowClonedResolver.ModelsFullPkg = b.DefaultImports[b.ModelsPackage]
	rslv := &shallowClonedResolver

	pascalize, ok := b.GenOpts.funcMap["pascalize"].(func(string) string)
	if !ok {
		return schema, errors.New("internal error: expected pascalize to be func(string) string")
	}

	jsonify, ok := b.GenOpts.funcMap["json"].(func(any) (string, error))
	if !ok {
		return schema, errors.New("internal error: expected json to be func(any) (string, error)")
	}

	sc := schemaGenContext{
		Path:                       schemaPath,
		Name:                       containerName,
		Receiver:                   receiverName,
		ValueExpr:                  receiverName,
		IndexVar:                   indexVar,
		Schema:                     *sch,
		Required:                   false,
		TypeResolver:               rslv,
		Named:                      false,
		IncludeModel:               true,
		IncludeValidator:           b.GenOpts.IncludeValidator,
		StrictAdditionalProperties: b.GenOpts.StrictAdditionalProperties,
		ExtraSchemas:               make(map[string]GenSchema),
		StructTags:                 b.GenOpts.StructTags,
		mangler:                    b.GenOpts.LanguageOpts.Mangler,
		pascalize:                  pascalize,
		jsonify:                    jsonify,
	}

	var (
		br *typeResolver
		bs *spec.Schema
	)

	if sch.Ref.String() == "" {
		// backup the type resolver context
		// (not needed when the schema has a name)
		br, bs = b.saveResolveContext(rslv, sch)
	}

	if err := sc.makeGenSchema(); err != nil {
		return GenSchema{}, err
	}
	maps.Copy(b.Imports, findImports(&sc.GenSchema))

	if sch.Ref.String() == "" && len(sc.ExtraSchemas) > 0 {
		newSchema, err := b.liftExtraSchemas(resolver, br, bs, &sc)
		if err != nil {
			return GenSchema{}, err
		}
		if newSchema != nil {
			schema = *newSchema
		}
	} else {
		schema = sc.GenSchema
	}

	// new schemas will be in api pkg
	schemaPkg := b.GenOpts.LanguageOpts.ManglePackageName(b.APIPackage, "")
	schema.Pkg = schemaPkg

	if !schema.IsAnonymous {
		// we're done with a named schema
		return schema, nil
	}

	// a generated name for anonymous schema
	// TODO: support x-go-name
	hasProperties := len(schema.Properties) > 0
	isAllOf := len(schema.AllOf) > 0
	isInterface := schema.IsInterface
	hasValidations := schema.HasValidations

	// for complex anonymous objects, produce an extra schema
	switch {
	case hasProperties || isAllOf:
		if b.ExtraSchemas == nil {
			b.ExtraSchemas = make(map[string]GenSchema)
		}
		schema.Name = schemaName
		schema.GoName = schemaName
		schema.GoType = schemaName
		schema.IsAnonymous = false
		b.ExtraSchemas[schemaName] = schema

		// constructs new schema to refer to the newly created type
		schema = GenSchema{}
		schema.IsAnonymous = false
		schema.IsComplexObject = true
		schema.SwaggerType = schemaName
		schema.HasValidations = hasValidations
		schema.GoType = schemaName
		schema.Pkg = schemaPkg

		return schema, nil

	case isInterface:
		schema = GenSchema{}
		schema.IsAnonymous = false
		schema.IsComplexObject = false
		schema.IsInterface = true
		schema.HasValidations = false
		schema.GoType = iface

		return schema, nil

	default:
		return schema, nil
	}
}

// analyze tags for an operation.
func (b *codeGenOpBuilder) analyzeTags() (string, []string, bool) {
	var (
		filter         []string
		tag            string
		hasTagOverride bool
	)
	if b.GenOpts != nil {
		filter = b.GenOpts.Tags
	}

	intersected := intersectTags(pruneEmpty(b.Operation.Tags), filter)
	if !b.GenOpts.SkipTagPackages && len(intersected) > 0 {
		// tag = b.inferTagFromExt(intersected)
		//
		// override generation with: x-go-operation-tag
		tag, hasTagOverride = b.Operation.Extensions.GetString(xGoOperationTag)
		if !hasTagOverride {
			// TODO(fred): this part should be delegated to some new TagsFor(operation) in go-openapi/analysis
			tag = intersected[0]
			gtags := b.Doc.Spec().Tags
			for _, gtag := range gtags {
				if gtag.Name != tag {
					continue
				}
				//  honor x-go-name in tag
				if name, hasGoName := gtag.Extensions.GetString(xGoName); hasGoName {
					// NOTE: the tag always run through ManglePackageName below
					// (and the returned value likewise feeds package mangling), which neutralises any potential breakout
					// from non-legit values.
					//
					// Tags legitimately carry non-identifier values (e.g. "nr!nasty" -> package "nr_bang_nasty"), so validating
					// as a Go identifier would reject valid specs.
					tag = name
					break
				}

				//  honor x-go-operation-tag in tag
				if name, hasOpName := gtag.Extensions.GetString(xGoOperationTag); hasOpName {
					tag = name
					break
				}
			}
		}
	}

	if tag == b.APIPackage {
		// conflict with "operations" package is handled separately
		tag = renameOperationPackage(intersected, tag)
	}

	const boundMatchSubExpressions = 2
	if matches := versionedPkgRex.FindStringSubmatch(tag); len(matches) > boundMatchSubExpressions {
		// rename packages like "v1", "v2" ... as they hold a special meaning for go
		tag = "version" + matches[2]
	}

	b.APIPackage = b.GenOpts.LanguageOpts.ManglePackageName(tag, b.APIPackage) // actual package name
	b.APIPackageAlias = deconflictTag(intersected, b.APIPackage)               // deconflicted import alias

	return tag, intersected, len(filter) == 0 || len(filter) > 0 && len(intersected) > 0
}

var versionedPkgRex = regexp.MustCompile(`(?i)^(v)([0-9]+)$`)

// paramFlags holds parameters categorized by their location and characteristics.
//
// This is a helper type the codegen operation builder delegates the parameters categorization to.
type paramFlags struct {
	hasQueryParams     bool
	hasFileParams      bool
	hasFormValueParams bool
	hasPathParams      bool
	hasHeaderParams    bool
	hasBodyParams      bool
	hasFormParams      bool
	hasStreamingForm   bool

	// all parameters, then parameters by location type (query, path, header, form-data)
	params, qp, pp, hp, fp GenParameters
	serverParams           GenParameters
	multipartFormName      string
	receiver               string
	builder                *codeGenOpBuilder
	resolver               *typeResolver
	paramMappings          map[string]map[string]string
}

func newParamFlags(builder *codeGenOpBuilder, receiver string, resolver *typeResolver, paramMappings map[string]map[string]string, numParams int) *paramFlags {
	return &paramFlags{
		builder:       builder,
		receiver:      receiver,
		resolver:      resolver,
		paramMappings: paramMappings,
		params:        make(GenParameters, 0, numParams),
		qp:            make(GenParameters, 0, numParams),
		pp:            make(GenParameters, 0, numParams),
		hp:            make(GenParameters, 0, numParams),
		fp:            make(GenParameters, 0, numParams),
	}
}

// handleParameters categorizes the parameters for an operation.
func (f *paramFlags) handleParameters(params map[string]spec.Parameter) error {
	for _, param := range params {
		if err := f.handleParameter(param); err != nil {
			return err
		}
	}

	// make rendering stable across generations.
	f.sortAll()

	// filter out self-managed parameters for server-side generation: optional streaming form-data parameters
	// for which the generated server does only minimal binding.
	if f.hasStreamingForm {
		f.serverParams = filterServerParameters(f.params)
		f.multipartFormName = deconflictMultipartFormName(f.serverParams)
	} else {
		f.serverParams = f.params
	}

	return nil
}

// handleParameter builds the parameter and categorizes it.
func (f *paramFlags) handleParameter(p spec.Parameter) error {
	cp, err := f.builder.MakeParameter(f.receiver, f.resolver, p, f.paramMappings)
	if err != nil {
		return err
	}

	if cp.IsQueryParam() {
		f.hasQueryParams = true
		f.qp = append(f.qp, cp)
	}

	if cp.IsFormParam() {
		if p.Type == file {
			f.hasFileParams = true
		}
		if p.Type != file {
			f.hasFormValueParams = true
		}
		f.hasFormParams = true
		f.fp = append(f.fp, cp)
	}

	if cp.IsPathParam() {
		f.hasPathParams = true
		f.pp = append(f.pp, cp)
	}

	if cp.IsHeaderParam() {
		f.hasHeaderParams = true
		f.hp = append(f.hp, cp)
	}

	if cp.IsBodyParam() {
		f.hasBodyParams = true
	}

	if !f.hasStreamingForm {
		f.hasStreamingForm, err = hasStreamingFormEnabled(p, f.builder.Method, f.builder.Path)
		if err != nil {
			return err
		}
	}

	f.params = append(f.params, cp)

	return nil
}

// sortAll sorts all parameters so as to produce a stable rendering.
func (f *paramFlags) sortAll() {
	sort.Sort(f.params)
	sort.Sort(f.qp)
	sort.Sort(f.pp)
	sort.Sort(f.hp)
	sort.Sort(f.fp)
}

// responseFlags holds responses categorized by their characteristics (success, streaming, default).
//
// This is a helper type the codegen operation builder delegates the responses categorization to.
type responseFlags struct {
	responses, successResponses []GenResponse
	successResponse             *GenResponse
	defaultResponse             *GenResponse
	hasStreamingResponse        bool
	receiver                    string
	builder                     *codeGenOpBuilder
	resolver                    *typeResolver
}

func newResponseFlags(builder *codeGenOpBuilder, receiver string, resolver *typeResolver) *responseFlags {
	return &responseFlags{
		builder:  builder,
		receiver: receiver,
		resolver: resolver,
	}
}

// makeFallbackDefaultResponse produces a fallback response when nothing is available in the spec.
func (f *responseFlags) makeFallbackDefaultResponse() error {
	defaultResponse, err := f.builder.MakeResponse(f.receiver, f.builder.Name+" default", false, f.resolver, -1, spec.Response{})
	if err != nil {
		return err
	}

	f.defaultResponse = &defaultResponse

	return nil
}

// handleResponses walks the responses in the spec and catagorizes them.
//
// A generated default response may be added when none is provided.
func (f *responseFlags) handleResponses(responses *spec.Responses) error {
	if responses == nil {
		// no response defined - propose a default response whatsoever
		return f.makeFallbackDefaultResponse()
	}

	// sort responses to guarantee stable rendering across generations.
	sorted := sortedResponses(responses.StatusCodeResponses)
	f.responses = make([]GenResponse, 0, len(sorted))
	f.successResponses = make([]GenResponse, 0, len(sorted))

	for _, v := range sorted {
		if err := f.handleResponse(v); err != nil {
			return err
		}
	}

	if responses.Default != nil {
		defaultResponse, err := f.builder.MakeResponse(f.receiver, f.builder.Name+" default", false, f.resolver, -1, *responses.Default)
		if err != nil {
			return err
		}
		f.defaultResponse = &defaultResponse
	} else if len(sorted) == 0 {
		// always render a default response when no responses at all are found.
		//
		// NOTE: a fallback default response is not guaranteed - if there are responses and no default one, then no fallback is added.
		if err := f.makeFallbackDefaultResponse(); err != nil {
			return err
		}
	}

	// spot the first success response.
	for _, resp := range f.successResponses {
		sr := resp
		if sr.IsSuccess {
			f.successResponse = &sr
			break
		}
	}

	// explore responses and determine if at least one is a stream.
	if f.defaultResponse != nil && f.defaultResponse.Schema != nil && f.defaultResponse.Schema.IsStream {
		f.hasStreamingResponse = true
	}

	if !f.hasStreamingResponse {
		for _, sr := range f.successResponses {
			if sr.Schema != nil && sr.Schema.IsStream {
				f.hasStreamingResponse = true
				break
			}
		}

		if !f.hasStreamingResponse {
			for _, r := range f.responses {
				if r.Schema != nil && r.Schema.IsStream {
					f.hasStreamingResponse = true
					break
				}
			}
		}
	}

	return nil
}

func (f *responseFlags) handleResponse(v respSort) error {
	// honor x-go-name in response.
	name, ok := v.Response.Extensions.GetString(xGoName)
	if !ok {
		// look for name of well-known codes.
		name = runtime.Statuses[v.Code]
		if name == "" {
			// non-standard codes deserve some name.
			name = fmt.Sprintf("Status %d", v.Code)
		}
	}

	mangler := f.builder.GenOpts.LanguageOpts.Mangler
	name = mangler.ToJSONName(f.builder.Name + " " + name)
	const (
		httpStatusCodeDivider = 100
		httpStatusCodeSuccess = 2
	)
	isSuccess := v.Code/httpStatusCodeDivider == httpStatusCodeSuccess
	gr, err := f.builder.MakeResponse(f.receiver, name, isSuccess, f.resolver, v.Code, v.Response)
	if err != nil {
		return err
	}

	if isSuccess {
		f.successResponses = append(f.successResponses, gr)
	}

	f.responses = append(f.responses, gr)

	return nil
}
