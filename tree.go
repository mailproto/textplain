package textplain

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Markers stand in for things that cannot be settled until spacing and
// wrapping have run. The parser never emits control characters, so none of
// these can collide with content. Allocated in order; take the next free value
// when adding another.
const (
	// horizontalRule is drawn once Convert knows the line length
	horizontalRule = "\x00"

	// a blockquote's extent, turned into line prefixes after wrapping
	quoteOpen  = "\x01"
	quoteClose = "\x02"

	// one level of list nesting, since fixSpacing strips leading whitespace
	indentMark = "\x03"
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

	if strings.Contains(text, horizontalRule) {
		width := lineLength
		if width <= 0 {
			width = DefaultLineLength
		}

		text = strings.ReplaceAll(text, horizontalRule, strings.Repeat("-", width))
	}

	wrapped := WordWrap(strings.TrimSpace(text), lineLength)
	wrapped = strings.ReplaceAll(wrapped, "(\n", "\n( ") // XXX: cheap fix for wrapping open braces. move into WordWrap
	wrapped = strings.ReplaceAll(wrapped, "\n)", " )\n") // XXX: cheap fix for wrapping closed braces. move into WordWrap

	return applyQuotes(strings.ReplaceAll(wrapped, indentMark, "  ")), nil
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
			}

			if isHidden(c) {
				continue
			}

			switch c.DataAtom {
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
				parts = append(parts, nestedListBreak(n)...)
				parts = append(parts, t.listItems(c, unordered)...)

				continue
			case atom.Ol:
				parts = append(parts, nestedListBreak(n)...)
				parts = append(parts, t.listItems(c, ordered)...)

				continue
			case atom.Li:
				parts = append(parts, t.listItem(c, "* "))

				continue
			case atom.Dt, atom.Dd:
				parts = append(parts, t.listItem(c, ""))

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
			case atom.Blockquote:
				inner := strings.Trim(strings.Join(t.doConvert(c), ""), "\n")
				parts = append(parts, "\n\n", quoteOpen, inner, quoteClose, "\n\n")

				continue
			case atom.Br:
				parts = append(parts, "\n")

				continue
			case atom.Hr:
				parts = append(parts, "\n\n", horizontalRule, "\n\n")

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

// applyQuotes turns the marked blockquote regions into a "> " prefix on every
// line they cover, including lines produced by wrapping
func applyQuotes(text string) string {
	if !strings.Contains(text, quoteOpen) {
		return text
	}

	strip := strings.NewReplacer(quoteOpen, "", quoteClose, "")

	var (
		out   strings.Builder
		depth int
	)

	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}

		depth += strings.Count(line, quoteOpen)

		closes := strings.Count(line, quoteClose)
		line = strip.Replace(line)

		switch {
		case depth <= 0:
		case strings.TrimSpace(line) == "":
			out.WriteString(strings.TrimRight(strings.Repeat("> ", depth), " "))
		default:
			out.WriteString(strings.Repeat("> ", depth))
		}

		out.WriteString(line)

		depth -= closes
	}

	return out.String()
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
		if c.Type == html.ElementNode && isHidden(c) {
			continue
		}

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

// nestedListBreak starts a list nested in an item on its own line, so that it
// does not run into the item's text. listItem relies on that break to tell the
// two apart.
func nestedListBreak(parent *html.Node) []string {
	if parent != nil && parent.DataAtom == atom.Li {
		return []string{"\n"}
	}

	return nil
}

func (t *TreeConverter) listItem(n *html.Node, prefix string) string {
	content := strings.Trim(strings.Join(t.doConvert(n), ""), "\n")

	// everything after the first line came from a nested list and belongs one
	// level further in; marks already present deepen as they bubble up
	first, nested, _ := strings.Cut(content, "\n")

	var out strings.Builder

	out.WriteString(strings.TrimSpace(prefix + first))
	out.WriteString("\n")

	for line := range strings.SplitSeq(nested, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		out.WriteString(indentMark)
		out.WriteString(strings.TrimLeft(line, " \t"))
		out.WriteString("\n")
	}

	return out.String()
}

func (t *TreeConverter) wrapSpans(n *html.Node) (*html.Node, []string) {
	var parts []string

	var c *html.Node
	for c = n; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom != atom.Span {
			return c.PrevSibling, parts
		}

		if c.Type == html.ElementNode && isHidden(c) {
			continue
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

// isHidden reports whether an element is kept out of the rendered message.
// Preheader text meant only for the inbox preview is the usual case.
func isHidden(n *html.Node) bool {
	for _, a := range n.Attr {
		switch a.Key {
		case "hidden":
			return true
		case "aria-hidden":
			if strings.EqualFold(strings.TrimSpace(a.Val), "true") {
				return true
			}
		case "style":
			if hiddenByStyle(a.Val) {
				return true
			}
		}
	}

	return false
}

func hiddenByStyle(style string) bool {
	for declaration := range strings.SplitSeq(style, ";") {
		property, value, ok := strings.Cut(declaration, ":")
		if !ok {
			continue
		}

		property = strings.ToLower(strings.TrimSpace(property))
		value = strings.ToLower(strings.TrimSpace(value))

		switch property {
		case "display":
			if value == "none" {
				return true
			}
		case "visibility":
			if value == "hidden" || value == "collapse" {
				return true
			}
		case "opacity", "font-size", "max-height":
			if isZeroValue(value) {
				return true
			}
		}
	}

	return false
}

// isZeroValue reports whether a css number or length is zero, with or without a unit,
// so that opacity:0.5 and font-size:0.9em are not mistaken for zero
func isZeroValue(value string) bool {
	size, err := strconv.ParseFloat(strings.TrimRight(value, "abcdefghijklmnopqrstuvwxyz%"), 64)

	return err == nil && size == 0
}

func getAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}

	return ""
}
