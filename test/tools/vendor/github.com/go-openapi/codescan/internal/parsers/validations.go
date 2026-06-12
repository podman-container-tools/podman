// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/go-openapi/codescan/internal/ifaces"
	oaispec "github.com/go-openapi/spec"
)

var (
	rxMaximum           = regexp.MustCompile(fmt.Sprintf(rxMaximumFmt, ""))
	rxMinimum           = regexp.MustCompile(fmt.Sprintf(rxMinimumFmt, ""))
	rxMultipleOf        = regexp.MustCompile(fmt.Sprintf(rxMultipleOfFmt, ""))
	rxMinItems          = regexp.MustCompile(fmt.Sprintf(rxMinItemsFmt, ""))
	rxMaxItems          = regexp.MustCompile(fmt.Sprintf(rxMaxItemsFmt, ""))
	rxMaxLength         = regexp.MustCompile(fmt.Sprintf(rxMaxLengthFmt, ""))
	rxMinLength         = regexp.MustCompile(fmt.Sprintf(rxMinLengthFmt, ""))
	rxPattern           = regexp.MustCompile(fmt.Sprintf(rxPatternFmt, ""))
	rxCollectionFormat  = regexp.MustCompile(fmt.Sprintf(rxCollectionFormatFmt, ""))
	rxUnique            = regexp.MustCompile(fmt.Sprintf(rxUniqueFmt, ""))
	rxEnumValidation    = regexp.MustCompile(fmt.Sprintf(rxEnumFmt, ""))
	rxDefaultValidation = regexp.MustCompile(fmt.Sprintf(rxDefaultFmt, ""))
	rxExample           = regexp.MustCompile(fmt.Sprintf(rxExampleFmt, ""))
)

type PrefixRxOption func(string) *regexp.Regexp

func WithItemsPrefixLevel(level int) PrefixRxOption {
	// the expression is 1-index based not 0-index
	itemsPrefix := fmt.Sprintf(rxItemsPrefixFmt, level+1)
	return func(expr string) *regexp.Regexp {
		return Rxf(expr, itemsPrefix) // Proposal for enhancement(fred): cache
	}
}

type SetMaximum struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetMaximum(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetMaximum {
	rx := rxMaximum
	for _, apply := range opts {
		rx = apply(rxMaximumFmt)
	}

	return &SetMaximum{
		builder: builder,
		rx:      rx,
	}
}

func (sm *SetMaximum) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 2 && len(matches[2]) > 0 {
		maximum, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return err
		}
		sm.builder.SetMaximum(maximum, matches[1] == "<")
	}
	return nil
}

func (sm *SetMaximum) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type SetMinimum struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetMinimum(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetMinimum {
	rx := rxMinimum
	for _, apply := range opts {
		rx = apply(rxMinimumFmt)
	}

	return &SetMinimum{
		builder: builder,
		rx:      rx,
	}
}

func (sm *SetMinimum) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

func (sm *SetMinimum) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 2 && len(matches[2]) > 0 {
		minimum, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return err
		}
		sm.builder.SetMinimum(minimum, matches[1] == ">")
	}
	return nil
}

type SetMultipleOf struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetMultipleOf(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetMultipleOf {
	rx := rxMultipleOf
	for _, apply := range opts {
		rx = apply(rxMultipleOfFmt)
	}

	return &SetMultipleOf{
		builder: builder,
		rx:      rx,
	}
}

func (sm *SetMultipleOf) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

func (sm *SetMultipleOf) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		multipleOf, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return err
		}
		sm.builder.SetMultipleOf(multipleOf)
	}
	return nil
}

type SetMaxItems struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetMaxItems(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetMaxItems {
	rx := rxMaxItems
	for _, apply := range opts {
		rx = apply(rxMaxItemsFmt)
	}

	return &SetMaxItems{
		builder: builder,
		rx:      rx,
	}
}

func (sm *SetMaxItems) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

func (sm *SetMaxItems) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		maxItems, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return err
		}
		sm.builder.SetMaxItems(maxItems)
	}
	return nil
}

type SetMinItems struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetMinItems(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetMinItems {
	rx := rxMinItems
	for _, apply := range opts {
		rx = apply(rxMinItemsFmt)
	}

	return &SetMinItems{
		builder: builder,
		rx:      rx,
	}
}

func (sm *SetMinItems) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

func (sm *SetMinItems) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		minItems, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return err
		}
		sm.builder.SetMinItems(minItems)
	}
	return nil
}

type SetMaxLength struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetMaxLength(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetMaxLength {
	rx := rxMaxLength
	for _, apply := range opts {
		rx = apply(rxMaxLengthFmt)
	}

	return &SetMaxLength{
		builder: builder,
		rx:      rx,
	}
}

func (sm *SetMaxLength) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		maxLength, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return err
		}
		sm.builder.SetMaxLength(maxLength)
	}
	return nil
}

func (sm *SetMaxLength) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type SetMinLength struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetMinLength(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetMinLength {
	rx := rxMinLength
	for _, apply := range opts {
		rx = apply(rxMinLengthFmt)
	}

	return &SetMinLength{
		builder: builder,
		rx:      rx,
	}
}

func (sm *SetMinLength) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		minLength, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return err
		}
		sm.builder.SetMinLength(minLength)
	}
	return nil
}

func (sm *SetMinLength) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type SetPattern struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetPattern(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetPattern {
	rx := rxPattern
	for _, apply := range opts {
		rx = apply(rxPatternFmt)
	}

	return &SetPattern{
		builder: builder,
		rx:      rx,
	}
}

func (sm *SetPattern) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		sm.builder.SetPattern(matches[1])
	}
	return nil
}

func (sm *SetPattern) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type SetCollectionFormat struct {
	builder ifaces.OperationValidationBuilder
	rx      *regexp.Regexp
}

func NewSetCollectionFormat(builder ifaces.OperationValidationBuilder, opts ...PrefixRxOption) *SetCollectionFormat {
	rx := rxCollectionFormat
	for _, apply := range opts {
		rx = apply(rxCollectionFormatFmt)
	}

	return &SetCollectionFormat{
		builder: builder,
		rx:      rx,
	}
}

func (sm *SetCollectionFormat) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := sm.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		sm.builder.SetCollectionFormat(matches[1])
	}
	return nil
}

func (sm *SetCollectionFormat) Matches(line string) bool {
	return sm.rx.MatchString(line)
}

type SetUnique struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetUnique(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetUnique {
	rx := rxUnique
	for _, apply := range opts {
		rx = apply(rxUniqueFmt)
	}

	return &SetUnique{
		builder: builder,
		rx:      rx,
	}
}

func (su *SetUnique) Matches(line string) bool {
	return su.rx.MatchString(line)
}

func (su *SetUnique) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := su.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		su.builder.SetUnique(req)
	}
	return nil
}

type SetRequiredParam struct {
	tgt *oaispec.Parameter
}

func NewSetRequiredParam(param *oaispec.Parameter) *SetRequiredParam {
	return &SetRequiredParam{
		tgt: param,
	}
}

func (su *SetRequiredParam) Matches(line string) bool {
	return rxRequired.MatchString(line)
}

func (su *SetRequiredParam) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := rxRequired.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		su.tgt.Required = req
	}
	return nil
}

type SetReadOnlySchema struct {
	tgt *oaispec.Schema
}

func NewSetReadOnlySchema(schema *oaispec.Schema) *SetReadOnlySchema {
	return &SetReadOnlySchema{
		tgt: schema,
	}
}

func (su *SetReadOnlySchema) Matches(line string) bool {
	return rxReadOnly.MatchString(line)
}

func (su *SetReadOnlySchema) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := rxReadOnly.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		su.tgt.ReadOnly = req
	}
	return nil
}

type SetRequiredSchema struct {
	Schema *oaispec.Schema
	Field  string
}

func NewSetRequiredSchema(schema *oaispec.Schema, field string) *SetRequiredSchema {
	return &SetRequiredSchema{
		Schema: schema,
		Field:  field,
	}
}

func (su *SetRequiredSchema) Matches(line string) bool {
	return rxRequired.MatchString(line)
}

func (su *SetRequiredSchema) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := rxRequired.FindStringSubmatch(lines[0])
	if len(matches) <= 1 || len(matches[1]) == 0 {
		return nil
	}

	req, err := strconv.ParseBool(matches[1])
	if err != nil {
		return err
	}
	midx := -1
	for i, nm := range su.Schema.Required {
		if nm == su.Field {
			midx = i
			break
		}
	}
	if req {
		if midx < 0 {
			su.Schema.Required = append(su.Schema.Required, su.Field)
		}
	} else if midx >= 0 {
		su.Schema.Required = append(su.Schema.Required[:midx], su.Schema.Required[midx+1:]...)
	}
	return nil
}

type SetDefault struct {
	scheme  *oaispec.SimpleSchema
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetDefault(scheme *oaispec.SimpleSchema, builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetDefault {
	rx := rxDefaultValidation
	for _, apply := range opts {
		rx = apply(rxDefaultFmt)
	}

	return &SetDefault{
		scheme:  scheme,
		builder: builder,
		rx:      rx,
	}
}

func (sd *SetDefault) Matches(line string) bool {
	return sd.rx.MatchString(line)
}

func (sd *SetDefault) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	matches := sd.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		d, err := parseValueFromSchema(matches[1], sd.scheme)
		if err != nil {
			return err
		}
		sd.builder.SetDefault(d)
	}

	return nil
}

type SetExample struct {
	scheme  *oaispec.SimpleSchema
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetExample(scheme *oaispec.SimpleSchema, builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetExample {
	rx := rxExample
	for _, apply := range opts {
		rx = apply(rxExampleFmt)
	}

	return &SetExample{
		scheme:  scheme,
		builder: builder,
		rx:      rx,
	}
}

func (se *SetExample) Matches(line string) bool {
	return se.rx.MatchString(line)
}

func (se *SetExample) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	matches := se.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		d, err := parseValueFromSchema(matches[1], se.scheme)
		if err != nil {
			return err
		}
		se.builder.SetExample(d)
	}

	return nil
}

type SetDiscriminator struct {
	Schema *oaispec.Schema
	Field  string
}

func NewSetDiscriminator(schema *oaispec.Schema, field string) *SetDiscriminator {
	return &SetDiscriminator{
		Schema: schema,
		Field:  field,
	}
}

func (su *SetDiscriminator) Matches(line string) bool {
	return rxDiscriminator.MatchString(line)
}

func (su *SetDiscriminator) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}
	matches := rxDiscriminator.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		if req {
			su.Schema.Discriminator = su.Field
		} else if su.Schema.Discriminator == su.Field {
			su.Schema.Discriminator = ""
		}
	}
	return nil
}
