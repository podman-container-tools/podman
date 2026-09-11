package generator

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag/jsonutils"
	"github.com/go-openapi/swag/stringutils"
)

type respSort struct {
	Code     int
	Response spec.Response
}

type responses []respSort

func (s responses) Len() int           { return len(s) }
func (s responses) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s responses) Less(i, j int) bool { return s[i].Code < s[j].Code }

// sortedResponses produces a sorted list of responses.
//
// Invalid responses with a negative http code are elided.
//
// Sorting guarantees the rendering is stable across generations.
func sortedResponses(input map[int]spec.Response) responses {
	// maintainers: proposal for enhancement:
	// This is redundant with the definition given in struct.go.
	res := make(responses, 0, len(input))
	for k, v := range input {
		if k > 0 {
			res = append(res, respSort{k, v})
		}
	}

	sort.Sort(res)

	return res
}

func dumpOperations(operations GenOperations) error {
	for _, op := range operations {
		if err := dumpOperation(op); err != nil {
			return err
		}
	}

	return nil
}

func dumpOperation(op GenOperation) error {
	var dynamicOp any
	if err := jsonutils.FromDynamicJSON(op, &dynamicOp); err != nil {
		return err
	}

	_ = dumpData(os.Stdout, dynamicOp) // TODO(fred): why error is ignored

	return nil
}

func intersectTags(left, right []string) []string {
	// dedupe
	uniqueTags := make(map[string]struct{}, max(len(left), len(right)))
	for _, l := range left {
		if len(right) == 0 || slices.Contains(right, l) {
			uniqueTags[l] = struct{}{}
		}
	}

	filtered := make([]string, 0, len(uniqueTags))
	// stable output across generations, preserving original order
	for _, k := range left {
		if _, ok := uniqueTags[k]; !ok {
			continue
		}
		filtered = append(filtered, k)
		delete(uniqueTags, k)
	}

	return filtered
}

func hasStreamingFormEnabled(param spec.Parameter, method, path string) (bool, error) {
	raw, found := param.Extensions[xGoServerStreaming]
	if !found {
		return false, nil
	}

	streaming, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf(
			`%s %s, parameter %q: %q must be a boolean, not a %T`,
			method, path, param.Name, xGoServerStreaming, raw,
		)
	}
	if !streaming {
		return false, nil
	}

	if param.In != formData || param.Type != file {
		return false, fmt.Errorf(
			`%s %s, parameter %q: %q may only be enabled on a formData file parameter`,
			method, path, param.Name, xGoServerStreaming,
		)
	}

	return true, nil
}

// filterServerParameters filters parameters that are _not handled_ by codegen.
//
// At this moment, this excludes form-data parameters where self-handled server-side streaming is enabled.
func filterServerParameters(params GenParameters) GenParameters {
	serverParams := make(GenParameters, 0, len(params))
	for _, param := range params {
		if param.IsFormParam() {
			continue
		}
		serverParams = append(serverParams, param)
	}

	return serverParams
}

func deconflictMultipartFormName(params GenParameters) string {
	seenIDs := make(map[string]any, len(params))
	for _, param := range params {
		seenIDs[strings.ToLower(param.ID)] = struct{}{}
	}

	return rename(multipartFormNamePreferences)(seenIDs, multipartFormNamePreferences[0], 0)
}

// headersNeedStrfmt reports whether any response header resolves to a strfmt
// type (e.g. format: uri, uuid, date-time), including types nested in array
// items (e.g. []strfmt.DateTime). Headers are resolved via simpleResolvedType
// and never go through the schema import collection used for body/parameter
// types, so the responses template needs the import registered explicitly.
func headersNeedStrfmt(headers GenHeaders) bool {
	for _, h := range headers {
		if strings.HasPrefix(h.GoType, "strfmt.") {
			return true
		}
		for child := h.Child; child != nil; child = child.Child {
			if strings.HasPrefix(child.GoType, "strfmt.") {
				return true
			}
		}
	}
	return false
}

// rename the variable in use by client template to avoid conflicting
// with param names.
//
// NOTE: this merely protects the timeout field in the client parameter struct,
// fields "Context" and "HTTPClient" remain exposed to name conflicts.
func rename(preferences []string) func(map[string]any, string, int) string {
	return func(seenIDs map[string]any, previous string, index int) string {
		if seenIDs == nil {
			return previous
		}

		current := strings.ToLower(previous)
		if _, ok := seenIDs[current]; !ok {
			return previous
		}

		var next string
		if index < len(preferences)-1 {
			index++
			next = preferences[index]
		} else {
			next = previous + "1"
		}

		return rename(preferences)(seenIDs, next, index)
	}
}

func producesOrDefault(produces []string, fallback []string, defaultProduces string) []string {
	if len(produces) > 0 {
		return produces
	}
	if len(fallback) > 0 {
		return fallback
	}
	return []string{defaultProduces}
}

func schemeOrDefault(schemes []string, defaultScheme string) []string {
	if len(schemes) == 0 {
		return []string{defaultScheme}
	}
	return schemes
}

/**************************************/
/* name deconfiction helpers */
/**************************************/

// deconflictTag ensures generated packages for operations based on tags do not conflict
// with other imports.
func deconflictTag(seenTags []string, pkg string) string {
	return deconflictPkg(pkg, func(pkg string) string { return renameOperationPackage(seenTags, pkg) })
}

// deconflictPrincipal ensures that whenever an external principal package is added, it doesn't conflict
// with standard imports.
func deconflictPrincipal(pkg string) string {
	switch pkg {
	case "principal":
		return renamePrincipalPackage(pkg)
	default:
		return deconflictPkg(pkg, renamePrincipalPackage)
	}
}

// deconflictPkg renames package names which conflict with standard imports.
func deconflictPkg(pkg string, renamer func(string) string) string {
	switch pkg {
	// package conflict with variables
	case "api", "httptransport", "formats", "server":
		fallthrough
	// package conflict with go-openapi imports
	case "conv", "errors", "runtime", "middleware", "security", "spec", "strfmt", "jsonutils", "loads", "netutils", "stringutils", "typeutils", "validate":
		fallthrough
	// package conflict with stdlib/other lib imports
	case "tls", "http", "fmt", "strings", "log", "flags", "pflag", "json", "time":
		return renamer(pkg)
	}

	return pkg
}

func renameOperationPackage(seenTags []string, pkg string) string {
	current := strings.ToLower(pkg) + "ops"
	if len(seenTags) == 0 {
		return current
	}
	for stringutils.ContainsStringsCI(seenTags, current) {
		current += "1"
	}
	return current
}

func renamePrincipalPackage(_ string) string {
	// favors readability over perfect deconfliction
	return "auth"
}

func renameServerPackage(pkg string) string {
	// favors readability over perfect deconfliction
	return "swagger" + pkg + "srv"
}

func renameAPIPackage(pkg string) string {
	// favors readability over perfect deconfliction
	return "swagger" + pkg
}

func renameImplementationPackage(pkg string) string {
	// favors readability over perfect deconfliction
	return "swagger" + pkg + "impl"
}
