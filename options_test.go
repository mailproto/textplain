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
		opts   []textplain.Option
		expect string
	}{
		{
			name:   "no options wraps at the default length",
			expect: "*****\nTitle\n*****\n\nSee one ( https://a.example ) and two ( https://b.example ).\n\n* x\n1. y",
		},
		{
			name:   "links omitted",
			opts:   []textplain.Option{textplain.WithLinks(textplain.LinksOmitted)},
			expect: "*****\nTitle\n*****\n\nSee one and two.\n\n* x\n1. y",
		},
		{
			name:   "links as footnotes",
			opts:   []textplain.Option{textplain.WithLinks(textplain.LinksFootnotes)},
			expect: "*****\nTitle\n*****\n\nSee one [1] and two [2].\n\n* x\n1. y\n\n[1] https://a.example\n[2] https://b.example",
		},
		{
			name:   "plain headings",
			opts:   []textplain.Option{textplain.WithPlainHeadings()},
			expect: "Title\n\nSee one ( https://a.example ) and two ( https://b.example ).\n\n* x\n1. y",
		},
		{
			name:   "custom item prefixes",
			opts:   []textplain.Option{textplain.WithBullet("- "), textplain.WithOrderedSuffix(") ")},
			expect: "*****\nTitle\n*****\n\nSee one ( https://a.example ) and two ( https://b.example ).\n\n- x\n1) y",
		},
		{
			name:   "later options win",
			opts:   []textplain.Option{textplain.WithBullet("- "), textplain.WithBullet("+ ")},
			expect: "*****\nTitle\n*****\n\nSee one ( https://a.example ) and two ( https://b.example ).\n\n+ x\n1. y",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := textplain.Convert(optionsDoc, tc.opts...)
			require.NoError(t, err)
			assert.Equal(t, tc.expect, result)
		})
	}
}

func TestOptionsWithoutWrapping(t *testing.T) {
	body := "<p>" + strings.Repeat("word ", 40) + "</p>"

	result, err := textplain.Convert(body, textplain.WithLineLength(0))
	require.NoError(t, err)
	assert.NotContains(t, result, "\n")
}

func TestOptionsDefaultToTheDefaultLineLength(t *testing.T) {
	body := "<p>" + strings.Repeat("word ", 40) + "</p>"

	withNone, err := textplain.Convert(body)
	require.NoError(t, err)

	explicit, err := textplain.Convert(body, textplain.WithLineLength(textplain.DefaultLineLength))
	require.NoError(t, err)

	assert.Equal(t, explicit, withNone)

	for _, line := range strings.Split(withNone, "\n") {
		assert.LessOrEqual(t, len(line), textplain.DefaultLineLength)
	}
}

func TestOptionsFootnotesOnlyWhenLinksExist(t *testing.T) {
	result, err := textplain.Convert("<p>no links here</p>", textplain.WithLinks(textplain.LinksFootnotes))
	require.NoError(t, err)
	assert.Equal(t, "no links here", result)
}
