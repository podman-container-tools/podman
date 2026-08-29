// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"iter"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/go-openapi/runtime"
)

const (
	jsonSerializer = "json"
	formData       = "formData"
	multipartForm  = "multipartform"
)

type mediaMatcher struct {
	rex  *regexp.Regexp
	name string
}

func mediaTypeNames() iter.Seq[mediaMatcher] {
	return slices.Values([]mediaMatcher{
		{rex: regexp.MustCompile("application/.*json"), name: jsonSerializer},
		{rex: regexp.MustCompile("application/.*yaml"), name: "yaml"},
		{rex: regexp.MustCompile("application/.*protobuf"), name: "protobuf"},
		{rex: regexp.MustCompile("application/.*capnproto"), name: "capnproto"},
		{rex: regexp.MustCompile("application/.*thrift"), name: "thrift"},
		{rex: regexp.MustCompile("(?:application|text)/.*xml"), name: "xml"},
		{rex: regexp.MustCompile("text/.*markdown"), name: "markdown"},
		{rex: regexp.MustCompile("text/.*html"), name: "html"},
		{rex: regexp.MustCompile("text/.*csv"), name: "csv"},
		{rex: regexp.MustCompile("text/.*tsv"), name: "tsv"},
		{rex: regexp.MustCompile("text/.*javascript"), name: "js"},
		{rex: regexp.MustCompile("text/.*css"), name: "css"},
		{rex: regexp.MustCompile("text/.*plain"), name: "txt"},
		{rex: regexp.MustCompile("application/.*octet-stream"), name: "bin"},
		{rex: regexp.MustCompile("application/.*tar"), name: "tar"},
		{rex: regexp.MustCompile("application/.*gzip"), name: "gzip"},
		{rex: regexp.MustCompile("application/.*gz"), name: "gzip"},
		{rex: regexp.MustCompile("application/.*raw-stream"), name: "bin"},
		{rex: regexp.MustCompile("application/x-www-form-urlencoded"), name: "urlform"},
		{rex: regexp.MustCompile("application/javascript"), name: "txt"},
		{rex: regexp.MustCompile("multipart/form-data"), name: multipartForm},
		{rex: regexp.MustCompile("image/.*"), name: "bin"},
		{rex: regexp.MustCompile("audio/.*"), name: "bin"},
		{rex: regexp.MustCompile("application/pdf"), name: "bin"},
	})
}

var knownProducers = map[string]string{
	jsonSerializer: "runtime.JSONProducer()",
	"yaml":         "yamlpc.YAMLProducer()",
	"xml":          "runtime.XMLProducer()",
	"txt":          "runtime.TextProducer()",
	"bin":          "runtime.ByteStreamProducer()",
	"csv":          "runtime.CSVProducer()",
	"urlform":      "runtime.DiscardProducer",
	multipartForm:  "runtime.DiscardProducer",
}

var knownConsumers = map[string]string{
	jsonSerializer: "runtime.JSONConsumer()",
	"yaml":         "yamlpc.YAMLConsumer()",
	"xml":          "runtime.XMLConsumer()",
	"txt":          "runtime.TextConsumer()",
	"bin":          "runtime.ByteStreamConsumer()",
	"csv":          "runtime.CSVConsumer()",
	"urlform":      "runtime.ByteStreamConsumer()",
	multipartForm:  "runtime.ByteStreamConsumer()",
}

func wellKnownMime(tn string) (string, bool) {
	for matcher := range mediaTypeNames() {
		if matcher.rex.MatchString(tn) {
			return matcher.name, true
		}
	}

	return "", false
}

const mimeParamParts = 2

func mediaParameters(orig string) string {
	parts := strings.SplitN(orig, ";", mimeParamParts)
	if len(parts) < mimeParamParts {
		return ""
	}
	return parts[1]
}

func (a *appGenerator) makeSerializers(mediaTypes []string, known func(string) (string, bool)) (GenSerGroups, bool) {
	supportsJSON := false
	uniqueSerializers := make(map[string]*GenSerializer, len(mediaTypes))
	uniqueSerializerGroups := make(map[string]*GenSerGroup, len(mediaTypes))

	// build all required serializers
	for _, media := range mediaTypes {
		key := a.mediaMime(media)
		nm, ok := wellKnownMime(key)
		if !ok {
			// keep this serializer named, even though its implementation is empty (cf. #1557)
			nm = key
		}
		name := a.mangler.ToJSONName(nm)
		impl, _ := known(name)

		ser, ok := uniqueSerializers[key]
		if !ok {
			ser = &GenSerializer{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           name,
				MediaType:      key,
				Implementation: impl,
				Parameters:     []string{},
			}
			uniqueSerializers[key] = ser
		}
		// provide all known parameters (currently unused by codegen templates)
		if params := strings.TrimSpace(mediaParameters(media)); params != "" {
			if !slices.Contains(ser.Parameters, params) {
				ser.Parameters = append(ser.Parameters, params)
			}
		}

		uniqueSerializerGroups[name] = &GenSerGroup{
			GenSerializer: GenSerializer{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           name,
				Implementation: impl,
			},
		}
	}

	if len(uniqueSerializers) == 0 {
		impl, _ := known(jsonSerializer)
		uniqueSerializers[runtime.JSONMime] = &GenSerializer{
			AppName:        a.Name,
			ReceiverName:   a.Receiver,
			Name:           jsonSerializer,
			MediaType:      runtime.JSONMime,
			Implementation: impl,
			Parameters:     []string{},
		}
		uniqueSerializerGroups[jsonSerializer] = &GenSerGroup{
			GenSerializer: GenSerializer{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           jsonSerializer,
				Implementation: impl,
			},
		}
		supportsJSON = true
	}

	// group serializers by consumer/producer to serve several mime media types
	serializerGroups := make(GenSerGroups, 0, len(uniqueSerializers))

	for _, group := range uniqueSerializerGroups {
		if group.Name == jsonSerializer {
			supportsJSON = true
		}
		serializers := make(GenSerializers, 0, len(uniqueSerializers))
		for _, ser := range uniqueSerializers {
			if group.Name == ser.Name {
				sort.Strings(ser.Parameters)
				serializers = append(serializers, *ser)
			}
		}
		sort.Sort(serializers)
		group.AllSerializers = serializers // provides the full list of mime media types for this serializer group
		serializerGroups = append(serializerGroups, *group)
	}
	sort.Sort(serializerGroups)
	return serializerGroups, supportsJSON
}

func (a *appGenerator) makeConsumes() (GenSerGroups, bool) {
	// builds a codegen struct from all consumes in the spec
	return a.makeSerializers(a.Analyzed.RequiredConsumes(), func(media string) (string, bool) {
		c, ok := knownConsumers[media]
		return c, ok
	})
}

func (a *appGenerator) makeProduces() (GenSerGroups, bool) {
	// builds a codegen struct from all produces in the spec
	return a.makeSerializers(a.Analyzed.RequiredProduces(), func(media string) (string, bool) {
		p, ok := knownProducers[media]
		return p, ok
	})
}
