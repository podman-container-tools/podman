// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	oaispec "github.com/go-openapi/spec"
)

const (
	// r)sponseTag used when specifying a response to point to a defined swagger:response.
	responseTag = "response"

	// bodyTag used when specifying a response to point to a model/schema.
	bodyTag = "body"

	// descriptionTag used when specifying a response that gives a description of the response.
	descriptionTag = "description"
)

type SetOpResponses struct {
	set         func(*oaispec.Response, map[int]oaispec.Response)
	rx          *regexp.Regexp
	definitions map[string]oaispec.Schema
	responses   map[string]oaispec.Response
}

func NewSetResponses(definitions map[string]oaispec.Schema, responses map[string]oaispec.Response, setter func(*oaispec.Response, map[int]oaispec.Response)) *SetOpResponses {
	return &SetOpResponses{
		set:         setter,
		rx:          rxResponses,
		definitions: definitions,
		responses:   responses,
	}
}

func (ss *SetOpResponses) Matches(line string) bool {
	return ss.rx.MatchString(line)
}

func (ss *SetOpResponses) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	var def *oaispec.Response
	var scr map[int]oaispec.Response

	for _, line := range lines {
		var err error
		def, scr, err = ss.parseResponseLine(line, def, scr)
		if err != nil {
			return err
		}
	}

	ss.set(def, scr)

	return nil
}

func (ss *SetOpResponses) parseResponseLine(line string, def *oaispec.Response, scr map[int]oaispec.Response) (*oaispec.Response, map[int]oaispec.Response, error) {
	kv := strings.SplitN(line, ":", kvParts)
	if len(kv) <= 1 {
		return def, scr, nil
	}

	key := strings.TrimSpace(kv[0])
	if key == "" {
		return def, scr, nil
	}

	value := strings.TrimSpace(kv[1])
	if value == "" {
		def, scr = assignResponse(key, oaispec.Response{}, def, scr)
		return def, scr, nil
	}

	refTarget, arrays, isDefinitionRef, description, err := parseTags(value)
	if err != nil {
		return def, scr, err
	}

	// A possible exception for having a definition
	if _, ok := ss.responses[refTarget]; !ok {
		if _, ok := ss.definitions[refTarget]; ok {
			isDefinitionRef = true
		}
	}

	var ref oaispec.Ref
	if isDefinitionRef {
		if description == "" {
			description = refTarget
		}
		ref, err = oaispec.NewRef("#/definitions/" + refTarget)
	} else {
		ref, err = oaispec.NewRef("#/responses/" + refTarget)
	}
	if err != nil {
		return def, scr, err
	}

	// description should used on anyway.
	resp := oaispec.Response{ResponseProps: oaispec.ResponseProps{Description: description}}

	if isDefinitionRef {
		resp.Schema = new(oaispec.Schema)
		resp.Description = description
		if arrays == 0 {
			resp.Schema.Ref = ref
		} else {
			cs := resp.Schema
			for range arrays {
				cs.Typed("array", "")
				cs.Items = new(oaispec.SchemaOrArray)
				cs.Items.Schema = new(oaispec.Schema)
				cs = cs.Items.Schema
			}
			cs.Ref = ref
		}
		// ref. could be empty while use description tag
	} else if len(refTarget) > 0 {
		resp.Ref = ref
	}

	def, scr = assignResponse(key, resp, def, scr)
	return def, scr, nil
}

func parseTags(line string) (modelOrResponse string, arrays int, isDefinitionRef bool, description string, err error) {
	tags := strings.Split(line, " ")
	parsedModelOrResponse := false

	for i, tagAndValue := range tags {
		tagValList := strings.SplitN(tagAndValue, ":", kvParts)
		var tag, value string
		if len(tagValList) > 1 {
			tag = tagValList[0]
			value = tagValList[1]
		} else {
			// Proposal for enhancement: print a warning, and in the long term, do not support untagged values
			//
			// Add a default tag if none is supplied
			if i == 0 {
				tag = responseTag
			} else {
				tag = descriptionTag
			}
			value = tagValList[0]
		}

		foundModelOrResponse := false
		if !parsedModelOrResponse {
			if tag == bodyTag {
				foundModelOrResponse = true
				isDefinitionRef = true
			}
			if tag == responseTag {
				foundModelOrResponse = true
				isDefinitionRef = false
			}
		}
		if foundModelOrResponse {
			// Read the model or response tag
			parsedModelOrResponse = true
			// Check for nested arrays
			arrays = 0
			for strings.HasPrefix(value, "[]") {
				arrays++
				value = value[2:]
			}
			// What's left over is the model name
			modelOrResponse = value
			continue
		}

		if tag == descriptionTag {
			// Descriptions are special, they read the rest of the line
			descriptionWords := []string{value}
			if i < len(tags)-1 {
				descriptionWords = append(descriptionWords, tags[i+1:]...)
			}
			description = strings.Join(descriptionWords, " ")
			break
		}

		if tag == responseTag || tag == bodyTag {
			err = fmt.Errorf("valid tag %s, but not in a valid position: %w", tag, ErrParser)
		} else {
			err = fmt.Errorf("invalid tag: %s: %w", tag, ErrParser)
		}

		// Error case
		return modelOrResponse, arrays, isDefinitionRef, description, err
	}

	// Proposal for enhancement: maybe do, if !parsedModelOrResponse {return some error}

	return modelOrResponse, arrays, isDefinitionRef, description, err
}

func assignResponse(key string, resp oaispec.Response, def *oaispec.Response, scr map[int]oaispec.Response) (*oaispec.Response, map[int]oaispec.Response) {
	if strings.EqualFold("default", key) {
		if def == nil {
			def = &resp
		}
		return def, scr
	}

	if sc, err := strconv.Atoi(key); err == nil {
		if scr == nil {
			scr = make(map[int]oaispec.Response)
		}
		scr[sc] = resp
	}

	return def, scr
}
