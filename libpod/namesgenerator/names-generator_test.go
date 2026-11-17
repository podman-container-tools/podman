// This file has been copied from
// https://github.com/moby/moby/tree/a52171f5eeb6e553e7c4744abf6b722962b2aca4/internal/namesgenerator.
// It is licensed under the Apache License 2.0.
// See https://github.com/moby/moby/blob/a52171f5eeb6e553e7c4744abf6b722962b2aca4/LICENSE.

package namesgenerator

import (
	"strings"
	"testing"
)

func TestNameFormat(t *testing.T) {
	name := GetRandomName(0)
	if !strings.Contains(name, "_") {
		t.Fatalf("Generated name does not contain an underscore")
	}
	if strings.ContainsAny(name, "0123456789") {
		t.Fatalf("Generated name contains numbers!")
	}
}

func TestNameRetries(t *testing.T) {
	name := GetRandomName(1)
	if !strings.Contains(name, "_") {
		t.Fatalf("Generated name does not contain an underscore")
	}
	if !strings.ContainsAny(name, "0123456789") {
		t.Fatalf("Generated name doesn't contain a number")
	}
}

func BenchmarkGetRandomName(b *testing.B) {
	b.ReportAllocs()
	var out string
	for b.Loop() {
		out = GetRandomName(5)
	}
	b.Log("Last result:", out)
}
