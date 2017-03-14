package textplain_test

import (
	"strings"
	"testing"

	"github.com/mailproto/textplain"
)

func TestToPlaintextWithFragment(t *testing.T) {
	checkConvertToText(t, `Test`, `<p>Test</p>`)
}

func TestToPlaintextWithBody(t *testing.T) {
	checkConvertToText(t, `Test`, `<html>
	    <title>Ignore me</title>
	    <body>
			<p>Test</p>
			</body>
			</html>`)
}

func TestToPlaintextWithMalformedBody(t *testing.T) {
	checkConvertToText(t, `Test`, `<html>
    <title>Ignore me</title>
    <body>
		<p>Test`)
}

func TestSpecialChars(t *testing.T) {
	checkConvertToText(t, "cédille garçon & à ñ", "c&eacute;dille gar&#231;on &amp; &agrave; &ntilde;")
}

func TestStrippingWhitespace(t *testing.T) {
	checkConvertToText(t, "text\ntext", "  \ttext\ntext\n")
	checkConvertToText(t, "a\na", "  \na \n a \t")
	checkConvertToText(t, "a\n\na", "  \na \n\t \n \n a \t")
	checkConvertToText(t, "test text", "test text&nbsp;")
	checkConvertToText(t, "test text", "test        text")
}

func TestWrappingSpans(t *testing.T) {
	checkConvertToText(t, `Test line 2`, `<html>
	    <body>
			<p><span>Test</span>
			<span>line 2</span>
			</p>`)
}

func TestLineBreaks(t *testing.T) {
	checkConvertToText(t, "Test text\nTest text", "Test text\r\nTest text")
	checkConvertToText(t, "Test text\nTest text", "Test text\rTest text")
}

func TestLists(t *testing.T) {
	checkConvertToText(t, "* item 1\n* item 2", "<li class='123'>item 1</li> <li>item 2</li>\n")
	checkConvertToText(t, "* item 1\n* item 2\n* item 3", "<li>item 1</li> \t\n <li>item 2</li> <li> item 3</li>\n")
}

func TestStrippingHTML(t *testing.T) {
	checkConvertToText(t, "test text", "<p class=\"123'45 , att\" att=tester>test <span class='te\"st'>text</span>\n")
}

func TestStrippingIgnoredBlocks(t *testing.T) {
	checkConvertToText(t, "test\n\ntext", `<p>test</p>
    <!-- start text/html -->
      <img src="logo.png" alt="logo">
    <!-- end text/html -->
    <p>text</p>`)
}

func TestParagraphsAndBreaks(t *testing.T) {
	checkConvertToText(t, "Test text\n\nTest text", "<p>Test text</p><p>Test text</p>")
	checkConvertToText(t, "Test text\n\nTest text", "\n<p>Test text</p>\n\n\n\t<p>Test text</p>\n")
	checkConvertToText(t, "Test text\nTest text", "\n<p>Test text<br/>Test text</p>\n")
	checkConvertToText(t, "Test text\nTest text", "\n<p>Test text<br> \tTest text<br></p>\n")
	checkConvertToText(t, "Test text\n\nTest text", "Test text<br><BR />Test text")
}

func TestHeadings(t *testing.T) {
	checkConvertToText(t, "****\nTest\n****", "<h1>Test</h1>")
	checkConvertToText(t, "****\nTest\n****", "\t<h1>\nTest</h1>")
	checkConvertToText(t, "***********\nTest line 1\nTest 2\n***********", "\t<h1>\nTest line 1<br>Test 2</h1> ")
	checkConvertToText(t, "****\nTest\n****\n\n****\nTest\n****", "<h1>Test</h1> <h1>Test</h1>")
	checkConvertToText(t, "----\nTest\n----", "<h2>Test</h2>")
	checkConvertToText(t, "Test\n----", "<h3> <span class='a'>Test </span></h3>")
}

func TestWrappingLines(t *testing.T) {
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
	raw := "Long     line new line"
	expect := "Long line\nnew line"
	if plain, err := textplain.Convert(raw, 10); err != nil || strings.TrimSpace(plain) != expect {
		t.Errorf("Wrong plaintext content, want: %v got: %v (%v)", expect, plain, err)
	}
}

func TestImgAltTags(t *testing.T) {

	//  ensure html imag tags that aren't self-closed are parsed,
	//  along with accepting both '' and "" as attribute quotes

	//  <img alt="" /> closed
	checkConvertToText(t, "Example ( http://example.com/ )", `<a href="http://example.com/"><img src="http://example.ru/hello.jpg" alt="Example"/></a>`)

	//  <img alt=""> not closed
	checkConvertToText(t, "Example ( http://example.com/ )", `<a href="http://example.com/"><img src="http://example.ru/hello.jpg" alt="Example"></a>`)

	//  <img alt='' />
	checkConvertToText(t, "Example ( http://example.com/ )", `<a href='http://example.com/'><img src='http://example.ru/hello.jpg' alt='Example'/></a>`)

	//  <img alt=''>
	checkConvertToText(t, "Example ( http://example.com/ )", `<a href='http://example.com/'><img src='http://example.ru/hello.jpg' alt='Example'></a>`)

}

func TestLinks(t *testing.T) {

	// basic
	checkConvertToText(t, `Link ( http://example.com/ )`, `<a href="http://example.com/">Link</a>`)

	// nested html
	checkConvertToText(t, `Link ( http://example.com/ )`, `<a href="http://example.com/"><span class="a">Link</span></a>`)

	// nested html with new line
	checkConvertToText(t, `Link ( http://example.com/ )`, "<a href='http://example.com/'>\n\t<span class='a'>Link</span>\n\t</a>")

	// mailto
	checkConvertToText(t, `Contact Us ( contact@example.org )`, `<a href='mailto:contact@example.org'>Contact Us</a>`)

	// complex link
	checkConvertToText(t, `Link ( http://example.com:80/~user?aaa=bb&c=d,e,f#foo )`, `<a href="http://example.com:80/~user?aaa=bb&amp;c=d,e,f#foo">Link</a>`)

	// attributes
	checkConvertToText(t, `Link ( http://example.com/ )`, `<a title='title' href="http://example.com/">Link</a>`)

	// spacing
	checkConvertToText(t, `Link ( http://example.com/ )`, `<a href="   http://example.com/ "> Link </a>`)

	// multiple
	checkConvertToText(t, `Link A ( http://example.com/a/ ) Link B ( http://example.com/b/ )`, `<a href="http://example.com/a/">Link A</a> <a href="http://example.com/b/">Link B</a>`)

	// merge links
	checkConvertToText(t, `Link ( %%LINK%% )`, `<a href="%%LINK%%">Link</a>`)
	checkConvertToText(t, `Link ( [LINK] )`, `<a href="[LINK]">Link</a>`)
	checkConvertToText(t, `Link ( {LINK} )`, `<a href="{LINK}">Link</a>`)

	// unsubscribe
	checkConvertToText(t, `Link ( [[!unsubscribe]] )`, `<a href="[[!unsubscribe]]">Link</a>`)

	// empty link gets dropped, and shouldn`t run forever
	content := strings.Repeat("\n<p>This is some more text</p>", 15)
	checkConvertToText(t, strings.Repeat("This is some more text\n\n", 14)+"This is some more text", "<a href=\"test\"></a>"+content)

	// links that go outside of line should wrap nicely
	checkConvertToText(t, "Long text before the actual link and then LINK TEXT \n( http://www.long.link ) and then more text that does not wrap", `Long text before the actual link and then <a href="http://www.long.link"/>LINK TEXT</a> and then more text that does not wrap`)

	// same text and link
	checkConvertToText(t, `http://example.com`, `<a href="http://example.com">http://example.com</a>`)

}

// see https://github.com/alexdunae/premailer/issues/72
func TestMultipleLinksPerLine(t *testing.T) {
	html := `<p>This is <a href="http://www.google.com" >link1</a> and <a href="http://www.google.com" >link2 </a> is next.</p>`
	expect := `This is link1 ( http://www.google.com ) and link2 ( http://www.google.com ) is next.`

	if plain, err := textplain.Convert(html, 10000); err != nil {
		t.Error("Error converting to plaintext", err)
	} else if strings.TrimSpace(plain) != expect {
		t.Errorf("Wrong conversion of `%v`, want: %v got: %v (%v)", html, expect, plain, err)
	}
}

// see https://github.com/alexdunae/premailer/issues/72
func TestLinksWithinHeadings(t *testing.T) {
	checkConvertToText(t, "****************************\nTest ( http://example.com/ )\n****************************", "<h1><a href='http://example.com/'>Test</a></h1>")
}

func TestStripsNonContentTags(t *testing.T) {
	html := `<html>
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
</html>`

	checkConvertToText(t, "No hacks here\n\nThis is not a bold statement", html)
}
