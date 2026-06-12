// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"net/mail"
	"regexp"
	"strings"

	"github.com/go-openapi/spec"
)

type MetaSection struct {
	Comments *ast.CommentGroup
}

func metaTOSSetter(meta *spec.Info) func([]string) {
	return func(lines []string) {
		meta.TermsOfService = JoinDropLast(lines)
	}
}

func metaConsumesSetter(meta *spec.Swagger) func([]string) {
	return func(consumes []string) { meta.Consumes = consumes }
}

func metaProducesSetter(meta *spec.Swagger) func([]string) {
	return func(produces []string) { meta.Produces = produces }
}

func metaSchemeSetter(meta *spec.Swagger) func([]string) {
	return func(schemes []string) { meta.Schemes = schemes }
}

func metaSecuritySetter(meta *spec.Swagger) func([]map[string][]string) {
	return func(secDefs []map[string][]string) { meta.Security = secDefs }
}

func metaSecurityDefinitionsSetter(meta *spec.Swagger) func(json.RawMessage) error {
	return func(jsonValue json.RawMessage) error {
		var jsonData spec.SecurityDefinitions
		err := json.Unmarshal(jsonValue, &jsonData)
		if err != nil {
			return err
		}
		meta.SecurityDefinitions = jsonData
		return nil
	}
}

func metaVendorExtensibleSetter(meta *spec.Swagger) func(json.RawMessage) error {
	return func(jsonValue json.RawMessage) error {
		var jsonData spec.Extensions
		err := json.Unmarshal(jsonValue, &jsonData)
		if err != nil {
			return err
		}
		for k := range jsonData {
			if !rxAllowedExtensions.MatchString(k) {
				return fmt.Errorf("invalid schema extension name, should start from `x-`: %s: %w", k, ErrParser)
			}
		}
		meta.Extensions = jsonData
		return nil
	}
}

func infoVendorExtensibleSetter(meta *spec.Swagger) func(json.RawMessage) error {
	return func(jsonValue json.RawMessage) error {
		var jsonData spec.Extensions
		err := json.Unmarshal(jsonValue, &jsonData)
		if err != nil {
			return err
		}
		for k := range jsonData {
			if !rxAllowedExtensions.MatchString(k) {
				return fmt.Errorf("invalid schema extension name, should start from `x-`: %s: %w", k, ErrParser)
			}
		}
		meta.Info.Extensions = jsonData
		return nil
	}
}

func NewMetaParser(swspec *spec.Swagger) *SectionedParser {
	sp := new(SectionedParser)
	if swspec.Info == nil {
		swspec.Info = new(spec.Info)
	}
	info := swspec.Info
	sp.setTitle = func(lines []string) {
		tosave := JoinDropLast(lines)
		if len(tosave) > 0 {
			tosave = rxStripTitleComments.ReplaceAllString(tosave, "")
		}
		info.Title = tosave
	}
	sp.setDescription = func(lines []string) { info.Description = JoinDropLast(lines) }
	sp.taggers = []TagParser{
		NewMultiLineTagParser("TOS", newMultilineDropEmptyParser(rxTOS, metaTOSSetter(info)), false),
		NewMultiLineTagParser("Consumes", newMultilineDropEmptyParser(rxConsumes, metaConsumesSetter(swspec)), false),
		NewMultiLineTagParser("Produces", newMultilineDropEmptyParser(rxProduces, metaProducesSetter(swspec)), false),
		NewSingleLineTagParser("Schemes", NewSetSchemes(metaSchemeSetter(swspec))),
		NewMultiLineTagParser("Security", newSetSecurity(rxSecuritySchemes, metaSecuritySetter(swspec)), false),
		NewMultiLineTagParser("SecurityDefinitions", NewYAMLParser(WithMatcher(rxSecurity), WithSetter(metaSecurityDefinitionsSetter(swspec))), true),
		NewSingleLineTagParser("Version", &setMetaSingle{Spec: swspec, Rx: rxVersion, Set: setInfoVersion}),
		NewSingleLineTagParser("Host", &setMetaSingle{Spec: swspec, Rx: rxHost, Set: setSwaggerHost}),
		NewSingleLineTagParser("BasePath", &setMetaSingle{swspec, rxBasePath, setSwaggerBasePath}),
		NewSingleLineTagParser("Contact", &setMetaSingle{Spec: swspec, Rx: rxContact, Set: setInfoContact}),
		NewSingleLineTagParser("License", &setMetaSingle{Spec: swspec, Rx: rxLicense, Set: setInfoLicense}),
		NewMultiLineTagParser("YAMLInfoExtensionsBlock", NewYAMLParser(WithMatcher(rxInfoExtensions), WithSetter(infoVendorExtensibleSetter(swspec))), true),
		NewMultiLineTagParser("YAMLExtensionsBlock", NewYAMLParser(WithExtensionMatcher(), WithSetter(metaVendorExtensibleSetter(swspec))), true),
	}

	return sp
}

type setMetaSingle struct {
	Spec *spec.Swagger
	Rx   *regexp.Regexp
	Set  func(spec *spec.Swagger, lines []string) error
}

func (s *setMetaSingle) Matches(line string) bool {
	return s.Rx.MatchString(line)
}

func (s *setMetaSingle) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := s.Rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		return s.Set(s.Spec, []string{matches[1]})
	}
	return nil
}

func setSwaggerHost(swspec *spec.Swagger, lines []string) error {
	lns := lines
	if len(lns) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		lns = []string{"localhost"}
	}
	swspec.Host = lns[0]
	return nil
}

func setSwaggerBasePath(swspec *spec.Swagger, lines []string) error {
	var ln string
	if len(lines) > 0 {
		ln = lines[0]
	}
	swspec.BasePath = ln
	return nil
}

func setInfoVersion(swspec *spec.Swagger, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	info := safeInfo(swspec)
	info.Version = strings.TrimSpace(lines[0])
	return nil
}

func setInfoContact(swspec *spec.Swagger, lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	contact, err := parseContactInfo(lines[0])
	if err != nil {
		return err
	}
	info := safeInfo(swspec)
	info.Contact = contact
	return nil
}

func parseContactInfo(line string) (*spec.ContactInfo, error) {
	nameEmail, url := splitURL(line)
	var name, email string
	if len(nameEmail) > 0 {
		addr, err := mail.ParseAddress(nameEmail)
		if err != nil {
			return nil, err
		}
		name, email = addr.Name, addr.Address
	}
	return &spec.ContactInfo{
		ContactInfoProps: spec.ContactInfoProps{
			URL:   url,
			Name:  name,
			Email: email,
		},
	}, nil
}

func setInfoLicense(swspec *spec.Swagger, lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	info := safeInfo(swspec)
	line := lines[0]
	name, url := splitURL(line)
	info.License = &spec.License{
		LicenseProps: spec.LicenseProps{
			Name: name,
			URL:  url,
		},
	}
	return nil
}

func safeInfo(swspec *spec.Swagger) *spec.Info {
	if swspec.Info == nil {
		swspec.Info = new(spec.Info)
	}
	return swspec.Info
}

// httpFTPScheme matches http://, https://, ws://, wss://.
var httpFTPScheme = regexp.MustCompile("(?:(?:ht|f)tp|ws)s?://")

func splitURL(line string) (notURL, url string) {
	str := strings.TrimSpace(line)
	parts := httpFTPScheme.FindStringIndex(str)
	if len(parts) == 0 {
		if len(str) > 0 {
			notURL = str
		}
		return notURL, ""
	}
	if len(parts) > 0 {
		notURL = strings.TrimSpace(str[:parts[0]])
		url = strings.TrimSpace(str[parts[0]:])
	}
	return notURL, url
}
