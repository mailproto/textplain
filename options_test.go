package textplain_test

import (
	"strings"
	"testing"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const optionsDoc = `<h1>Title</h1><p>See <a href="https://a.example">one</a> and <a href="https://b.example">two</a>.</p><ul><li>x</li></ul><ol><li>y</li></ol>`

func TestOptions(t *testing.T) {
	cases := []struct {
		name   string
		opts   textplain.Options
		expect string
	}{
		{
			name:   "defaults match Convert",
			opts:   textplain.Options{LineLength: textplain.DefaultLineLength},
			expect: "*****\nTitle\n*****\n\nSee one ( https://a.example ) and two ( https://b.example ).\n\n* x\n1. y",
		},
		{
			name:   "links omitted",
			opts:   textplain.Options{LineLength: textplain.DefaultLineLength, Links: textplain.LinksOmitted},
			expect: "*****\nTitle\n*****\n\nSee one and two.\n\n* x\n1. y",
		},
		{
			name:   "links as footnotes",
			opts:   textplain.Options{LineLength: textplain.DefaultLineLength, Links: textplain.LinksFootnotes},
			expect: "*****\nTitle\n*****\n\nSee one [1] and two [2].\n\n* x\n1. y\n\n[1] https://a.example\n[2] https://b.example",
		},
		{
			name:   "plain headings",
			opts:   textplain.Options{LineLength: textplain.DefaultLineLength, PlainHeadings: true},
			expect: "Title\n\nSee one ( https://a.example ) and two ( https://b.example ).\n\n* x\n1. y",
		},
		{
			name:   "custom item prefixes",
			opts:   textplain.Options{LineLength: textplain.DefaultLineLength, Bullet: "- ", OrderedSuffix: ") "},
			expect: "*****\nTitle\n*****\n\nSee one ( https://a.example ) and two ( https://b.example ).\n\n- x\n1) y",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := textplain.ConvertWithOptions(optionsDoc, tc.opts)
			require.NoError(t, err)
			assert.Equal(t, tc.expect, result)
		})
	}
}

func TestOptionsZeroValueDoesNotWrap(t *testing.T) {
	body := "<p>" + strings.Repeat("word ", 40) + "</p>"

	result, err := textplain.ConvertWithOptions(body, textplain.Options{})
	require.NoError(t, err)
	assert.NotContains(t, result, "\n")
}

func TestOptionsMatchesConvert(t *testing.T) {
	viaConvert, err := textplain.Convert(optionsDoc, textplain.DefaultLineLength)
	require.NoError(t, err)

	viaOptions, err := textplain.ConvertWithOptions(optionsDoc, textplain.Options{LineLength: textplain.DefaultLineLength})
	require.NoError(t, err)

	assert.Equal(t, viaConvert, viaOptions)
}

func TestOptionsFootnotesOnlyWhenLinksExist(t *testing.T) {
	result, err := textplain.ConvertWithOptions("<p>no links here</p>", textplain.Options{Links: textplain.LinksFootnotes})
	require.NoError(t, err)
	assert.Equal(t, "no links here", result)
}
