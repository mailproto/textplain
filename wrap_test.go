package textplain_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
)

func TestWrappingInvalidLength(t *testing.T) {
	body := `.stylesheet {
		color: white;
		background-image: url('data:image/png;base64,123456789012345678901234567890');
		font-weight: bold;
		margin: 0px;
	}`

	wrapped := textplain.WordWrap(body, -1)
	assert.Equal(t, body, wrapped)
}

func TestWrappingTrailingWhitespace(t *testing.T) {
	body := "1 23 45\n67\n1234567890 1   "

	wrapped := textplain.WordWrap(body, 13)
	assert.Equal(t, "1 23 45\n67\n1234567890 1 \n", wrapped)

	wrapped = textplain.WordWrap("1234567890"+strings.Repeat(" ", 20), 10)
	assert.Equal(t, "1234567890\n", wrapped)
}

func TestWrappingShorterThanLimit(t *testing.T) {
	body := "1 12 12 1"

	wrapped := textplain.WordWrap(body, 3)
	assert.Equal(t, "1\n12\n12\n1", wrapped)
}

func TestWrappingCountsRunesNotBytes(t *testing.T) {
	// same shape, one ASCII and one multi-byte: both must wrap identically
	ascii := strings.Repeat("ab ", 10)
	accented := strings.Repeat("\u00e1b ", 10)

	assert.Equal(t, runeWidths(textplain.WordWrap(ascii, 20)), runeWidths(textplain.WordWrap(accented, 20)))
	assert.Equal(t, "\u00e1b \u00e1b \u00e1b \u00e1b \u00e1b \u00e1b \u00e1b\n\u00e1b \u00e1b \u00e1b ", textplain.WordWrap(accented, 20))
}

func TestWrappingMultibyteWithoutBreakpoints(t *testing.T) {
	unbroken := strings.Repeat("\u00e9", 40)

	assert.Equal(t, unbroken, textplain.WordWrap(unbroken, 10))
}

func TestWrappingPastARunOfSpaces(t *testing.T) {
	body := "a" + strings.Repeat(" ", 10) + "b c d e f"

	wrapped := textplain.WordWrap(body, 5)
	assert.Equal(t, strings.Fields(body), strings.Fields(wrapped))

	for _, line := range strings.Split(wrapped, "\n") {
		assert.LessOrEqual(t, utf8.RuneCountInString(line), 5)
	}
}

func TestWrappingKeepsBracketsWithTheirContent(t *testing.T) {
	assert.Equal(t, "see\n( abcdefgh )", textplain.WordWrap("see ( abcdefgh )", 12))
	assert.Equal(t, "one two\nthree )", textplain.WordWrap("one two three )", 13))
	assert.Equal(t, "one two\nthree ).", textplain.WordWrap("one two three ).", 14), "punctuation may follow the )")
	assert.Equal(t, "( abcdefghijkl", textplain.WordWrap("( abcdefghijkl", 5), "a leading ( has nowhere to go")
	assert.Equal(t, "abcd )\nefgh", textplain.WordWrap("abcd ) efgh", 5), "nor does a ) with no earlier break")
	assert.Equal(t, "f(\nx)", textplain.WordWrap("f( x)", 3), "only lone brackets are kept together")
}

func runeWidths(s string) []int {
	var widths []int
	for _, line := range strings.Split(s, "\n") {
		widths = append(widths, len([]rune(line)))
	}

	return widths
}

func FuzzWordWrap(f *testing.F) {
	for _, seed := range []string{"", " ", "hello world", "a  b   c", "1 23 45\n67\n1234567890 1   ", "áb áb áb", "日本語 の テキスト", strings.Repeat("é", 40)} {
		for _, width := range []int{-1, 0, 1, 5, 20} {
			f.Add(seed, width)
		}
	}

	f.Fuzz(func(t *testing.T, txt string, lineLength int) {
		if !utf8.ValidString(txt) {
			return
		}

		wrapped := textplain.WordWrap(txt, lineLength)

		assert.True(t, utf8.ValidString(wrapped), "wrapped output is not valid utf-8")
		// wrapping only ever breaks at spaces, so no other character may be lost or moved
		assert.Equal(t, stripWhitespace(txt), stripWhitespace(wrapped))
	})
}

func stripWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' {
			return -1
		}

		return r
	}, s)
}

func BenchmarkWordWrap(b *testing.B) {
	for name, txt := range map[string]string{
		"prose":     strings.Repeat("The quick brown fox jumps over the lazy dog (see https://example.com/a/b). ", 200),
		"mixed":     strings.Repeat("The quick brown fox’s jumps over the lazy dog (see https://example.com/a/b). ", 200),
		"multibyte": strings.Repeat("日本語 の テキスト áb éé ", 400),
		"unbroken":  strings.Repeat("x", 20000),
		"lines":     strings.Repeat("short line\n", 2000),
	} {
		b.Run(name, func(b *testing.B) {
			for range b.N {
				textplain.WordWrap(txt, textplain.DefaultLineLength)
			}
		})
	}
}
