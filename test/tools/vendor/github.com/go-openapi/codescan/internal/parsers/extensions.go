// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-openapi/codescan/internal/logger"
	oaispec "github.com/go-openapi/spec"
)

// alphaChars used when parsing for Vendor Extensions.
const alphaChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type SetOpExtensions struct {
	Set   func(*oaispec.Extensions)
	rx    *regexp.Regexp
	Debug bool
}

func NewSetExtensions(setter func(*oaispec.Extensions), debug bool) *SetOpExtensions {
	return &SetOpExtensions{
		Set:   setter,
		rx:    rxExtensions,
		Debug: debug,
	}
}

func (ss *SetOpExtensions) Matches(line string) bool {
	return ss.rx.MatchString(line)
}

func (ss *SetOpExtensions) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	cleanLines := cleanupScannerLines(lines, rxUncommentHeaders)

	exts := new(oaispec.VendorExtensible)
	extList := make([]extensionObject, 0)
	buildExtensionObjects(lines, cleanLines, 0, &extList, nil)

	// Extensions can be one of the following:
	// key:value pair
	// list/array
	// object
	for _, ext := range extList {
		if m, ok := ext.Root.(map[string]string); ok {
			exts.AddExtension(ext.Extension, m[ext.Extension])
		} else if m, ok := ext.Root.(map[string]*[]string); ok {
			exts.AddExtension(ext.Extension, *m[ext.Extension])
		} else if m, ok := ext.Root.(map[string]any); ok {
			exts.AddExtension(ext.Extension, m[ext.Extension])
		} else {
			logger.DebugLogf(ss.Debug, "Unknown Extension type: %s", fmt.Sprint(reflect.TypeOf(ext.Root)))
		}
	}

	ss.Set(&exts.Extensions)
	return nil
}

type extensionObject struct {
	Extension string
	Root      any
}

type extensionParsingStack []any

// Helper function to walk back through extensions until the proper nest level is reached.
func (stack *extensionParsingStack) walkBack(rawLines []string, lineIndex int) {
	indent := strings.IndexAny(rawLines[lineIndex], alphaChars)
	nextIndent := strings.IndexAny(rawLines[lineIndex+1], alphaChars)
	if nextIndent >= indent {
		return
	}

	// Pop elements off the stack until we're back where we need to be
	runbackIndex := 0
	poppedIndent := 1000
	for {
		checkIndent := strings.IndexAny(rawLines[lineIndex-runbackIndex], alphaChars)
		if nextIndent == checkIndent {
			break
		}
		if checkIndent < poppedIndent {
			*stack = (*stack)[:len(*stack)-1]
			poppedIndent = checkIndent
		}
		runbackIndex++
	}
}

// Recursively parses through the given extension lines, building and adding extension objects as it goes.
// Extensions may be key:value pairs, arrays, or objects.
func buildExtensionObjects(rawLines []string, cleanLines []string, lineIndex int, extObjs *[]extensionObject, stack *extensionParsingStack) {
	if lineIndex >= len(rawLines) {
		if stack != nil {
			if ext, ok := (*stack)[0].(extensionObject); ok {
				*extObjs = append(*extObjs, ext)
			}
		}
		return
	}

	kv := strings.SplitN(cleanLines[lineIndex], ":", kvParts)
	key := strings.TrimSpace(kv[0])
	if key == "" {
		// Some odd empty line
		return
	}

	nextIsList := false
	if lineIndex < len(rawLines)-1 {
		next := strings.SplitAfterN(cleanLines[lineIndex+1], ":", kvParts)
		nextIsList = len(next) == 1
	}

	if len(kv) <= 1 {
		// Should be a list item
		if stack == nil || len(*stack) == 0 {
			return
		}
		stackIndex := len(*stack) - 1
		list, ok := (*stack)[stackIndex].(*[]string)
		if !ok {
			panic(fmt.Errorf("internal error: expected *[]string, got %T: %w", (*stack)[stackIndex], ErrParser))
		}
		*list = append(*list, key)
		(*stack)[stackIndex] = list
		if lineIndex < len(rawLines)-1 && !rxAllowedExtensions.MatchString(cleanLines[lineIndex+1]) {
			stack.walkBack(rawLines, lineIndex)
		}
		buildExtensionObjects(rawLines, cleanLines, lineIndex+1, extObjs, stack)
		return
	}

	// Should be the start of a map or a key:value pair
	value := strings.TrimSpace(kv[1])

	if rxAllowedExtensions.MatchString(key) {
		buildNewExtension(key, value, nextIsList, stack, rawLines, cleanLines, lineIndex, extObjs)
		return
	}

	if stack == nil || len(*stack) == 0 {
		return
	}

	buildStackEntry(key, value, nextIsList, stack, rawLines, cleanLines, lineIndex)
	buildExtensionObjects(rawLines, cleanLines, lineIndex+1, extObjs, stack)
}

// buildNewExtension handles the start of a new x- extension key.
func buildNewExtension(key, value string, nextIsList bool, stack *extensionParsingStack, rawLines, cleanLines []string, lineIndex int, extObjs *[]extensionObject) {
	// Flush any previous extension on the stack
	if stack != nil {
		if ext, ok := (*stack)[0].(extensionObject); ok {
			*extObjs = append(*extObjs, ext)
		}
	}

	if value != "" {
		ext := extensionObject{
			Extension: key,
		}
		// Extension is simple key:value pair, no stack
		rootMap := make(map[string]string)
		rootMap[key] = value
		ext.Root = rootMap
		*extObjs = append(*extObjs, ext)
		buildExtensionObjects(rawLines, cleanLines, lineIndex+1, extObjs, nil)
		return
	}

	ext := extensionObject{
		Extension: key,
	}
	if nextIsList {
		// Extension is an array
		rootMap := make(map[string]*[]string)
		rootList := make([]string, 0)
		rootMap[key] = &rootList
		ext.Root = rootMap
		stack = &extensionParsingStack{}
		*stack = append(*stack, ext)
		rootListMap, ok := ext.Root.(map[string]*[]string)
		if !ok {
			panic(fmt.Errorf("internal error: expected map[string]*[]string, got %T: %w", ext.Root, ErrParser))
		}
		*stack = append(*stack, rootListMap[key])
	} else {
		// Extension is an object
		rootMap := make(map[string]any)
		innerMap := make(map[string]any)
		rootMap[key] = innerMap
		ext.Root = rootMap
		stack = &extensionParsingStack{}
		*stack = append(*stack, ext)
		*stack = append(*stack, innerMap)
	}
	buildExtensionObjects(rawLines, cleanLines, lineIndex+1, extObjs, stack)
}

func assertStackMap(stack *extensionParsingStack, index int) map[string]any {
	asMap, ok := (*stack)[index].(map[string]any)
	if !ok {
		panic(fmt.Errorf("internal error: stack index expected to be map[string]any, but got %T: %w", (*stack)[index], ErrParser))
	}
	return asMap
}

// buildStackEntry adds a key/value, nested list, or nested map to the current stack.
func buildStackEntry(key, value string, nextIsList bool, stack *extensionParsingStack, rawLines, cleanLines []string, lineIndex int) {
	stackIndex := len(*stack) - 1
	if value == "" {
		asMap := assertStackMap(stack, stackIndex)
		if nextIsList {
			// start of new list
			newList := make([]string, 0)
			asMap[key] = &newList
			*stack = append(*stack, &newList)
		} else {
			// start of new map
			newMap := make(map[string]any)
			asMap[key] = newMap
			*stack = append(*stack, newMap)
		}
		return
	}

	// key:value
	if reflect.TypeOf((*stack)[stackIndex]).Kind() == reflect.Map {
		asMap := assertStackMap(stack, stackIndex)
		asMap[key] = value
	}
	if lineIndex < len(rawLines)-1 && !rxAllowedExtensions.MatchString(cleanLines[lineIndex+1]) {
		stack.walkBack(rawLines, lineIndex)
	}
}
