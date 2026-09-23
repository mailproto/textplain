package textplain

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func FuzzFixSpacing(f *testing.F) {
	for _, seed := range []string{
		"", " ", "a", "\n\n\n", "* a\n\n* b", "a  \n  b", "\u2007\u034f a \u00ad b",
		"* one\n\nprose\n\n* two\n", "a\n\t\n\tb", "é日😀 \n\t*",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		if !utf8.ValidString(in) {
			return
		}

		out := fixSpacing(in)

		assert.True(t, utf8.ValidString(out), "output is not valid utf-8")
		assert.LessOrEqual(t, len(out), len(in), "spacing only ever removes")

		// the first two runes are passed through untouched, so only look past them
		if _, size := utf8.DecodeRuneInString(out); size < len(out) {
			rest := out[size:]
			assert.NotContains(t, rest, "\n ", "a line is left with leading space")
			assert.NotContains(t, rest, "\n\t", "a line is left with leading tab")
			assert.NotContains(t, rest, "\n\n\n", "more than one blank line survived")
		}
	})
}

func TestFixSpacingIsSingleAllocation(t *testing.T) {
	input := strings.Repeat("Some text.  \n\n   * item\n\t* item\n\nMore    prose.\n", 200)

	assert.Equal(t, 1.0, testing.AllocsPerRun(20, func() {
		_ = fixSpacing(input)
	}))
}
