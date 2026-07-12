//go:build !remote

package types

type APIContextKey int

const (
	DecoderKey APIContextKey = iota
	RuntimeKey
	IdleTrackerKey
	ConnKey
	CompatDecoderKey
)
