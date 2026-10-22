package textplain_test

import (
	"fmt"
	"strings"

	"github.com/mailproto/textplain"
	nethtml "golang.org/x/net/html"
)

const exampleEmail = `<html><body>
<div style="display:none">Preview text for the inbox</div>
<h1>Your order has shipped</h1>
<p>Hi Sam, your parcel is on its way. <a href="https://example.com/track/123">Track it</a>.</p>
<ul><li>1 x Widget</li><li>2 x Gadget</li></ul>
</body></html>`

func ExampleConvert() {
	text, err := textplain.Convert(exampleEmail)
	if err != nil {
		panic(err)
	}

	fmt.Println(text)
	// Output:
	// **********************
	// Your order has shipped
	// **********************
	//
	// Hi Sam, your parcel is on its way. Track it
	// ( https://example.com/track/123 ).
	//
	// * 1 x Widget
	// * 2 x Gadget
}

func ExampleWithLinks() {
	text, err := textplain.Convert(exampleEmail,
		textplain.WithLinks(textplain.LinksFootnotes),
		textplain.WithPlainHeadings(),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(text)
	// Output:
	// Your order has shipped
	//
	// Hi Sam, your parcel is on its way. Track it [1].
	//
	// * 1 x Widget
	// * 2 x Gadget
	//
	// [1] https://example.com/track/123
}

func ExampleWithMarkdown() {
	text, err := textplain.Convert(exampleEmail, textplain.WithMarkdown())
	if err != nil {
		panic(err)
	}

	fmt.Println(text)
	// Output:
	// # Your order has shipped
	//
	// Hi Sam, your parcel is on its way. [Track it](https://example.com/track/123).
	//
	// * 1 x Widget
	// * 2 x Gadget
}

func ExampleConvertNode() {
	root, err := nethtml.Parse(strings.NewReader(exampleEmail))
	if err != nil {
		panic(err)
	}

	// the same tree can be converted more than once, as when falling back to
	// hidden content if nothing visible is left
	text, err := textplain.ConvertNode(root, textplain.WithPlainHeadings(), textplain.WithLineLength(0))
	if err == nil && text == "" {
		text, err = textplain.ConvertNode(root, textplain.WithPlainHeadings(), textplain.WithLineLength(0), textplain.WithHiddenContent())
	}

	if err != nil {
		panic(err)
	}

	fmt.Println(text)
	// Output:
	// Your order has shipped
	//
	// Hi Sam, your parcel is on its way. Track it ( https://example.com/track/123 ).
	//
	// * 1 x Widget
	// * 2 x Gadget
}

func ExampleWordWrap() {
	fmt.Println(textplain.WordWrap("The quick brown fox jumps over the lazy dog", 15))
	// Output:
	// The quick brown
	// fox jumps over
	// the lazy dog
}
