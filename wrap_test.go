package textplain_test

import (
	"strings"
	"testing"

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

func runeWidths(s string) []int {
	var widths []int
	for _, line := range strings.Split(s, "\n") {
		widths = append(widths, len([]rune(line)))
	}

	return widths
}
