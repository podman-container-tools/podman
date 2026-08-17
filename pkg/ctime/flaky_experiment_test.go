package ctime

import (
    "math/rand"
    "testing"
)

func TestFlakyExperiment(t *testing.T) {
    k := rand.Float64()
    if k < 0.3 {
        t.Fatalf("k = %f: intentional flaky failure", k)
    }
}