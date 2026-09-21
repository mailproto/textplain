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
		return "", ErrBodyNotFound
	}

	text := t.fixSpacing(strings.Join(t.doConvert(body), ""))

	wrapped := WordWrap(strings.TrimSpace(text), lineLength)
	wrapped = strings.ReplaceAll(wrapped, "(\n", "\n( ") // XXX: cheap fix for wrapping open braces. move into WordWrap
	wrapped = strings.ReplaceAll(wrapped, "\n)", " )\n") // XXX: cheap fix for wrapping closed braces. move into WordWrap

	return wrapped, nil
}

func (t *TreeConverter) findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Body {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if body := t.findBody(c); body != nil {
			return body
		}
	}

	return nil
}

func (t *TreeConverter) doConvert(n *html.Node) []string {
	if n == nil {
		return nil
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
			// none of these render their text as document content: the media and
			// frame elements hold legacy fallback that conforming renderers ignore,
			// svg/math titles are metadata, and a template is inert
			case atom.Script, atom.Style, atom.Template,
				atom.Svg, atom.Math,
				atom.Iframe, atom.Object, atom.Embed, atom.Canvas, atom.Audio, atom.Video,
				atom.Select, atom.Datalist, atom.Textarea:
				continue
			case atom.P, atom.Div:
				more := t.doConvert(c)

				if len(parts) > 0 {
					if p := strings.Trim(parts[len(parts)-1], " \t"); len(p) == 0 || p[len(p)-1] != '\n' {
						parts = append(parts, "\n")
					}
				}

				parts = append(parts, more...)
				parts = append(parts, "\n\n")

				continue
			case atom.Ul:
				parts = append(parts, t.listItems(c, unordered)...)

				continue
			case atom.Ol:
				parts = append(parts, t.listItems(c, ordered)...)

				continue
			case atom.Li:
				parts = append(parts, t.listItem(c, "* "))

				continue
			case atom.Td, atom.Th:
				parts = append(parts, t.doConvert(c)...)
				parts = append(parts, " ")

				continue
			case atom.Tr:
				parts = append(parts, t.doConvert(c)...)
				parts = append(parts, "\n")

				continue
			case atom.Span:
				var more []string

				c, more = t.wrapSpans(c)

				parts = append(parts, more...)

				if c == nil {
					return parts
				}

				continue
			case atom.Br:
				parts = append(parts, "\n")

				continue
			case atom.H1:
				parts = append(parts, t.headerBlock(c, "*", true)...)

				continue
			case atom.H2:
				parts = append(parts, t.headerBlock(c, "-", true)...)

				continue
			case atom.H3, atom.H4, atom.H5, atom.H6:
				parts = append(parts, t.headerBlock(c, "-", false)...)

				continue
			case atom.Img:
				if alt := getAttr(c, "alt"); alt != "" {
					parts = append(parts, strings.TrimSpace(alt))
				}

				continue
			case atom.A:
				more := t.doConvert(c)

				href := strings.TrimSpace(getAttr(c, "href"))
				// a fragment only points within the document, so only its text carries over
				if href == "" || strings.HasPrefix(href, "#") {
					parts = append(parts, more...)

					continue
				}

				text := strings.TrimSpace(strings.Join(more, ""))
				if text == "" {
					text = strings.TrimSpace(getAttr(c, "alt"))
				}

				href = strings.TrimPrefix(href, "mailto:")

				if text == href {
					parts = append(parts, href)

					continue
				} else if text == "" {
					if containsImg(c) {
						parts = append(parts, "( "+href+" )")
					}

					continue
				}

				parts = append(parts, text, " ( ", href, " )")

				continue
			}
		}

		parts = append(parts, t.doConvert(c)...)
	}

	return parts
}

func containsImg(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.DataAtom == atom.Img {
			return true
		}

		if containsImg(c) {
			return true
		}
	}

	return false
}

func (t *TreeConverter) headerBlock(n *html.Node, blockChar string, prefix bool) []string {
	headerText := strings.TrimSpace(strings.Join(t.doConvert(n), ""))

	var maxSize int
	for line := range strings.SplitSeq(headerText, "\n") {
		if l := len(strings.TrimSpace(line)); l > maxSize {
			maxSize = l
		}
	}

	delimiter := strings.Repeat(blockChar, maxSize)

	block := []string{"\n\n"}
	if prefix {
		block = append(block, delimiter, "\n")
	}

	return append(block, headerText, "\n", delimiter, "\n\n")
}

func unordered(int) string { return "* " }

func ordered(idx int) string { return strconv.Itoa(idx) + ". " }

// listStart reads the start attribute of an ol, which may be negative
func listStart(n *html.Node) int {
	if start, err := strconv.Atoi(strings.TrimSpace(getAttr(n, "start"))); err == nil {
		return start
	}

	return 1
}

func (t *TreeConverter) listItems(n *html.Node, prefixer func(int) string) []string {
	var (
		parts []string
		idx   = listStart(n)
	)

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.DataAtom {
		case atom.Li:
			parts = append(parts, t.listItem(c, prefixer(idx)))
			idx++
		default:
			parts = append(parts, t.doConvert(c)...)
		}
	}

	return parts
}

func (t *TreeConverter) listItem(n *html.Node, prefix string) string {
	return strings.TrimSpace(prefix+strings.Join(t.doConvert(n), "")) + "\n"
}

func (t *TreeConverter) wrapSpans(n *html.Node) (*html.Node, []string) {
	var parts []string

	var c *html.Node
	for c = n; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom != atom.Span {
			return c.PrevSibling, parts
		}

		var span string

		switch c.Type {
		case html.ElementNode:
			span = strings.Join(t.doConvert(c), "")
		case html.TextNode:
			span = c.Data
		}

		if trimmed := strings.TrimRight(span, "\n\t "); len(trimmed) != len(span) {
			span = trimmed + " "
		}

		parts = append(parts, span)
	}

	return c, parts
}

func (t *TreeConverter) fixSpacing(rt string) string {
	runes := []rune(rt)

	if len(runes) < 2 {
		return rt
	}

	processed := make([]rune, 0, len(runes))
	processed = append(processed, runes[:2]...)
	idx := 1

	var inList = (processed[0] == '*' && processed[1] == ' ')

tidyLoop:
	for i := 2; i < len(runes); i++ {
		v := runes[i]

		switch processed[idx] {
		case '\n':
			if v == '\t' || v == ' ' {
				continue
			}

			if processed[idx-1] == '\n' && v == '\n' {
				continue
			}

			if inList && v == '\n' {
				// lookahead through any whitespace to make sure we are still in a list
				for j := i; j < len(runes); j++ {
					if runes[j] == '\t' || runes[j] == ' ' || runes[j] == '\n' {
						continue
					}

					if runes[j] == '*' && j+1 < len(runes) && runes[j+1] == ' ' {
						continue tidyLoop
					}

					break
				}
			}

			if runes[i-1] == '*' && v == ' ' {
				inList = true
			} else {
				inList = false
			}

		case ' ':
			if v == ' ' {
				continue
			}

			if v == '\t' || v == '\n' {
				processed[idx] = '\n'

				continue
			}
		}

		// handle whitespace characters being used for preheader blocks to produce a cleaner plaintext output
		switch v {
		case '\u034f', '\u00ad', '\u2007':
		whitespaceLoop:
			for j := i; j < len(runes); j++ {
				switch runes[j] {
				case ' ':
					continue
				case '\u034f', '\u00ad', '\u2007':
					i = j

					continue tidyLoop
				default:
					break whitespaceLoop
				}
			}
		}

		processed = append(processed, v)
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
