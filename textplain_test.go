package textplain_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name   string
	body   string
	expect string
}

func TestConvert(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "html fragment",
			body:   "<p>Test</p>",
			expect: "Test",
		},
		testCase{
			name: "html with body element",
			body: `<html>
			<title>Ignore me</title>
			<body>
				<p>Test</p>
				</body>
			</html>`,
			expect: "Test",
		},
		testCase{
			name: "malformed body",
			body: `<html>
			<title>Ignore me</title>
			<body>
				<p>Test`,
			expect: "Test",
		},
		testCase{
			name:   "special characters",
			body:   "c&eacute;dille gar&#231;on &amp; &agrave; &ntilde;",
			expect: "cédille garçon & à ñ",
		},
	)
}

func TestStrippingWhitespace(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "leading tab, trailing newline",
			body:   "  \ttext\ntext\n",
			expect: "text\ntext",
		},
		testCase{
			name:   "leading newline, trailing tab, infix spaces",
			body:   "  \na \n a \t",
			expect: "a\na",
		},
		testCase{
			name:   "leading and infix newlines, trailing tab",
			body:   "  \na \n\t \n \n a \t",
			expect: "a\n\na",
		},
		testCase{
			name:   "trailing non-breaking space",
			body:   "test text&nbsp;",
			expect: "test text",
		},
		testCase{
			name:   "preheader block",
			body:   "test text &#8199;&#847; &#8199;&#847; &#8199;&#847; &shy; &shy; &shy;\n\nhello",
			expect: "test text\n\nhello",
		},
		testCase{
			name:   "zero width preheader padding",
			body:   "<p>Preview&zwnj;&#8203;&#65279;</p><p>Body</p>",
			expect: "Preview\n\nBody",
		},
		testCase{
			name:   "zero width joiner is kept for emoji",
			body:   "<p>&#128104;&#8205;&#128105;</p>",
			expect: "\U0001F468\u200d\U0001F469",
		},
		testCase{
			name:   "infix repeated space",
			body:   "test        text",
			expect: "test text",
		},
	)
}

func TestWrappingSpans(t *testing.T) {
	runTestCases(t,
		testCase{
			body: `<html>
	    <body>
			<p><span>Test</span>
			<span>line 2</span>
			</p>`,
			expect: `Test line 2`,
		},
		testCase{
			body: `<html>
	    <body>
			<p><span>Test</span>
			<span> spans </span>
			<p>between</p>
			<span>line 2</span>

			<span>
				again
			</span>
			</p>`,
			expect: "Test spans\n\nbetween\n\nline 2\nagain",
		},
		testCase{
			name: "tables and spans",
			body: `<table>
						<tbody>
							<tr>
								<td>
									<span>ID</span>
									<p>ABC-1234</p>
								</td>
								<td>
									<span>Date</span>
									<p>Mar 29, 2023</p>
								</td>
							</tr>
						</tbody>
					</table>`,
			expect: "ID\nABC-1234\n\nDate\nMar 29, 2023",
		},
	)
}

func TestLineBreaks(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "line feed and newline become newline",
			body:   "Test text\r\nTest text",
			expect: "Test text\nTest text",
		},
		testCase{
			name:   "line feed becomes newline",
			body:   "Test text\rTest text",
			expect: "Test text\nTest text",
		},
	)
}

func TestLists(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "list items without list wrapper",
			body:   "<li class='123'>item 1</li> <li>item 2</li>\n",
			expect: "* item 1\n* item 2",
		},
		testCase{
			name:   "list items with infix whitespace",
			body:   "<li>item 1</li> \t\n <li>item 2</li> <li> item 3</li>\n",
			expect: "* item 1\n* item 2\n* item 3",
		},
		testCase{
			name:   "list items with <ul>",
			body:   "<ul><li>item 1</li><li>item 2</li><li>item 3</li></ul>",
			expect: "* item 1\n* item 2\n* item 3",
		},
		testCase{
			name:   "list items with <ol>",
			body:   "<ol><li>item 1</li><li>item 2</li><li>item 3</li></ol>",
			expect: "1. item 1\n2. item 2\n3. item 3",
		},
		testCase{
			name:   "<ol> numbering skips non-item children",
			body:   "<ol>\n\n<li>item 1</li>\n\n<li>item 2</li>\n</ol>",
			expect: "1. item 1\n2. item 2",
		},
		testCase{
			name:   "nested <ol> numbers independently",
			body:   "<ol><li>one</li><li>two</li></ol><ol><li>fresh</li></ol>",
			expect: "1. one\n2. two\n1. fresh",
		},
		testCase{
			name:   "<ol start> seeds the numbering",
			body:   `<ol start="5"><li>a</li><li>b</li></ol>`,
			expect: "5. a\n6. b",
		},
		testCase{
			name:   "<ol start> may be negative",
			body:   `<ol start="-1"><li>a</li><li>b</li></ol>`,
			expect: "-1. a\n0. b",
		},
		testCase{
			name:   "unparseable <ol start> falls back to one",
			body:   `<ol start="abc"><li>a</li><li>b</li></ol>`,
			expect: "1. a\n2. b",
		},
		testCase{
			name:   "start on <ul> is ignored",
			body:   `<ul start="5"><li>a</li><li>b</li></ul>`,
			expect: "* a\n* b",
		},
		testCase{
			name:   "list items with <ul> and infix whitespace",
			body:   "<ul><li>item 1</li>  \t\n\t <li>item 2</li><li>item 3</li></ul>",
			expect: "* item 1\n* item 2\n* item 3",
		},
		testCase{
			name:   "list with leading whitespace",
			body:   "<p>hello</p>\n\n\n<ul><li>item 1</li><li>item 2</li><li>item 3</li></ul>",
			expect: "hello\n\n* item 1\n* item 2\n* item 3",
		},
		testCase{
			name:   "list with leading and trailing whitespace",
			body:   "<p>hello</p>\n\n\n<ul><li>item 1</li><li>item 2</li><li>item 3</li></ul>\n\n<p>hi</p>",
			expect: "hello\n\n* item 1\n* item 2\n* item 3\n\nhi",
		},
		testCase{
			name:   "paragraph after a list keeps its blank line",
			body:   "<ul><li>one</li></ul><br/><p>prose</p>",
			expect: "* one\n\nprose",
		},
		testCase{
			name:   "a later list does not close the gap after an earlier one",
			body:   "<ul><li>one</li></ul><br/><p>prose</p><ul><li>two</li></ul>",
			expect: "* one\n\nprose\n\n* two",
		},
	)
}

func TestStrippingHTML(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "strip html",
			body:   "<p class=\"123'45 , att\" att=tester>test <span class='te\"st'>text</span>\n",
			expect: "test text",
		},
		testCase{
			name: "strip ignored blocks",
			body: `<p>test</p>
			<!-- start text/html -->
			  <img src="logo.png" alt="logo">
			<!-- end text/html -->
			<p>text</p>`,
			expect: "test\n\ntext",
		},
	)
}

func TestParagraphsAndBreaks(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "paragraphs",
			body:   "<p>Test text</p><p>Test text</p>",
			expect: "Test text\n\nTest text",
		},
		testCase{
			name:   "paragraphs with whitespace",
			body:   "\n<p>Test text</p>\n\n\n\t<p>Test text</p>\n",
			expect: "Test text\n\nTest text",
		},
		testCase{
			name:   "paragraph with infix break",
			body:   "\n<p>Test text<br/>Test text</p>\n",
			expect: "Test text\nTest text",
		},
		testCase{
			name:   "paragraph with end break",
			body:   "\n<p>Test text<br> \tTest text<br></p>\n",
			expect: "Test text\nTest text",
		},
		testCase{
			name:   "full caps break",
			body:   "Test text<br><BR />Test text",
			expect: "Test text\n\nTest text",
		},
	)
}

func TestHeadings(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "h1",
			body:   "<h1>Test</h1>",
			expect: "****\nTest\n****",
		},
		testCase{
			name:   "h1 with whitespace",
			body:   "\t<h1>\nTest</h1>",
			expect: "****\nTest\n****",
		},
		testCase{
			name:   "multiline h1",
			body:   "\t<h1>\nTest line 1<br>Test 2</h1> ",
			expect: "***********\nTest line 1\nTest 2\n***********",
		},
		testCase{
			name:   "multiple h1 tags",
			body:   "<h1>Test</h1> <h1>Test</h1>",
			expect: "****\nTest\n****\n\n****\nTest\n****",
		},
		testCase{
			name:   "h2",
			body:   "<h2>Test</h2>",
			expect: "----\nTest\n----",
		},
		testCase{
			name:   "h3",
			body:   "<h3> <span class='a'>Test </span></h3>",
			expect: "Test\n----",
		},
	)
}

func TestAppliesLineWrapping(t *testing.T) {
	txt, err := textplain.Convert(strings.Repeat("test ", 100), textplain.WithLineLength(20))
	require.NoError(t, err)

	var offendingLines []int

	for i, line := range strings.Split(txt, "\n") {
		if len(line) > 20 {
			offendingLines = append(offendingLines, i)
		}
	}

	assert.Empty(t, offendingLines)
}

func TestWrappingLinesWithSpaces(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "no wrap",
			body:   "Long " + strings.Repeat(" ", textplain.DefaultLineLength) + "space doesn't wrap",
			expect: "Long space doesn't wrap",
		},
		testCase{
			name:   "wrap on proper line",
			body:   "Long " + strings.Repeat("A", textplain.DefaultLineLength) + " wraps",
			expect: "Long\n" + strings.Repeat("A", textplain.DefaultLineLength) + " wraps",
		},
	)
}

func TestWrappingDoesntBreakWords(t *testing.T) {
	runTestCases(t, testCase{
		body:   strings.Repeat("A", textplain.DefaultLineLength+1),
		expect: strings.Repeat("A", textplain.DefaultLineLength+1),
	})
}

func TestImgAltTags(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "img nested inside the link",
			body:   `<a href="http://example.com/"><span><img src="http://example.ru/hello.jpg"></span></a>`,
			expect: "( http://example.com/ )",
		},
		testCase{
			name:   "self-closed img tag with alt value",
			body:   `<a href="http://example.com/"><img src="http://example.ru/hello.jpg" alt="Example"/></a>`,
			expect: "Example ( http://example.com/ )",
		},
		testCase{
			name:   "open img tag with alt value",
			body:   `<a href="http://example.com/"><img src="http://example.ru/hello.jpg" alt="Example"></a>`,
			expect: "Example ( http://example.com/ )",
		},
		testCase{
			name:   "self-closed img tag single quoted with alt",
			body:   `<a href='http://example.com/'><img src='http://example.ru/hello.jpg' alt='Example'/></a>`,
			expect: "Example ( http://example.com/ )",
		},
		testCase{
			name:   "open img tag single quoted with alt",
			body:   `<a href='http://example.com/'><img src='http://example.ru/hello.jpg' alt='Example'></a>`,
			expect: "Example ( http://example.com/ )",
		},
	)
}

func TestLinks(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "simple link",
			body:   `<a href="http://example.com/">Link</a>`,
			expect: `Link ( http://example.com/ )`,
		},
		testCase{
			name:   "link with nested html",
			body:   `<a href="http://example.com/"><span class="a">Link</span></a>`,
			expect: `Link ( http://example.com/ )`,
		},
		testCase{
			name:   "nested html with a new line",
			body:   "<a href='http://example.com/'>\n\t<span class='a'>Link</span>\n\t</a>",
			expect: `Link ( http://example.com/ )`,
		},
		testCase{
			name:   "mailto link",
			body:   `<a href='mailto:contact@example.org'>Contact Us</a>`,
			expect: `Contact Us ( contact@example.org )`,
		},
		testCase{
			name:   "uppercase mailto link",
			body:   `<a href='MAILTO:contact@example.org'>contact@example.org</a>`,
			expect: `contact@example.org`,
		},
		testCase{
			name:   "text is the href without its trailing slash",
			body:   `<a href="https://example.com/">https://example.com</a>`,
			expect: `https://example.com/`,
		},
		testCase{
			name:   "text is the href without its scheme",
			body:   `<a href="http://example.com">example.com</a>`,
			expect: `http://example.com`,
		},
		testCase{
			name:   "text differs from the href by more than the scheme",
			body:   `<a href="http://example.com/a">example.com</a>`,
			expect: `example.com ( http://example.com/a )`,
		},
		testCase{
			name:   "complicated link",
			body:   `<a href="http://example.com:80/~user?aaa=bb&amp;c=d,e,f#foo">Link</a>`,
			expect: `Link ( http://example.com:80/~user?aaa=bb&c=d,e,f#foo )`,
		},
		testCase{
			name:   "link with attribute",
			body:   `<a title='title' href="http://example.com/">Link</a>`,
			expect: `Link ( http://example.com/ )`,
		},
		testCase{
			name:   "href attribute spacing",
			body:   `<a href="   http://example.com/ "> Link </a>`,
			expect: `Link ( http://example.com/ )`,
		},
		testCase{
			name:   "multiple links",
			body:   `<a href="http://example.com/a/">Link A</a> <a href="http://example.com/b/">Link B</a>`,
			expect: `Link A ( http://example.com/a/ ) Link B ( http://example.com/b/ )`,
		},
		testCase{
			name:   "link containing merge tag",
			body:   `<a href="%%LINK%%">Link</a>`,
			expect: `Link ( %%LINK%% )`,
		},
		testCase{
			name:   "link in square brackets",
			body:   `<a href="[LINK]">Link</a>`,
			expect: `Link ( [LINK] )`,
		},
		testCase{
			name:   "link in curly braces",
			body:   `<a href="{LINK}">Link</a>`,
			expect: `Link ( {LINK} )`,
		},
		testCase{
			name:   "unsubscribe",
			body:   `<a href="[[!unsubscribe]]">Link</a>`,
			expect: `Link ( [[!unsubscribe]] )`,
		},
		testCase{
			name:   "empty link gets dropped, and shouldn`t run forever",
			body:   "<a href=\"test\"></a>" + strings.Repeat("\n<p>This is some more text</p>", 15),
			expect: strings.Repeat("This is some more text\n\n", 14) + "This is some more text",
		},
		testCase{
			name:   "links that go outside of line should wrap nicely",
			body:   "Long text before the actual link and then LINK TEXT \n( http://www.long.link ) and then more text that does not wrap",
			expect: "Long text before the actual link and then LINK TEXT\n( http://www.long.link ) and then more text that does not wrap",
		},
		testCase{
			name:   "same text and link",
			body:   `<a href="http://example.com">http://example.com</a>`,
			expect: `http://example.com`,
		},
		testCase{
			name: "long links stay on a single line",
			body: `<a href="http://example.com/` + strings.Repeat("A", textplain.DefaultLineLength) + `">Hello</a>`,
			expect: `Hello
( http://example.com/` + strings.Repeat("A", textplain.DefaultLineLength) + ` )`,
		},
		testCase{
			name: "long non-http links stay on a single line",
			body: `<a href="gopher://example.com/` + strings.Repeat("A", textplain.DefaultLineLength) + `">Hello</a>`,
			expect: `Hello
( gopher://example.com/` + strings.Repeat("A", textplain.DefaultLineLength) + ` )`,
		},
		testCase{
			name:   "link wrapping image",
			body:   `<a href="http://example.com"><img src="https://images.com/image.png" /></a>`,
			expect: `( http://example.com )`,
		},
		testCase{
			name:   "empty link falls back to alt",
			body:   `<a href="http://example.com" alt="Example"></a>`,
			expect: `Example ( http://example.com )`,
		},
		testCase{
			name:   "link alt labels an image with no alt of its own",
			body:   `<a href="http://example.com" alt="Example"><img src="https://images.com/image.png" /></a>`,
			expect: `Example ( http://example.com )`,
		},
		testCase{
			name:   "image alt takes precedence over link alt",
			body:   `<a href="http://example.com" alt="Link"><img alt="Image" src="https://images.com/image.png" /></a>`,
			expect: `Image ( http://example.com )`,
		},
		testCase{
			name:   "alt matching href is not repeated",
			body:   `<a href="http://example.com" alt="http://example.com"></a>`,
			expect: `http://example.com`,
		},
		testCase{
			name:   "fragment only link keeps just its text",
			body:   `<a href="#section">Jump</a>`,
			expect: `Jump`,
		},
		testCase{
			name:   "bare fragment link keeps just its text",
			body:   `<a href="#">Top</a>`,
			expect: `Top`,
		},
		testCase{
			name:   "a fragment on a real url is still a link",
			body:   `<a href="http://example.com/page#frag">Deep</a>`,
			expect: `Deep ( http://example.com/page#frag )`,
		},
	)
}

// see https://github.com/premailer/premailer/issues/72
func TestMultipleLinksPerLine(t *testing.T) {
	plain, err := textplain.Convert(`<p>This is <a href="http://www.google.com" >link1</a> and <a href="http://www.google.com" >link2 </a> is next.</p>`, textplain.WithLineLength(10000))
	require.NoError(t, err)

	assert.Equal(t, `This is link1 ( http://www.google.com ) and link2 ( http://www.google.com ) is next.`, plain)
}

// see https://github.com/premailer/premailer/issues/72
func TestLinksWithinHeadings(t *testing.T) {
	runTestCases(t,
		testCase{
			body:   "<h1><a href='http://example.com/'>Test</a></h1>",
			expect: "****************************\nTest ( http://example.com/ )\n****************************",
		},
	)
}

func TestStripsNonContentTags(t *testing.T) {
	runTestCases(t, testCase{
		body: `<html>
			<body>
				<script type="text/javascript">
					alert("haxx");
				</script>
				No hacks here
				<style>
					p {
						font-weight: bold;
					}
				</style>
				<p>
					This is not a bold statement
				</p>
			</body>
		</html>`,
		expect: "No hacks here\n\nThis is not a bold statement",
	})
}

func TestMultilineTitles(t *testing.T) {
	runTestCases(t, testCase{
		body: `<h1>Horse
		Friends
						Yeah
</h1>`,
		expect: "*******\nHorse\nFriends\nYeah\n*******",
	})
}

func TestWrappingDoesntAddUnnecessaryLineBreaks(t *testing.T) {
	runTestCases(t, testCase{
		body: `.stylesheet {
			color: white;
			background-image: url('data:image/png;base64,` + strings.Repeat("A", textplain.DefaultLineLength) + `');
			font-weight: bold;
			margin: 0px;
		}`,
		expect: `.stylesheet {
color: white;
background-image:
url('data:image/png;base64,` + strings.Repeat("A", textplain.DefaultLineLength) + `');
font-weight: bold;
margin: 0px;
}`,
	})
}

func TestStrippingComments(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "comment in tag",
			body:   "<p>in<!--comment 1-->between</p>",
			expect: "inbetween",
		},
		testCase{
			name:   "comment between tags",
			body:   `<p>before</p><!--comment 1--><p>after</p>`,
			expect: "before\n\nafter",
		},
		testCase{
			name:   "commented out tag",
			body:   `<p>before</p><!--comment 1<div>random</div>--><p>after</p>`,
			expect: "before\n\nafter",
		},
		testCase{
			name: "multiline comment",
			body: `<p>before</p>
			<!--
				replacing unordered list with an ordered one.
				uncomment out incase it breaks stuff.

				<h2>An Unordered HTML List</h2>
				<ul>
				  <li>Coffee</li>
				  <li>Tea</li>
				  <li>Milk</li>
				</ul>
			-->
			<p>after</p>`,
			expect: "before\n\nafter",
		},
		testCase{
			name: "comment within comment",
			body: `<p>before</p>
			<!--
				some reason to comment out the whole block

				<h2>An Unordered HTML List</h2>
				<ul>
				  <li>Coffee</li>
				  <li>Tea</li>
				  <li>Milk</li>
				</ul>

				<!-- awesome -->
				<p>sweet list</p>
			-->
			<p>after</p>`,
			expect: "before\n\nsweet list\n\n-->\nafter",
		},
	)
}

func TestFixSpacing(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "ends in *",
			body:   "<p>hello</p>*",
			expect: "hello\n\n*",
		},
		testCase{
			name:   "paragraphs starting with * keep their break",
			body:   "<p>* one</p><p>* two</p>",
			expect: "* one\n\n* two",
		},
	)
}

func TestConvertReader(t *testing.T) {
	t.Run("matches Convert", func(t *testing.T) {
		t.Parallel()
		result, err := textplain.ConvertReader(strings.NewReader("<p>Test</p>"), textplain.WithLineLength(0))
		require.NoError(t, err)
		assert.Equal(t, "Test", result)
	})

	t.Run("read errors are returned", func(t *testing.T) {
		t.Parallel()
		readErr := errors.New("connection reset")
		_, err := textplain.ConvertReader(iotest.ErrReader(readErr))
		require.ErrorIs(t, err, readErr)
	})
}

func TestMissingBody(t *testing.T) {
	t.Run("frameset document has no body", func(t *testing.T) {
		t.Parallel()
		_, err := textplain.Convert(`<frameset><frame src="a.html"/></frameset>`)
		require.ErrorIs(t, err, textplain.ErrBodyNotFound)
	})

	t.Run("empty body is not an error", func(t *testing.T) {
		t.Parallel()
		result, err := textplain.Convert(`<html><body></body></html>`)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestTables(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "cells are separated and rows break",
			body:   `<table><tr><td>Jan</td><td>Feb</td></tr><tr><td>1</td><td>2</td></tr></table>`,
			expect: "Jan Feb\n1 2",
		},
		testCase{
			name:   "header cells behave like data cells",
			body:   `<table><thead><tr><th>Month</th><th>Total</th></tr></thead><tbody><tr><td>Jan</td><td>$5</td></tr></tbody></table>`,
			expect: "Month Total\nJan $5",
		},
		testCase{
			name:   "layout cells holding blocks are unaffected",
			body:   `<table><tr><td><p>Layout cell</p></td></tr><tr><td><p>Second row</p></td></tr></table>`,
			expect: "Layout cell\n\nSecond row",
		},
		testCase{
			name:   "link in a cell is separated from the next cell",
			body:   `<table><tr><td><a href="http://example.com">Link</a></td><td>next</td></tr></table>`,
			expect: "Link ( http://example.com ) next",
		},
		testCase{
			name:   "empty cells add no separator of their own",
			body:   `<table><tr><td></td><td>only</td></tr></table>`,
			expect: "only",
		},
	)
}

func TestBlockquote(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "quoted text is marked",
			body:   `<blockquote>quoted</blockquote><p>after</p>`,
			expect: "> quoted\n\nafter",
		},
		testCase{
			name:   "blank lines inside a quote stay quoted",
			body:   `<p>before</p><blockquote><p>para one</p><p>para two</p></blockquote><p>after</p>`,
			expect: "before\n\n> para one\n>\n> para two\n\nafter",
		},
		testCase{
			name:   "nesting deepens the marker",
			body:   `<blockquote><blockquote>inner</blockquote>outer</blockquote>`,
			expect: "> > inner\n>\n> outer",
		},
		testCase{
			name:   "markup inside a quote still converts",
			body:   `<blockquote><ul><li>a</li><li>b</li></ul></blockquote>`,
			expect: "> * a\n> * b",
		},
	)

	t.Run("every wrapped line keeps the marker", func(t *testing.T) {
		t.Parallel()
		result, err := textplain.Convert("<blockquote><p>"+strings.Repeat("quoted words ", 6)+"</p></blockquote>", textplain.WithLineLength(30))
		require.NoError(t, err)

		for _, line := range strings.Split(result, "\n") {
			assert.True(t, strings.HasPrefix(line, ">"), "unmarked line %q", line)
		}
	})
}

func TestHorizontalRule(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "rule between paragraphs",
			body:   `<p>a</p><hr/><p>b</p>`,
			expect: "a\n\n" + strings.Repeat("-", textplain.DefaultLineLength) + "\n\nb",
		},
		testCase{
			name:   "consecutive rules are kept",
			body:   `<p>a</p><hr/><hr/><p>b</p>`,
			expect: "a\n\n" + strings.Repeat("-", textplain.DefaultLineLength) + "\n\n" + strings.Repeat("-", textplain.DefaultLineLength) + "\n\nb",
		},
	)

	t.Run("rule matches the requested line length", func(t *testing.T) {
		t.Parallel()
		result, err := textplain.Convert(`<p>a</p><hr/><p>b</p>`, textplain.WithLineLength(20))
		require.NoError(t, err)
		assert.Equal(t, "a\n\n"+strings.Repeat("-", 20)+"\n\nb", result)
	})

	t.Run("unwrapped output falls back to the default width", func(t *testing.T) {
		t.Parallel()
		result, err := textplain.Convert(`<hr/>`, textplain.WithLineLength(0))
		require.NoError(t, err)
		assert.Equal(t, strings.Repeat("-", textplain.DefaultLineLength), result)
	})
}

func TestSubAndSuperscript(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "subscript",
			body:   `<p>H<sub>2</sub>O</p>`,
			expect: "H_2O",
		},
		testCase{
			name:   "superscript",
			body:   `<p>E = mc<sup>2</sup></p>`,
			expect: "E = mc^2",
		},
		testCase{
			name:   "several superscripts",
			body:   `<p>x<sup>2</sup> + y<sup>2</sup></p>`,
			expect: "x^2 + y^2",
		},
		testCase{
			name:   "grouped when spaced",
			body:   `<p>2<sup>n + 1</sup></p>`,
			expect: "2^(n + 1)",
		},
		testCase{
			name:   "symbols left unmarked",
			body:   `<p>Acme<sup>&reg;</sup> Widget<sup>&dagger;</sup></p>`,
			expect: "Acme® Widget†",
		},
		testCase{
			name:   "empty",
			body:   `<p>a<sup> </sup>b</p>`,
			expect: "ab",
		},
	)
}

func TestAbbreviations(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "abbr expands its title",
			body:   `<p><abbr title="HyperText Markup Language">HTML</abbr> mail</p>`,
			expect: "HTML (HyperText Markup Language) mail",
		},
		testCase{
			name:   "acronym expands its title",
			body:   `<p><acronym title="As Soon As Possible">ASAP</acronym></p>`,
			expect: "ASAP (As Soon As Possible)",
		},
		testCase{
			name:   "no title",
			body:   `<p><abbr>HTML</abbr></p>`,
			expect: "HTML",
		},
		testCase{
			name:   "title repeating the text",
			body:   `<p><abbr title="html">HTML</abbr></p>`,
			expect: "HTML",
		},
	)
}

func TestBlockElements(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "sibling articles",
			body:   `<article>one</article><article>two</article>`,
			expect: "one\n\ntwo",
		},
		testCase{
			name:   "page landmarks",
			body:   `<header>Head</header><main><p>Body</p></main><footer>Foot</footer>`,
			expect: "Head\n\nBody\n\nFoot",
		},
		testCase{
			name:   "figure and caption",
			body:   `<figure><img alt="chart" src="c.png"/><figcaption>Fig 1</figcaption></figure>`,
			expect: "chart\nFig 1",
		},
		testCase{
			name:   "table caption",
			body:   `<table><caption>Sales</caption><tr><td>a</td></tr></table>`,
			expect: "Sales\n\na",
		},
		testCase{
			name:   "center",
			body:   `<center>a</center><center>b</center>`,
			expect: "a\n\nb",
		},
		testCase{
			name:   "output stays inline",
			body:   `<p>total <output>5</output> items</p>`,
			expect: "total 5 items",
		},
	)
}

func TestDefinitionLists(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "terms and definitions each get a line",
			body:   `<dl><dt>Term</dt><dd>Definition</dd><dt>T2</dt><dd>D2</dd></dl>`,
			expect: "Term\nDefinition\nT2\nD2",
		},
		testCase{
			name:   "a term may have several definitions",
			body:   `<dl><dt>Term</dt><dd>One</dd><dd>Two</dd></dl>`,
			expect: "Term\nOne\nTwo",
		},
		testCase{
			name:   "definitions keep their own markup",
			body:   `<dl><dt>Term</dt><dd><a href="http://example.com">link</a></dd></dl>`,
			expect: "Term\nlink ( http://example.com )",
		},
	)
}

func TestHiddenContent(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "display none",
			body:   `<p>shown</p><p style="display:none">hidden preheader</p>`,
			expect: "shown",
		},
		testCase{
			name:   "a trailing semicolon is not a declaration",
			body:   `<p style="color:red;">shown</p>`,
			expect: "shown",
		},
		testCase{
			name:   "declarations are matched regardless of case or spacing",
			body:   `<p>shown</p><p style="color:red; DISPLAY : NONE ;margin:0">hidden</p>`,
			expect: "shown",
		},
		testCase{
			name:   "visibility hidden",
			body:   `<p>shown</p><p style="visibility:hidden">hidden</p>`,
			expect: "shown",
		},
		testCase{
			name:   "zero opacity",
			body:   `<p>shown</p><p style="opacity:0">hidden</p>`,
			expect: "shown",
		},
		testCase{
			name:   "zero font size with a unit",
			body:   `<p>shown</p><p style="font-size:0px">hidden</p>`,
			expect: "shown",
		},
		testCase{
			name:   "zero max height",
			body:   `<p>shown</p><p style="max-height:0">hidden</p>`,
			expect: "shown",
		},
		testCase{
			name:   "hidden attribute",
			body:   `<p>shown</p><div hidden>hidden</div>`,
			expect: "shown",
		},
		testCase{
			name:   "aria-hidden true",
			body:   `<p>shown</p><p aria-hidden="true">hidden</p>`,
			expect: "shown",
		},
		testCase{
			name:   "the whole subtree goes",
			body:   `<p>shown</p><div style="display:none"><p>nested</p><a href="http://example.com">link</a></div>`,
			expect: "shown",
		},
		testCase{
			name:   "hidden span in a run of spans",
			body:   `<p>a</p><span>visible</span><span style="display:none">hidden</span>`,
			expect: "a\n\nvisible",
		},
		testCase{
			name:   "hidden list item does not take a number",
			body:   `<ol><li>a</li><li style="display:none">hidden</li><li>b</li></ol>`,
			expect: "1. a\n2. b",
		},
		testCase{
			name:   "hidden table row",
			body:   `<table><tr><td>cell</td></tr><tr style="display:none"><td>hidden row</td></tr></table>`,
			expect: "cell",
		},
		// values near zero are not zero
		testCase{
			name:   "fractional opacity stays visible",
			body:   `<p>shown</p><p style="opacity:0.5">still visible</p>`,
			expect: "shown\n\nstill visible",
		},
		testCase{
			name:   "fractional font size stays visible",
			body:   `<p>shown</p><p style="font-size:0.9em">still visible</p>`,
			expect: "shown\n\nstill visible",
		},
		testCase{
			name:   "aria-hidden false stays visible",
			body:   `<p>shown</p><p aria-hidden="false">still visible</p>`,
			expect: "shown\n\nstill visible",
		},
	)
}

func TestNonContentElements(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "select options are not document text",
			body:   `<p>a</p><select><option>opt1</option><option>opt2</option></select>`,
			expect: "a",
		},
		testCase{
			name:   "textarea holds a value, not text",
			body:   `<p>a</p><textarea>editable</textarea>`,
			expect: "a",
		},
		testCase{
			name:   "iframe fallback is ignored",
			body:   `<p>a</p><iframe src="x.html">fallback</iframe>`,
			expect: "a",
		},
		testCase{
			name:   "svg title is metadata",
			body:   `<p>a</p><svg><title>icon</title></svg>`,
			expect: "a",
		},
		testCase{
			name:   "media fallback is ignored",
			body:   `<p>a</p><video src="v.mp4">no video support</video>`,
			expect: "a",
		},
		testCase{
			name:   "template contents are inert",
			body:   `<p>a</p><template><p>inert</p></template>`,
			expect: "a",
		},
		testCase{
			name:   "noscript is kept, since scripts never run here",
			body:   `<p>a</p><noscript>shown when scripts are off</noscript>`,
			expect: "a\n\nshown when scripts are off",
		},
		testCase{
			name:   "button labels are visible text",
			body:   `<p>a</p><button>Confirm your email</button>`,
			expect: "a\n\nConfirm your email",
		},
	)
}

func TestNestedLists(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "nested list breaks from the item text and indents",
			body:   `<ul><li>top<ul><li>child</li><li>child2</li></ul></li><li>top2</li></ul>`,
			expect: "* top\n  * child\n  * child2\n* top2",
		},
		testCase{
			name:   "each level numbers from its own start",
			body:   `<ol><li>one<ol><li>inner</li></ol></li><li>two</li></ol>`,
			expect: "1. one\n  1. inner\n2. two",
		},
		testCase{
			name:   "indentation deepens with each level",
			body:   `<ul><li>a<ul><li>b<ul><li>c</li></ul></li></ul></li></ul>`,
			expect: "* a\n  * b\n    * c",
		},
		testCase{
			name:   "list types may alternate",
			body:   `<ul><li>mixed<ol><li>num</li></ol></li></ul>`,
			expect: "* mixed\n  1. num",
		},
		testCase{
			name:   "a nested list reached through a div still indents",
			body:   `<ul><li>item<div><ul><li>via div</li></ul></div></li></ul>`,
			expect: "* item\n  * via div",
		},
		testCase{
			name:   "sibling lists are not indented under each other",
			body:   `<ol><li>one</li></ol><ol><li>fresh</li></ol>`,
			expect: "1. one\n1. fresh",
		},
		testCase{
			name:   "indentation survives inside a quote",
			body:   `<blockquote><ul><li>top<ul><li>child</li></ul></li></ul></blockquote><p>after</p>`,
			expect: "> * top\n>   * child\n\nafter",
		},
	)
}

func TestManyPreformattedBlocks(t *testing.T) {
	var body, expect []string
	for i := range 50 {
		n := strconv.Itoa(i)
		body = append(body, "<pre>block "+n+"\n  indented</pre>")
		expect = append(expect, "block "+n+"\n  indented")
	}

	result, err := textplain.Convert(strings.Join(body, ""))
	require.NoError(t, err)
	assert.Equal(t, strings.Join(expect, "\n\n"), result)
}

func TestMarkersInContent(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "entities cannot open a quote",
			body:   `<p>see &#x01;quote&#x02; here</p>`,
			expect: "see quote here",
		},
		testCase{
			name:   "raw control characters are dropped",
			body:   "<p>a\x00b\x01c\x02d\x03e\x04f\x05g\x06h</p>",
			expect: "abcdefgh",
		},
		testCase{
			name:   "a placeholder in text does not take a preformatted block",
			body:   "<p>x&#6;y</p><pre>code</pre>",
			expect: "xy\n\ncode",
		},
		testCase{
			name:   "attributes are cleaned too",
			body:   `<img alt="a&#3;b"/> <a href="http://e.com/&#1;x">link</a>`,
			expect: "ab link ( http://e.com/x )",
		},
	)
}

func TestPreformatted(t *testing.T) {
	runTestCases(t,
		testCase{
			name:   "indentation and tabs survive",
			body:   "<pre>line1\n  indented\n\ttabbed</pre>",
			expect: "line1\n  indented\n\ttabbed",
		},
		testCase{
			name:   "surrounding text is unaffected",
			body:   "<p>before</p><pre>a\n  b</pre><p>after</p>",
			expect: "before\n\na\n  b\n\nafter",
		},
		testCase{
			name:   "each block is restored in order",
			body:   "<pre>one</pre><pre>  two</pre>",
			expect: "one\n\n  two",
		},
		testCase{
			name:   "line breaks inside are kept",
			body:   "<pre>a<br>b</pre>",
			expect: "a\nb",
		},
		testCase{
			name:   "markup inside is taken as text",
			body:   "<pre><code>func main() {\n\tfmt.Println(1)\n}</code></pre>",
			expect: "func main() {\n\tfmt.Println(1)\n}",
		},
	)

	runTestCases(t,
		testCase{
			name:   "preformatted text inside a quote is marked on every line",
			body:   "<blockquote><pre>quoted code\n  indented</pre></blockquote><p>after</p>",
			expect: "> quoted code\n>   indented\n\nafter",
		},
		testCase{
			name:   "a rule and a block do not tread on each other",
			body:   "<p>a</p><hr/><pre>code</pre><p>b</p>",
			expect: "a\n\n" + strings.Repeat("-", textplain.DefaultLineLength) + "\n\ncode\n\nb",
		},
	)

	t.Run("preformatted text is not wrapped", func(t *testing.T) {
		t.Parallel()
		line := "aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd"
		result, err := textplain.Convert("<pre>"+line+"</pre>", textplain.WithLineLength(20))
		require.NoError(t, err)
		assert.Equal(t, line, result)
	})
}
