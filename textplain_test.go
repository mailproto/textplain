package textplain_test

import (
	"strings"
	"testing"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name       string
	body       string
	expect     string
	skipRegexp bool
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
			name:       "preheader block",
			body:       "test text &#8199;&#847; &#8199;&#847; &#8199;&#847; &shy; &shy; &shy;\n\nhello",
			expect:     "test text\n\nhello",
			skipRegexp: true,
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
			expect: "* item 1\n* item 2\n* item 3",
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
	txt, err := textplain.Convert(strings.Repeat("test ", 100), 20)
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
	)
}

// see https://github.com/premailer/premailer/issues/72
func TestMultipleLinksPerLine(t *testing.T) {
	plain, err := textplain.Convert(`<p>This is <a href="http://www.google.com" >link1</a> and <a href="http://www.google.com" >link2 </a> is next.</p>`, 10000)
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
	runTestCases(t, testCase{
		name:   "ends in *",
		body:   "<p>hello</p>*",
		expect: "hello\n\n*",
	})
}
