//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRLimits(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		result, err := parseRLimits(nil)
		require.NoError(t, err)
		assert.Nil(t, result, "nil input must return nil so container_internal.go skips rlimit replacement")
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		result, err := parseRLimits([]WirePOSIXRlimit{})
		require.NoError(t, err)
		assert.Nil(t, result, "empty input must return nil so container_internal.go skips rlimit replacement")
	})

	t.Run("valid rlimits are parsed correctly", func(t *testing.T) {
		input := []WirePOSIXRlimit{
			{Type: "RLIMIT_NOFILE", Soft: 1024, Hard: 2048},
			{Type: "RLIMIT_NPROC", Soft: 512, Hard: 512},
		}
		result, err := parseRLimits(input)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, specs.POSIXRlimit{Type: "RLIMIT_NOFILE", Soft: 1024, Hard: 2048}, result[0])
		assert.Equal(t, specs.POSIXRlimit{Type: "RLIMIT_NPROC", Soft: 512, Hard: 512}, result[1])
	})

	t.Run("unlimited sentinel (uint64 max) is passed through correctly", func(t *testing.T) {
		// UInt64OrMinusOne is a uint64; -1 in JSON unmarshals to ^uint64(0).
		unlimited := UInt64OrMinusOne(^uint64(0))
		input := []WirePOSIXRlimit{
			{Type: "RLIMIT_NOFILE", Soft: unlimited, Hard: unlimited},
		}
		result, err := parseRLimits(input)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, uint64(unlimited), result[0].Soft, "Soft unlimited should be uint64 max")
		assert.Equal(t, uint64(unlimited), result[0].Hard, "Hard unlimited should be uint64 max")
	})

	t.Run("empty type returns error", func(t *testing.T) {
		input := []WirePOSIXRlimit{
			{Type: "", Soft: 1024, Hard: 1024},
		}
		_, err := parseRLimits(input)
		assert.ErrorContains(t, err, "invalid value for POSIXRlimit.type: empty")
	})
}
