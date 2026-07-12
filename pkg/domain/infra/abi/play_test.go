//go:build !remote && (linux || freebsd)

package abi

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	v12 "go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReadConfigMapFromFile(t *testing.T) {
	tests := []struct {
		name             string
		configMapContent string
		expectError      bool
		expectedErrorMsg string
		expected         []v1.ConfigMap
	}{
		{
			"ValidConfigMap",
			`
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myvar: foo
`,
			false,
			"",
			[]v1.ConfigMap{
				{
					TypeMeta: v12.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: v12.ObjectMeta{
						Name: "foo",
					},
					Data: map[string]string{
						"myvar": "foo",
					},
				},
			},
		},
		{
			"InvalidYAML",
			`
Invalid YAML
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myvar: foo
`,
			true,
			"unable to read as kube YAML",
			[]v1.ConfigMap{},
		},
		{
			"InvalidKind",
			`
apiVersion: v1
kind: InvalidKind
metadata:
  name: foo
data:
  myvar: foo
`,
			true,
			"invalid YAML kind",
			[]v1.ConfigMap{},
		},
		{
			"ValidBinaryDataConfigMap",
			`
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
binaryData:
  data.zip: UEsDBBQACAAIAMm7SlUAAAAAAAAAAAwAAAAIACAAZGF0YS50eHRVVA0AB+qORGM7j0Rj6o5EY3V4CwABBOgDAAAE6AMAAEvKzEssqlRISSxJ5AIAUEsHCN0J2aAOAAAADAAAAFBLAQIUAxQACAAIAMm7SlXdCdmgDgAAAAwAAAAIACAAAAAAAAAAAACkgQAAAABkYXRhLnR4dFVUDQAH6o5EYzuPRGPqjkRjdXgLAAEE6AMAAAToAwAAUEsFBgAAAAABAAEAVgAAAGQAAAAAAA==
`,
			false,
			"",
			[]v1.ConfigMap{
				{
					TypeMeta: v12.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: v12.ObjectMeta{
						Name: "foo",
					},
					BinaryData: map[string][]byte{"data.zip": {0x50, 0x4b, 0x03, 0x04, 0x14, 0x00, 0x08, 0x00, 0x08, 0x00, 0xc9, 0xbb, 0x4a, 0x55, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x08, 0x00, 0x20, 0x00, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x74, 0x78, 0x74, 0x55, 0x54, 0x0d, 0x00, 0x07, 0xea, 0x8e, 0x44, 0x63, 0x3b, 0x8f, 0x44, 0x63, 0xea, 0x8e, 0x44, 0x63, 0x75, 0x78, 0x0b, 0x00, 0x01, 0x04, 0xe8, 0x03, 0x00, 0x00, 0x04, 0xe8, 0x03, 0x00, 0x00, 0x4b, 0xca, 0xcc, 0x4b, 0x2c, 0xaa, 0x54, 0x48, 0x49, 0x2c, 0x49, 0xe4, 0x02, 0x00, 0x50, 0x4b, 0x07, 0x08, 0xdd, 0x09, 0xd9, 0xa0, 0x0e, 0x00, 0x00, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x50, 0x4b, 0x01, 0x02, 0x14, 0x03, 0x14, 0x00, 0x08, 0x00, 0x08, 0x00, 0xc9, 0xbb, 0x4a, 0x55, 0xdd, 0x09, 0xd9, 0xa0, 0x0e, 0x00, 0x00, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x08, 0x00, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa4, 0x81, 0x00, 0x00, 0x00, 0x00, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x74, 0x78, 0x74, 0x55, 0x54, 0x0d, 0x00, 0x07, 0xea, 0x8e, 0x44, 0x63, 0x3b, 0x8f, 0x44, 0x63, 0xea, 0x8e, 0x44, 0x63, 0x75, 0x78, 0x0b, 0x00, 0x01, 0x04, 0xe8, 0x03, 0x00, 0x00, 0x04, 0xe8, 0x03, 0x00, 0x00, 0x50, 0x4b, 0x05, 0x06, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x56, 0x00, 0x00, 0x00, 0x64, 0x00, 0x00, 0x00, 0x00, 0x00}},
				},
			},
		},
		{
			"MultiDocConfigMapFile",
			`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myvar: foo
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: bar
data:
  myvar: bar
`,
			false,
			"",
			[]v1.ConfigMap{
				{
					TypeMeta: v12.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: v12.ObjectMeta{
						Name: "foo",
					},
					Data: map[string]string{
						"myvar": "foo",
					},
				},
				{
					TypeMeta: v12.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: v12.ObjectMeta{
						Name: "bar",
					},
					Data: map[string]string{
						"myvar": "bar",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := bytes.NewReader([]byte(test.configMapContent))
			cm, err := readConfigMapFromFile(buf)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
				for _, expected := range test.expected {
					assert.Contains(t, cm, expected)
				}
			}
		})
	}
}

func TestGetKubeKind(t *testing.T) {
	tests := []struct {
		name             string
		kubeYAML         string
		expectError      bool
		expectedErrorMsg string
		expected         string
	}{
		{
			"ValidKubeYAML",
			`
apiVersion: v1
kind: Pod
`,
			false,
			"",
			"Pod",
		},
		{
			"InvalidKubeYAML",
			"InvalidKubeYAML",
			true,
			"cannot unmarshal",
			"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kind, err := getKubeKind([]byte(test.kubeYAML))
			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, kind)
			}
		})
	}
}

func TestSplitMultiDocYAML(t *testing.T) {
	tests := []struct {
		name             string
		kubeYAML         string
		expectError      bool
		expectedErrorMsg string
		expected         int
	}{
		{
			"ValidNumberOfDocs",
			`
apiVersion: v1
kind: Pod
---
apiVersion: v1
kind: Pod
---
apiVersion: v1
kind: Pod
`,
			false,
			"",
			3,
		},
		{
			"InvalidMultiDocYAML",
			`
apiVersion: v1
kind: Pod
---
apiVersion: v1
kind: Pod
-
`,
			true,
			"multi doc yaml could not be split",
			0,
		},
		{
			"DocWithList",
			`
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Pod
- apiVersion: v1
  kind: Pod
- apiVersion: v1
  kind: Pod
`,
			false,
			"",
			3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			docs, err := splitMultiDocYAML([]byte(test.kubeYAML))
			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, len(docs))
			}
		})
	}
}
