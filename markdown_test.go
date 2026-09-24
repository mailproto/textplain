package textplain_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"
)

// Each case also renders its Markdown with goldmark, so that the expected
// output is checked against what a CommonMark reader makes of it.
func TestMarkdown(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		opts     []textplain.Option
		markdown string
		rendered string
	}{
		{
			name:     "headings",
			body:     `<h1>Title</h1><h3>Two<br>lines</h3><h2></h2><p>body</p>`,
			markdown: "# Title\n\n### Two lines\n\nbody",
			rendered: "<h1>Title</h1>\n<h3>Two lines</h3>\n<p>body</p>",
		},
		{
			name:     "inline markup",
			body:     `<p><b>bold</b>, <i> italic </i><em>em</em><strong></strong> and <code>x()</code></p>`,
			markdown: "**bold**, *italic* *em* and `x()`",
			rendered: "<p><strong>bold</strong>, <em>italic</em> <em>em</em> and <code>x()</code></p>",
		},
		{
			name:     "emphasis across blocks is left bare",
			body:     `<b><p>one</p><p>two</p></b>`,
			markdown: "one\n\ntwo",
			rendered: "<p>one</p>\n<p>two</p>",
		},
		{
			name:     "code spans with backticks",
			body:     "<p><code>a`b</code> <code>`x</code> <code> </code></p>",
			markdown: "``a`b`` `` `x ``",
			rendered: "<p><code>a`b</code> <code>`x</code></p>",
		},
		{
			name:     "preformatted",
			body:     "<pre>a *b*\n  ```c</pre>",
			markdown: "````\na *b*\n  ```c\n````",
			rendered: "<pre><code>a *b*\n  ```c\n</code></pre>",
		},
		{
			name:     "literal markup is escaped",
			body:     `<p>*not* _emphasis_ [x] ` + "`y`" + ` &lt;b&gt; a\b snake_case</p>`,
			markdown: `\*not\* \_emphasis\_ \[x] \` + "`y\\`" + ` \<b> a\\b snake_case`,
			rendered: "<p>*not* _emphasis_ [x] `y` &lt;b&gt; a\\b snake_case</p>",
		},
		{
			name:     "line starts are escaped only at the start of a line",
			body:     `<p># one - two</p><p>- a</p><p>&gt; b</p><p>1. c</p><p>2) d</p><p>+ e<br>= f</p>`,
			markdown: "\\# one - two\n\n\\- a\n\n\\> b\n\n1\\. c\n\n2\\) d\n\n\\+ e\\\n\\= f",
			rendered: "<p># one - two</p>\n<p>- a</p>\n<p>&gt; b</p>\n<p>1. c</p>\n<p>2) d</p>\n<p>+ e<br>\n= f</p>",
		},
		{
			name:     "line breaks",
			body:     `<p>one<br>two</p><p>a<br><br>b</p><p>end<br></p>`,
			markdown: "one\\\ntwo\n\na\n\nb\n\nend",
			rendered: "<p>one<br>\ntwo</p>\n<p>a</p>\n<p>b</p>\n<p>end</p>",
		},
		{
			name:     "table rows keep their lines",
			body:     `<table><tr><td>Jan</td><td>Feb</td></tr><tr><td>1</td><td>2</td></tr></table>`,
			markdown: "Jan Feb\\\n1 2",
			rendered: "<p>Jan Feb<br>\n1 2</p>",
		},
		{
			name:     "lists",
			body:     `<ul><li>top<ul><li>child</li></ul></li><li>- dash</li></ul><ol><li>one<ul><li>under</li></ul></li></ol>`,
			markdown: "* top\n  * child\n* \\- dash\n1. one\n    * under",
			rendered: "<ul>\n<li>top\n<ul>\n<li>child</li>\n</ul>\n</li>\n<li>- dash</li>\n</ul>\n<ol>\n<li>one\n<ul>\n<li>under</li>\n</ul>\n</li>\n</ol>",
		},
		{
			name:     "blockquote",
			body:     `<blockquote><p>- quoted</p><p>two</p></blockquote>`,
			markdown: "> \\- quoted\n>\n> two",
			rendered: "<blockquote>\n<p>- quoted</p>\n<p>two</p>\n</blockquote>",
		},
		{
			name:     "links",
			body:     `<a href="https://e.com/a b(c)">Click</a> <a href="https://e.com/?q=&lt;x&gt;">q</a> <a href="mailto:a@b.co">mail</a> <a href="#top">top</a> <a href="https://empty"></a> <a href="https://x"><img alt="Logo" src="l.png"></a> <a href="https://y"><img src="p.png"></a>`,
			markdown: "[Click](<https://e.com/a b(c)>) [q](https://e.com/?q=%3Cx%3E) [mail](mailto:a@b.co) top [![Logo](l.png)](https://x) [https://y](https://y)",
			rendered: `<p><a href="https://e.com/a%20b(c)">Click</a> <a href="https://e.com/?q=%3Cx%3E">q</a> <a href="mailto:a@b.co">mail</a> top <a href="https://x"><img src="l.png" alt="Logo"></a> <a href="https://y">https://y</a></p>`,
		},
		{
			name:     "links as footnotes",
			body:     `<p>See <a href="https://a">one</a> and <a href="https://b c">two</a></p>`,
			opts:     []textplain.Option{textplain.WithLinks(textplain.LinksFootnotes)},
			markdown: "See [one][1] and [two][2]\n\n[1]: https://a\n[2]: <https://b c>",
			rendered: `<p>See <a href="https://a">one</a> and <a href="https://b%20c">two</a></p>`,
		},
		{
			name:     "links omitted",
			body:     `<p>See <a href="https://a">one</a></p>`,
			opts:     []textplain.Option{textplain.WithLinks(textplain.LinksOmitted)},
			markdown: "See one",
			rendered: "<p>See one</p>",
		},
		{
			name:     "subscript does not open emphasis",
			body:     `<p>H<sub>2</sub>O and x_</p>`,
			markdown: `H\_2O and x\_`,
			rendered: "<p>H_2O and x_</p>",
		},
		{
			name:     "does not wrap",
			body:     "<p>" + strings.Repeat("word ", 30) + "</p><hr><p>after</p>",
			opts:     []textplain.Option{textplain.WithLineLength(20)},
			markdown: strings.TrimSpace(strings.Repeat("word ", 30)) + "\n\n" + strings.Repeat("-", textplain.DefaultLineLength) + "\n\nafter",
			rendered: "<p>" + strings.TrimSpace(strings.Repeat("word ", 30)) + "</p>\n<hr>\n<p>after</p>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := textplain.Convert(tc.body, append([]textplain.Option{textplain.WithMarkdown()}, tc.opts...)...)
			require.NoError(t, err)
			assert.Equal(t, tc.markdown, out)

			var rendered bytes.Buffer
			require.NoError(t, goldmark.Convert([]byte(out), &rendered))
			assert.Equal(t, tc.rendered, strings.TrimSpace(rendered.String()))
		})
	}
}
