package textplain_test

import (
	"testing"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseURL(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		base   string
		expect string
	}{
		{
			name:   "root relative target",
			body:   `<a href="/signup">Sign up</a>`,
			base:   "https://example.com",
			expect: "Sign up ( https://example.com/signup )",
		},
		{
			name:   "relative to a directory base",
			body:   `<a href="page.html">Rel</a>`,
			base:   "https://example.com/dir/",
			expect: "Rel ( https://example.com/dir/page.html )",
		},
		{
			name:   "no base leaves the target alone",
			body:   `<a href="/signup">Sign up</a>`,
			expect: "Sign up ( /signup )",
		},
		{
			name:   "absolute targets are untouched",
			body:   `<a href="https://other.example/x">Abs</a>`,
			base:   "https://example.com",
			expect: "Abs ( https://other.example/x )",
		},
		{
			name:   "mailto is untouched",
			body:   `<a href="mailto:a@b.co">Mail</a>`,
			base:   "https://example.com",
			expect: "Mail ( a@b.co )",
		},
		{
			name:   "fragment only links never reach resolution",
			body:   `<a href="#frag">F</a>`,
			base:   "https://example.com",
			expect: "F",
		},
		{
			name:   "a base element wins over the option",
			body:   `<head><base href="https://from-doc.example/"/></head><body><a href="/x">D</a></body>`,
			base:   "https://example.com",
			expect: "D ( https://from-doc.example/x )",
		},
		{
			name:   "a relative base element resolves against the option",
			body:   `<head><base href="/sub/"/></head><body><a href="x">D</a></body>`,
			base:   "https://example.com",
			expect: "D ( https://example.com/sub/x )",
		},
		{
			name:   "a relative base with no option is not usable",
			body:   `<head><base href="/sub/"/></head><body><a href="x">D</a></body>`,
			expect: "D ( x )",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := textplain.ConvertWithOptions(tc.body, textplain.Options{LineLength: 200, BaseURL: tc.base})
			require.NoError(t, err)
			assert.Equal(t, tc.expect, result)
		})
	}
}
