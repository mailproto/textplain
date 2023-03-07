package textplain

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type TreeConverter struct{}

func (t *TreeConverter) Convert(document string, lineLength int) (string, error) {

	root, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return "", err
	}

	body := t.findBody(root)
	if body == nil {
		return "", nil
	}

	lines, err := t.doConvert(body)
	if err != nil {
		return "", err
	}

	var text string
	for _, line := range lines {
		// XXX: some other stuff here
		text += WordWrap(line, lineLength)
	}

	return text, nil
}

func (t *TreeConverter) findBody(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if n.Type == html.ElementNode && n.DataAtom == atom.Body {
			return n
		}
		if body := t.findBody(c); body != nil {
			return body
		}
	}
	return nil
}

func (t *TreeConverter) doConvert(n *html.Node) ([]string, error) {
	if n == nil {
		return nil, nil
	}

	var parts []string

childLoop:
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			parts = append(parts, strings.TrimSpace(c.Data))
		case html.ElementNode:
			switch c.DataAtom {
			case atom.Ul:
				li, err := t.listItems(c, unordered)
				if err != nil {
					return nil, err
				}
				parts = append(parts, li...)
				continue childLoop
			case atom.Ol:
				li, err := t.listItems(c, unordered)
				if err != nil {
					return nil, err
				}
				parts = append(parts, li...)
				continue childLoop
			case atom.Li:

				parts = append(parts, unordered(0))

				more, err := t.doConvert(c)
				if err != nil {
					return nil, err
				}

				parts = append(parts, more...)
				parts = append(parts, "\n")
				continue childLoop
				// case atom.Li:
				// parts = append(parts,
				// if getAttr(c, "href") != "" {
				// 	parts = append(parts, )
				// }
			}
		}
		more, err := t.doConvert(c)
		if err != nil {
			return nil, err
		}
		parts = append(parts, more...)
	}

	return parts, nil
}

func unordered(idx int) string { return "* " }
func ordered(idx int) string   { return strconv.Itoa(idx) + ". " }

func (t *TreeConverter) listItems(n *html.Node, prefixer func(int) string) ([]string, error) {
	var parts []string
	var idx = 1
	for c := n.FirstChild; c != nil; c = c.NextSibling {

		contents, err := t.doConvert(c)
		if err != nil {
			return nil, err
		}

		switch c.DataAtom {
		case atom.Li:
			parts = append(parts, prefixer(idx))
			idx++
			parts = append(parts, contents...)
			parts = append(parts, "\n")
		default:
			parts = append(parts, contents...)
		}
	}

	return parts, nil
}

func getAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
