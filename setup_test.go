package textplain_test

import (
	"testing"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
)

func runTestCases(t *testing.T, testCases []testCase) {

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			runTestCase(tt, tc)
		})
	}
}

func runTestCase(t *testing.T, tc testCase) {

	converters := map[string]textplain.Converter{
		"regexp": textplain.NewRegexpConverter(),
		"tree":   textplain.NewTreeConverter(),
	}

	for name, converter := range converters {
		t.Run(name, func(tt *testing.T) {
			result, err := converter.Convert(tc.body, textplain.DefaultLineLength)
			assert.Nil(t, err)
			assert.Equal(t, tc.expect, result)
		})
	}

}

const html = `<!DOCTYPE html PUBLIC "-//W3C//DTD HTML 4.0 Transitional//EN" "http://www.w3.org/TR/REC-html40/loose.dtd"><html><head>
<meta http-equiv="Content-Type" content="text/html; charset=UTF-8"/>
    <style type="text/css">
      .button { color: red; }
    </style>
  </head>
  <body>
    <!-- start email -->
    <h6>Small header</h6>

<p style="font-weight: bold; color: red;" class="button">Hello></p>
<p>
<a href="http://example.com"><img alt="An Example" src="https://example.com/image.jpg/></a><br/>
<a href="https://example.com/fancy" class="fancy">Fancy text</a>
</p>
<ol>

  <li>item one</li>

  <li>item two</li>
  <li>item three</li>

</ol>

<img src="https://example.com/footer-animation.gif" /></body></html>`

func BenchmarkRegexp(b *testing.B) {
	converter := textplain.NewRegexpConverter()
	for i := 0; i < b.N; i++ {
		_, _ = converter.Convert(html, textplain.DefaultLineLength)
	}
}

func BenchmarkTree(b *testing.B) {
	converter := textplain.NewTreeConverter()
	for i := 0; i < b.N; i++ {
		_, _ = converter.Convert(html, textplain.DefaultLineLength)
	}
}
