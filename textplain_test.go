package textplain_test

import (
	"strings"
	"testing"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	name   string
	body   string
	expect string
}

func TestConvert(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "html fragment",
			body:   "<p>Test</p>",
			expect: "Test",
		},
		{
			name: "html with body element",
			body: `<html>
			<title>Ignore me</title>
			<body>
				<p>Test</p>
				</body>
			</html>`,
			expect: "Test",
		},
		{
			name: "malformed body",
			body: `<html>
			<title>Ignore me</title>
			<body>
				<p>Test`,
			expect: "Test",
		},
		{
			name:   "special characters",
			body:   "c&eacute;dille gar&#231;on &amp; &agrave; &ntilde;",
			expect: "cédille garçon & à ñ",
		},
	})
}

func TestStrippingWhitespace(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "leading tab, trailing newline",
			body:   "  \ttext\ntext\n",
			expect: "text\ntext",
		},
		{
			name:   "leading newline, trailing tab, infix spaces",
			body:   "  \na \n a \t",
			expect: "a\na",
		},
		{
			name:   "leading and infix newlines, trailing tab",
			body:   "  \na \n\t \n \n a \t",
			expect: "a\n\na",
		},
		{
			name:   "trailing non-breaking space",
			body:   "test text&nbsp;",
			expect: "test text",
		},
		{
			name:   "infix repeated space",
			body:   "test        text",
			expect: "test text",
		},
	})

}

func TestWrappingSpans(t *testing.T) {
	runTestCases(t, []testCase{
		{
			body: `<html>
	    <body>
			<p><span>Test</span>
			<span>line 2</span>
			</p>`,
			expect: `Test line 2`,
		},
		{
			body: `<html>
	    <body>
			<p><span>Test</span>
			<span> spans </span>
			<p>inbetween</p>
			<span>line 2</span>

			<span>
				again
			</span>
			</p>`,
			expect: "Test spans\n\ninbetween\n\nline 2\nagain",
		},
	})
}

func TestLineBreaks(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "line feed and newline become newline",
			body:   "Test text\r\nTest text",
			expect: "Test text\nTest text",
		},
		{
			name:   "line feed becomes newline",
			body:   "Test text\rTest text",
			expect: "Test text\nTest text",
		},
	})
}

func TestLists(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "list items without list wrapper",
			body:   "<li class='123'>item 1</li> <li>item 2</li>\n",
			expect: "* item 1\n* item 2",
		},
		{
			name:   "list items with infix whitespace",
			body:   "<li>item 1</li> \t\n <li>item 2</li> <li> item 3</li>\n",
			expect: "* item 1\n* item 2\n* item 3",
		},
		{
			name:   "list items with <ul>",
			body:   "<ul><li>item 1</li><li>item 2</li><li>item 3</li></ul>",
			expect: "* item 1\n* item 2\n* item 3",
		},
		{
			name:   "list items with <ol>",
			body:   "<ol><li>item 1</li><li>item 2</li><li>item 3</li></ol>",
			expect: "* item 1\n* item 2\n* item 3",
		},
		{
			name:   "list items with <ul> and infix whitespace",
			body:   "<ul><li>item 1</li>  \t\n\t <li>item 2</li><li>item 3</li></ul>",
			expect: "* item 1\n* item 2\n* item 3",
		},
		{
			name:   "list with leading whitespace",
			body:   "<p>hello</p>\n\n\n<ul><li>item 1</li><li>item 2</li><li>item 3</li></ul>",
			expect: "hello\n\n* item 1\n* item 2\n* item 3",
		},
	})
}

func TestStrippingHTML(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "strip html",
			body:   "<p class=\"123'45 , att\" att=tester>test <span class='te\"st'>text</span>\n",
			expect: "test text",
		},
		{
			name: "strip ignored blocks",
			body: `<p>test</p>
			<!-- start text/html -->
			  <img src="logo.png" alt="logo">
			<!-- end text/html -->
			<p>text</p>`,
			expect: "test\n\ntext",
		},
	})
}

func TestParagraphsAndBreaks(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "paragraphs",
			body:   "<p>Test text</p><p>Test text</p>",
			expect: "Test text\n\nTest text",
		},
		{
			name:   "paragraphs with whitespace",
			body:   "\n<p>Test text</p>\n\n\n\t<p>Test text</p>\n",
			expect: "Test text\n\nTest text",
		},
		{
			name:   "paragraph with infix break",
			body:   "\n<p>Test text<br/>Test text</p>\n",
			expect: "Test text\nTest text",
		},
		{
			name:   "paragraph with end break",
			body:   "\n<p>Test text<br> \tTest text<br></p>\n",
			expect: "Test text\nTest text",
		},
		{
			name:   "full caps break",
			body:   "Test text<br><BR />Test text",
			expect: "Test text\n\nTest text",
		},
	})
}

func TestHeadings(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "h1",
			body:   "<h1>Test</h1>",
			expect: "****\nTest\n****",
		},
		{
			name:   "h1 with whitespace",
			body:   "\t<h1>\nTest</h1>",
			expect: "****\nTest\n****",
		},
		{
			name:   "multiline h1",
			body:   "\t<h1>\nTest line 1<br>Test 2</h1> ",
			expect: "***********\nTest line 1\nTest 2\n***********",
		},
		{
			name:   "multiple h1 tags",
			body:   "<h1>Test</h1> <h1>Test</h1>",
			expect: "****\nTest\n****\n\n****\nTest\n****",
		},
		{
			name:   "h2",
			body:   "<h2>Test</h2>",
			expect: "----\nTest\n----",
		},
		{
			name:   "h3",
			body:   "<h3> <span class='a'>Test </span></h3>",
			expect: "Test\n----",
		},
	})
}

func TestAppliesLineWrapping(t *testing.T) {
	txt, err := textplain.Convert(strings.Repeat("test ", 100), 20)
	if err != nil {
		t.Error(err)
	}

	var offendingLines []int
	for i, line := range strings.Split(txt, "\n") {
		if len(line) > 20 {
			offendingLines = append(offendingLines, i)
		}
	}

	if len(offendingLines) > 0 {
		t.Errorf("Found lines longer than 20 chars: %v", offendingLines)
	}
}

func TestWrappingLinesWithSpaces(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "no wrap",
			body:   "Long " + strings.Repeat(" ", textplain.DefaultLineLength) + "space doesn't wrap",
			expect: "Long space doesn't wrap",
		},
		{
			name:   "wrap on proper line",
			body:   "Long " + strings.Repeat("A", textplain.DefaultLineLength) + " wraps",
			expect: "Long\n" + strings.Repeat("A", textplain.DefaultLineLength) + " wraps",
		},
	})
}

func TestWrappingDoesntBreakWords(t *testing.T) {
	runTestCase(t, testCase{
		body:   strings.Repeat("A", textplain.DefaultLineLength+1),
		expect: strings.Repeat("A", textplain.DefaultLineLength+1),
	})
}

func TestImgAltTags(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "self-closed img tag with alt value",
			body:   `<a href="http://example.com/"><img src="http://example.ru/hello.jpg" alt="Example"/></a>`,
			expect: "Example ( http://example.com/ )",
		},
		{
			name:   "open img tag with alt value",
			body:   `<a href="http://example.com/"><img src="http://example.ru/hello.jpg" alt="Example"></a>`,
			expect: "Example ( http://example.com/ )",
		},
		{
			name:   "self-closed img tag single quoted with alt",
			body:   `<a href='http://example.com/'><img src='http://example.ru/hello.jpg' alt='Example'/></a>`,
			expect: "Example ( http://example.com/ )",
		},
		{
			name:   "open img tag single quoted with alt",
			body:   `<a href='http://example.com/'><img src='http://example.ru/hello.jpg' alt='Example'></a>`,
			expect: "Example ( http://example.com/ )",
		},
	})
}

func TestLinks(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name:   "simple link",
			body:   `<a href="http://example.com/">Link</a>`,
			expect: `Link ( http://example.com/ )`,
		},
		{
			name:   "link with nested html",
			body:   `<a href="http://example.com/"><span class="a">Link</span></a>`,
			expect: `Link ( http://example.com/ )`,
		},
		{
			name:   "nested html with a new line",
			body:   "<a href='http://example.com/'>\n\t<span class='a'>Link</span>\n\t</a>",
			expect: `Link ( http://example.com/ )`,
		},
		{
			name:   "mailto link",
			body:   `<a href='mailto:contact@example.org'>Contact Us</a>`,
			expect: `Contact Us ( contact@example.org )`,
		},
		{
			name:   "complicated link",
			body:   `<a href="http://example.com:80/~user?aaa=bb&amp;c=d,e,f#foo">Link</a>`,
			expect: `Link ( http://example.com:80/~user?aaa=bb&c=d,e,f#foo )`,
		},
		{
			name:   "link with attribute",
			body:   `<a title='title' href="http://example.com/">Link</a>`,
			expect: `Link ( http://example.com/ )`,
		},
		{
			name:   "href attribute spacing",
			body:   `<a href="   http://example.com/ "> Link </a>`,
			expect: `Link ( http://example.com/ )`,
		},
		{
			name:   "multiple links",
			body:   `<a href="http://example.com/a/">Link A</a> <a href="http://example.com/b/">Link B</a>`,
			expect: `Link A ( http://example.com/a/ ) Link B ( http://example.com/b/ )`,
		},
		{
			name:   "link containing merge tag",
			body:   `<a href="%%LINK%%">Link</a>`,
			expect: `Link ( %%LINK%% )`,
		},
		{
			name:   "link in square brackets",
			body:   `<a href="[LINK]">Link</a>`,
			expect: `Link ( [LINK] )`,
		},
		{
			name:   "link in curly braces",
			body:   `<a href="{LINK}">Link</a>`,
			expect: `Link ( {LINK} )`,
		},
		{
			name:   "unsubscribe",
			body:   `<a href="[[!unsubscribe]]">Link</a>`,
			expect: `Link ( [[!unsubscribe]] )`,
		},
		{
			name:   "empty link gets dropped, and shouldn`t run forever",
			body:   "<a href=\"test\"></a>" + strings.Repeat("\n<p>This is some more text</p>", 15),
			expect: strings.Repeat("This is some more text\n\n", 14) + "This is some more text",
		},
		{
			name:   "links that go outside of line should wrap nicely",
			body:   "Long text before the actual link and then LINK TEXT \n( http://www.long.link ) and then more text that does not wrap",
			expect: "Long text before the actual link and then LINK TEXT\n( http://www.long.link ) and then more text that does not wrap",
		},
		{
			name:   "same text and link",
			body:   `<a href="http://example.com">http://example.com</a>`,
			expect: `http://example.com`,
		},
	})

}

// see https://github.com/premailer/premailer/issues/72
func TestMultipleLinksPerLine(t *testing.T) {
	plain, err := textplain.Convert(`<p>This is <a href="http://www.google.com" >link1</a> and <a href="http://www.google.com" >link2 </a> is next.</p>`, 10000)
	assert.Nil(t, err)

	assert.Equal(t, plain, `This is link1 ( http://www.google.com ) and link2 ( http://www.google.com ) is next.`)
}

// see https://github.com/premailer/premailer/issues/72
func TestLinksWithinHeadings(t *testing.T) {
	runTestCases(t, []testCase{
		{
			body:   "<h1><a href='http://example.com/'>Test</a></h1>",
			expect: "****************************\nTest ( http://example.com/ )\n****************************",
		},
	})
}

func TestStripsNonContentTags(t *testing.T) {
	runTestCase(t, testCase{
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
	runTestCase(t, testCase{
		body: `<h1>Horse
		Friends
						Yeah
</h1>`,
		expect: "*******\nHorse\nFriends\nYeah\n*******",
	})
}

func TestWrappingDoesntAddUnnecessaryLineBreaks(t *testing.T) {
	runTestCase(t, testCase{
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
	runTestCases(t, []testCase{
		{
			name:   "comment in tag",
			body:   "<p>in<!--comment 1-->between</p>",
			expect: "inbetween",
		},
		{
			name:   "comment between tags",
			body:   `<p>before</p><!--comment 1--><p>after</p>`,
			expect: "before\n\nafter",
		},
		{
			name:   "commented out tag",
			body:   `<p>before</p><!--comment 1<div>random</div>--><p>after</p>`,
			expect: "before\n\nafter",
		},
		{
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
		{
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
	})
}
