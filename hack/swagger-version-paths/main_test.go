package main

import (
	"encoding/json"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestTransformSwagger(t *testing.T) {
	input := []byte(`
swagger: "2.0"
info:
  title: test
  version: 6.0.0
parameters:
  existing:
    name: existing
    in: query
    type: string
paths:
  /info:
    get:
      responses:
        "200":
          description: OK
  /libpod/_ping:
    get:
      responses:
        "200":
          description: OK
  /libpod/info:
    get:
      responses:
        "200":
          description: OK
  /libpod/containers/{name}:
    parameters:
      - name: name
        in: path
        required: true
        type: string
    get:
      responses:
        "200":
          description: OK
`)

	output, err := transformSwagger(input)
	if err != nil {
		t.Fatal(err)
	}

	document := decodeDocument(t, output)
	paths := decodeMap(t, document["paths"])

	for _, path := range []string{
		"/info",
		"/libpod/_ping",
		"/v{version}/libpod/info",
		"/v{version}/libpod/containers/{name}",
	} {
		if _, found := paths[path]; !found {
			t.Errorf("expected path %q", path)
		}
	}
	for _, path := range []string{"/libpod/info", "/libpod/containers/{name}"} {
		if _, found := paths[path]; found {
			t.Errorf("unexpected unversioned path %q", path)
		}
	}

	containerPath := decodeMap(t, paths["/v{version}/libpod/containers/{name}"])
	var pathParameters []map[string]any
	if err := json.Unmarshal(containerPath["parameters"], &pathParameters); err != nil {
		t.Fatal(err)
	}
	if len(pathParameters) != 2 {
		t.Fatalf("expected existing and version parameters, got %d", len(pathParameters))
	}
	if got := pathParameters[1]["$ref"]; got != libpodVersionReference {
		t.Errorf("expected version parameter reference %q, got %q", libpodVersionReference, got)
	}

	parameters := decodeMap(t, document["parameters"])
	if _, found := parameters["existing"]; !found {
		t.Error("existing global parameter was removed")
	}
	var versionParameter map[string]any
	if err := json.Unmarshal(parameters[libpodVersionParameter], &versionParameter); err != nil {
		t.Fatal(err)
	}
	if got := versionParameter["name"]; got != "version" {
		t.Errorf("expected path parameter name version, got %q", got)
	}
	if got := versionParameter["required"]; got != true {
		t.Errorf("expected required path parameter, got %v", got)
	}
}

func TestTransformSwaggerRejectsPathCollision(t *testing.T) {
	input := []byte(`
swagger: "2.0"
paths:
  /libpod/info:
    get: {}
  /v{version}/libpod/info:
    get: {}
`)

	_, err := transformSwagger(input)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected path collision error, got %v", err)
	}
}

func TestTransformSwaggerRequiresLibpodPaths(t *testing.T) {
	input := []byte(`
swagger: "2.0"
paths:
  /info:
    get: {}
`)

	_, err := transformSwagger(input)
	if err == nil || !strings.Contains(err.Error(), "no versioned Libpod paths") {
		t.Fatalf("expected missing Libpod paths error, got %v", err)
	}
}

func decodeDocument(t *testing.T, input []byte) map[string]json.RawMessage {
	t.Helper()

	documentJSON, err := yaml.YAMLToJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(documentJSON, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func decodeMap(t *testing.T, input json.RawMessage) map[string]json.RawMessage {
	t.Helper()

	var output map[string]json.RawMessage
	if err := json.Unmarshal(input, &output); err != nil {
		t.Fatal(err)
	}
	return output
}
