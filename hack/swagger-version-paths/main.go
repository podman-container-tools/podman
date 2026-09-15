package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	libpodPathPrefix          = "/libpod/"
	versionedLibpodPathPrefix = "/v{version}/libpod/"
	libpodPingPath            = "/libpod/_ping"
	libpodVersionParameter    = "libpodApiVersion"
	libpodVersionReference    = "#/parameters/" + libpodVersionParameter
)

var libpodVersionDefinition = json.RawMessage(`{
	"name": "version",
	"in": "path",
	"required": true,
	"type": "string",
	"pattern": "^[0-9][0-9A-Za-z.-]*$",
	"description": "Libpod API version. Use the Libpod-API-Version response header from GET /_ping."
}`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	inputPath := flag.String("input", "", "path to the generated Swagger YAML")
	outputPath := flag.String("output", "", "path for the transformed Swagger YAML")
	flag.Parse()

	if *inputPath == "" || *outputPath == "" {
		return errors.New("both -input and -output are required")
	}
	if filepath.Clean(*inputPath) == filepath.Clean(*outputPath) {
		return errors.New("-input and -output must be different files")
	}

	input, err := os.ReadFile(*inputPath)
	if err != nil {
		return fmt.Errorf("reading generated Swagger: %w", err)
	}

	output, err := transformSwagger(input)
	if err != nil {
		return err
	}

	if err := os.WriteFile(*outputPath, output, 0o644); err != nil {
		return fmt.Errorf("writing transformed Swagger: %w", err)
	}
	return nil
}

func transformSwagger(input []byte) ([]byte, error) {
	documentJSON, err := yaml.YAMLToJSON(input)
	if err != nil {
		return nil, fmt.Errorf("parsing generated Swagger: %w", err)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(documentJSON, &document); err != nil {
		return nil, fmt.Errorf("decoding generated Swagger: %w", err)
	}

	pathsJSON, found := document["paths"]
	if !found {
		return nil, errors.New("generated Swagger has no paths")
	}
	var paths map[string]json.RawMessage
	if err := json.Unmarshal(pathsJSON, &paths); err != nil {
		return nil, fmt.Errorf("decoding Swagger paths: %w", err)
	}

	transformed := 0
	for path, pathItem := range paths {
		if !strings.HasPrefix(path, libpodPathPrefix) || path == libpodPingPath {
			continue
		}

		versionedPath := versionedLibpodPathPrefix + strings.TrimPrefix(path, libpodPathPrefix)
		if _, exists := paths[versionedPath]; exists {
			return nil, fmt.Errorf("versioned Libpod path %q already exists", versionedPath)
		}

		pathItem, err = addVersionParameter(path, pathItem)
		if err != nil {
			return nil, err
		}

		delete(paths, path)
		paths[versionedPath] = pathItem
		transformed++
	}
	if transformed == 0 {
		return nil, errors.New("generated Swagger has no versioned Libpod paths")
	}

	parameters := make(map[string]json.RawMessage)
	if parametersJSON, found := document["parameters"]; found {
		if err := json.Unmarshal(parametersJSON, &parameters); err != nil {
			return nil, fmt.Errorf("decoding Swagger parameters: %w", err)
		}
	}
	if _, exists := parameters[libpodVersionParameter]; exists {
		return nil, fmt.Errorf("swagger parameter %q already exists", libpodVersionParameter)
	}
	parameters[libpodVersionParameter] = libpodVersionDefinition

	document["paths"], err = json.Marshal(paths)
	if err != nil {
		return nil, fmt.Errorf("encoding Swagger paths: %w", err)
	}
	document["parameters"], err = json.Marshal(parameters)
	if err != nil {
		return nil, fmt.Errorf("encoding Swagger parameters: %w", err)
	}

	documentJSON, err = json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encoding transformed Swagger: %w", err)
	}
	output, err := yaml.JSONToYAML(documentJSON)
	if err != nil {
		return nil, fmt.Errorf("rendering transformed Swagger: %w", err)
	}
	return output, nil
}

func addVersionParameter(path string, pathItemJSON json.RawMessage) (json.RawMessage, error) {
	var pathItem map[string]json.RawMessage
	if err := json.Unmarshal(pathItemJSON, &pathItem); err != nil {
		return nil, fmt.Errorf("decoding Swagger path %q: %w", path, err)
	}

	parameters := make([]json.RawMessage, 0, 1)
	if parametersJSON, found := pathItem["parameters"]; found {
		if err := json.Unmarshal(parametersJSON, &parameters); err != nil {
			return nil, fmt.Errorf("decoding parameters for Swagger path %q: %w", path, err)
		}
	}
	parameters = append(parameters, json.RawMessage(`{"$ref":"`+libpodVersionReference+`"}`))

	var err error
	pathItem["parameters"], err = json.Marshal(parameters)
	if err != nil {
		return nil, fmt.Errorf("encoding parameters for Swagger path %q: %w", path, err)
	}
	output, err := json.Marshal(pathItem)
	if err != nil {
		return nil, fmt.Errorf("encoding Swagger path %q: %w", path, err)
	}
	return output, nil
}
