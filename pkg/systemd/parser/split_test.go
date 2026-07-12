package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCUnescapeOne(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		acceptNul bool

		ret      rune
		count    int
		eightBit bool
	}{
		{name: "empty", in: "", ret: 0, count: -1},
		{name: `invalid \k`, in: "k", ret: 0, count: -1},
		{name: `\a`, in: "a", ret: '\a', count: 1},
		{name: `\b`, in: "b", ret: '\b', count: 1},
		{name: `\f`, in: "f", ret: '\f', count: 1},
		{name: `\n`, in: "n", ret: '\n', count: 1},
		{name: `\r`, in: "r", ret: '\r', count: 1},
		{name: `\t`, in: "t", ret: '\t', count: 1},
		{name: `\v`, in: "v", ret: '\v', count: 1},
		{name: `\\`, in: "\\", ret: '\\', count: 1},
		{name: `"`, in: "\"", ret: '"', count: 1},
		{name: `'`, in: "'", ret: '\'', count: 1},
		{name: `\s`, in: "s", ret: ' ', count: 1},
		{name: `too short \x1`, in: "x1", ret: 0, count: -1},
		{name: `invalid hex \xzz`, in: "xzz", ret: 0, count: -1},
		{name: `invalid hex \xaz`, in: "xaz", ret: 0, count: -1},
		{name: `\xAb1`, in: "xAb1", ret: '«', count: 3, eightBit: true},
		{name: `\x000 acceptNul=false`, in: "x000", ret: 0, count: -1},
		{name: `\x000 acceptNul=true`, in: "x000", ret: 0, count: 3, eightBit: true, acceptNul: true},
		{name: `too short \u123`, in: "u123", ret: 0, count: -1},
		{name: `\u2a00`, in: "u2a00", ret: '⨀', count: 5},
		{name: `invalid hex \u12v1A`, in: "u12v1A", ret: 0, count: -1},
		{name: `\u0000 acceptNul=false`, in: "u0000", ret: 0, count: -1},
		{name: `\u0000 acceptNul=true`, in: "u0000", ret: 0, count: 5, acceptNul: true},
		{name: `too short \U123`, in: "U123", ret: 0, count: -1},
		{name: `invalid unicode \U12345678`, in: "U12345678", ret: 0, count: -1},
		{name: `invalid hex \U1234V678`, in: "U1234V678", ret: 0, count: -10},
		{name: `\U0001F51F`, in: "U0001F51F", ret: '🔟', count: 9},
		{name: `\U00000000 acceptNul=false`, in: "U00000000", ret: 0, count: -1, acceptNul: false},
		{name: `\U00000000 acceptNul=true`, in: "U00000000", ret: 0, count: 9, acceptNul: true},
		{name: "376", in: "376", ret: 'þ', count: 3, eightBit: true},
		{name: `too short 77`, in: "77", ret: 0, count: -1},
		{name: `invalid octal 792`, in: "792", ret: 0, count: -1},
		{name: `invalid octal 758`, in: "758", ret: 0, count: -1},
		{name: `000 acceptNul=false`, in: "000", ret: 0, count: -1},
		{name: `000 acceptNul=true`, in: "000", ret: 0, count: 3, acceptNul: true, eightBit: true},
		{name: `too big 777 > 255 bytes`, in: "777", ret: 0, count: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			count, out, eightBit := cUnescapeOne(test.in, test.acceptNul)
			assert.Equal(t, test.count, count)
			assert.Equal(t, test.ret, out)
			assert.Equal(t, test.eightBit, eightBit)
		})
	}
}

func TestExtractFirstWordUnescapes(t *testing.T) {
	input := `\a \b \f \n \r \t \v \\ \" \' \s \x50odman is \U0001F51F/\u0031\u0030 \110ello \127orld`
	expected := []string{
		"\a", "\b", "\f", "\n", "\r", "\t", "\v", "\\", "\"", "'", " ",
		"Podman", "is", "🔟/10", "Hello", "World",
	}

	next := input
	for i := range expected {
		word, remaining, _, err := extractFirstWord(next, " ", SplitCUnescape)
		require.NoError(t, err)

		next = remaining
		assert.Equal(t, expected[i], word)
	}
}
