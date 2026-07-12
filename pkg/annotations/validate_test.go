/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package annotations

import (
	"strings"
	"testing"

	"go.podman.io/podman/v6/libpod/define"
)

func TestValidateAnnotations(t *testing.T) {
	successCases := []map[string]string{
		{"simple": "bar"},
		{"now-with-dashes": "bar"},
		{"1-starts-with-num": "bar"},
		{"1234": "bar"},
		{"simple/simple": "bar"},
		{"now-with-dashes/simple": "bar"},
		{"now-with-dashes/now-with-dashes": "bar"},
		{"now.with.dots/simple": "bar"},
		{"now-with.dashes-and.dots/simple": "bar"},
		{"1-num.2-num/3-num": "bar"},
		{"1234/5678": "bar"},
		{"1.2.3.4/5678": "bar"},
		{"UpperCase123": "bar"},
		{"a": strings.Repeat("b", define.TotalAnnotationSizeLimitB-1)},
		{
			"a": strings.Repeat("b", define.TotalAnnotationSizeLimitB/2-1),
			"c": strings.Repeat("d", define.TotalAnnotationSizeLimitB/2-1),
		},
	}

	for i := range successCases {
		if err := ValidateAnnotations(successCases[i]); err != nil {
			t.Errorf("case[%d] expected success, got %v", i, err)
		}
	}

	nameErrorCases := []map[string]string{
		{"nospecialchars^=@": "bar"},
		{"cantendwithadash-": "bar"},
		{"only/one/slash": "bar"},
		{strings.Repeat("a", 254): "bar"},
	}

	for i := range nameErrorCases {
		if err := ValidateAnnotations(nameErrorCases[i]); err == nil {
			t.Errorf("case[%d]: expected failure", i)
		}
	}

	totalSizeErrorCases := []map[string]string{
		{"a": strings.Repeat("b", define.TotalAnnotationSizeLimitB)},
		{
			"a": strings.Repeat("b", define.TotalAnnotationSizeLimitB/2),
			"c": strings.Repeat("d", define.TotalAnnotationSizeLimitB/2),
		},
	}

	for i := range totalSizeErrorCases {
		if err := ValidateAnnotations(totalSizeErrorCases[i]); err == nil {
			t.Errorf("case[%d] expected failure", i)
		}
	}
}
