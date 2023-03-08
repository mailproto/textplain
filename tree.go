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
		text += line
	}

	if len(text) < 2 {
		return WordWrap(strings.TrimSpace(text), lineLength), nil
	}

	processed := make([]byte, 0, len(text))
	processed = append(processed, text[:2]...)
	idx := 1

	for i := 2; i < len(text); i++ {

		switch processed[idx] {
		case '\n':
			if text[i] == '\t' || text[i] == ' ' {
				continue
			}

			if processed[idx-1] == '\n' && text[i] == '\n' {
				continue
			}
		case ' ':
			if text[i] == ' ' {
				continue
			}
		}

		processed = append(processed, text[i])
		idx++
	}

	return WordWrap(strings.TrimSpace(string(processed)), lineLength), nil
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
			// XXX: support <!-- start text/html --> cleanup
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
				parts = append(parts, unordered(0))

				more, err := t.doConvert(c)
				if err != nil {
					return nil, err
				}

				parts = append(parts, more...)
				parts = append(parts, "\n")
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

				if strings.HasPrefix(href, "mailto:") {
					href = href[7:]
				}

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
		if l := len(line); l > maxSize {
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

		contents, err := t.doConvert(c)
		if err != nil {
			return nil, err
		}

		switch c.DataAtom {
		case atom.Li:
			parts = append(parts, prefixer(idx))
			idx++
			parts = append(parts, strings.TrimSpace(strings.Join(contents, "")))
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
