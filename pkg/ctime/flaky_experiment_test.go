package ctime

import (
	"math/rand"
	"testing"
)

func TestFlakyExperiment(t *testing.T) {
	if rand.Float64() < 0.3 {
		t.Fatalf("intentional flaky failure")
	}
}