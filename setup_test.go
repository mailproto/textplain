package textplain_test

import (
	"strings"
	"testing"

	"github.com/mailproto/textplain"
	"golang.org/x/net/html"
)

func checkConvertToText(t *testing.T, expect, html string) {
	if plain, err := textplain.Convert(html, textplain.DefaultLineLength); err != nil {
		t.Error("Error converting to plaintext", err)
	} else if strings.TrimSpace(plain) != expect {
		t.Errorf("Wrong conversion of `%v`, want: %v got: %v", html, expect, plain)
	}
}

// XXX: these are copied from the main pkg
func eachElement(root *html.Node, callback func(n *html.Node) bool) {
	var iter func(*html.Node)
	iter = func(n *html.Node) {
		if n == nil {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if !callback(c) {
				return
			}
			iter(c)
		}
	}
	iter(root)
	callback(root)
}

func findElement(root *html.Node, element string) *html.Node {
	var found *html.Node
	eachElement(root, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == element {
			found = n
			return false
		}
		return true
	})

	return found
}
