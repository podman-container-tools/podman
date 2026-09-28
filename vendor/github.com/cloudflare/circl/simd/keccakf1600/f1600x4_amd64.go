//go:build amd64 && !purego
// +build amd64,!purego

package keccakf1600

import "github.com/cloudflare/circl/internal/sha3"

func permuteSIMDx4(state []uint64, turbo bool) { f1600x4AVX2(&state[0], &sha3.RC, turbo) }

func permuteSIMDx2(state []uint64, turbo bool) { permuteScalarX2(state, turbo) }
