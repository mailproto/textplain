package textplain

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type TreeConverter struct{}

func NewTreeConverter() Converter {
	return &TreeConverter{}
}

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

	text := t.fixSpacing(strings.Join(lines, ""))

	return WordWrap(strings.TrimSpace(text), lineLength), nil
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

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.CommentNode:
			if strings.TrimSpace(c.Data) == "start text/html" {
				for s := c.NextSibling; s != nil; s = s.NextSibling {
					if s.Type == html.CommentNode && strings.TrimSpace(s.Data) == "end text/html" {
						c = s
						break
					}
				}
			}
			continue
		case html.TextNode:
			parts = append(parts, c.Data)
		case html.ElementNode:
			switch c.DataAtom {
			case atom.Script, atom.Style:
				continue
			case atom.P:
				more, err := t.doConvert(c)
				if err != nil {
					return nil, err
				}
				parts = append(parts, more...)
				parts = append(parts, "\n\n")
				continue
			case atom.Ul:
				li, err := t.listItems(c, unordered)
				if err != nil {
					return nil, err
				}
				parts = append(parts, li...)
				continue
			case atom.Ol:
				li, err := t.listItems(c, unordered) // XXX: change to ordered
				if err != nil {
					return nil, err
				}
				parts = append(parts, li...)
				continue
			case atom.Li:
				item, err := t.listItem(c, "* ")
				if err != nil {
					return nil, err
				}
				parts = append(parts, item)
				continue
			case atom.Span:
				var more []string
				var err error
				c, more, err = t.wrapSpans(c)
				if err != nil {
					return nil, err
				}
				parts = append(parts, more...)

				if c == nil {
					return parts, nil
				}
				continue
			case atom.Br:
				parts = append(parts, "\n")
				continue
			case atom.H1:
				more, err := t.headerBlock(c, "*", true)
				if err != nil {
					return nil, err
				}
				parts = append(parts, more...)
				continue
			case atom.H2:
				more, err := t.headerBlock(c, "-", true)
				if err != nil {
					return nil, err
				}
				parts = append(parts, more...)
				continue
			case atom.H3, atom.H4, atom.H5, atom.H6:
				more, err := t.headerBlock(c, "-", false)
				if err != nil {
					return nil, err
				}
				parts = append(parts, more...)
				continue
			case atom.Img, atom.Image:
				if alt := getAttr(c, "alt"); alt != "" {
					parts = append(parts, strings.TrimSpace(alt))
				}
				continue
			case atom.A:
				more, err := t.doConvert(c)
				if err != nil {
					return nil, err
				}

				href := getAttr(c, "href")
				if href == "" {
					parts = append(parts, more...)
					continue
				}
				text := strings.TrimSpace(strings.Join(more, ""))
				if text == "" {
					if alt := getAttr(c, "alt"); alt != "" {
						text = strings.TrimSpace(text)
					}
				}

				href = strings.TrimPrefix(href, "mailto:")

				if text == href {
					parts = append(parts, href)
					continue
				} else if text == "" {
					continue
				}

				parts = append(parts, text, " ( ", strings.TrimSpace(href), " )")

				continue
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

func (t *TreeConverter) headerBlock(n *html.Node, blockChar string, prefix bool) ([]string, error) {
	content, err := t.doConvert(n)
	if err != nil {
		return nil, err
	}
	headerText := strings.TrimSpace(strings.Join(content, ""))
	var maxSize int
	for _, line := range strings.Split(headerText, "\n") {
		if l := len(strings.TrimSpace(line)); l > maxSize {
			maxSize = l
		}
	}
	delimiter := strings.Repeat(blockChar, maxSize)

	block := []string{"\n\n"}
	if prefix {
		block = append(block, delimiter, "\n")
	}

	return append(block, headerText, "\n", delimiter, "\n\n"), nil
}

func unordered(idx int) string { return "* " }
func ordered(idx int) string   { return strconv.Itoa(idx) + ". " }

func (t *TreeConverter) listItems(n *html.Node, prefixer func(int) string) ([]string, error) {
	var parts []string
	var idx = 1
	for c := n.FirstChild; c != nil; c = c.NextSibling {

		switch c.DataAtom {
		case atom.Li:
			prefix := prefixer(idx)
			idx++

			item, err := t.listItem(c, prefix)
			if err != nil {
				return nil, err
			}
			parts = append(parts, item)
		default:
			contents, err := t.doConvert(c)
			if err != nil {
				return nil, err
			}
			parts = append(parts, contents...)
		}
	}

	return parts, nil
}

func (t *TreeConverter) listItem(n *html.Node, prefix string) (string, error) {
	contents, err := t.doConvert(n)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(prefix+strings.Join(contents, "")) + "\n", nil
}

func (t *TreeConverter) wrapSpans(n *html.Node) (*html.Node, []string, error) {

	var parts []string

	var c *html.Node
	for c = n; c != nil; c = c.NextSibling {

		if c.Type == html.ElementNode && c.DataAtom != atom.Span {
			return c, parts, nil
		}

		var span string
		switch c.Type {
		case html.ElementNode:
			children, err := t.doConvert(c)
			if err != nil {
				return c, nil, err
			}

			span = strings.Join(children, "")
		case html.TextNode:
			span = c.Data
		}

		if trimmed := strings.TrimRight(span, "\n\t "); len(trimmed) != len(span) {
			span = trimmed + " "
		}

		parts = append(parts, span)
	}

	return c, parts, nil
}

func (t *TreeConverter) fixSpacing(text string) string {

	if len(text) < 2 {
		return text
	}

	processed := make([]byte, 0, len(text))
	processed = append(processed, text[:2]...)
	idx := 1

	var inList = (processed[0] == '*' && processed[1] == ' ')

	for i := 2; i < len(text); i++ {

		switch processed[idx] {
		case '\n':

			if text[i] == '\t' || text[i] == ' ' {
				continue
			}

			if processed[idx-1] == '\n' && text[i] == '\n' {
				continue
			}

			if inList && text[i] == '\n' {
				continue
			}

			if text[i] == '*' && text[i+1] == ' ' {
				inList = true
			} else {
				inList = false
			}

		case ' ':
			if text[i] == ' ' {
				continue
			}
			if text[i] == '\t' || text[i] == '\n' {
				processed[idx] = '\n'
				continue
			}
		}

		processed = append(processed, text[i])
		idx++
	}

	return string(processed)
}

func getAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
