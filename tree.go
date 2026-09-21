package textplain

import (
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	quoteOpen      = "\x01"
	quoteClose     = "\x02"
	horizontalRule = "\x00"
)

type TreeConverter struct{}

// conversion holds the state of a single Convert call. TreeConverter is shared,
// so nothing that varies per document may live on it.
type conversion struct {
	opts  Options
	links []string
	base  *url.URL
}

func NewTreeConverter() Converter {
	return &TreeConverter{}
}

func (t *TreeConverter) Convert(document string, lineLength int) (string, error) {
	return t.ConvertWithOptions(document, Options{LineLength: lineLength})
}

func (t *TreeConverter) ConvertWithOptions(document string, opts Options) (string, error) {
	root, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return "", err
	}

	cv := &conversion{opts: opts}
	cv.base = documentBase(root, opts.BaseURL)

	body := cv.findBody(root)
	if body == nil {
		return "", ErrBodyNotFound
	}

	text := cv.fixSpacing(strings.Join(cv.doConvert(body), ""))

	if strings.Contains(text, horizontalRule) {
		width := opts.LineLength
		if width <= 0 {
			width = DefaultLineLength
		}

		text = strings.ReplaceAll(text, horizontalRule, strings.Repeat("-", width))
	}

	wrapped := WordWrap(strings.TrimSpace(text), opts.LineLength)
	wrapped = strings.ReplaceAll(wrapped, "(\n", "\n( ") // XXX: cheap fix for wrapping open braces. move into WordWrap
	wrapped = strings.ReplaceAll(wrapped, "\n)", " )\n") // XXX: cheap fix for wrapping closed braces. move into WordWrap

	return applyQuotes(wrapped) + cv.footnotes(), nil
}

// footnotes lists the collected link targets under the body
func (cv *conversion) footnotes() string {
	if len(cv.links) == 0 {
		return ""
	}

	var out strings.Builder

	out.WriteString("\n\n")

	for i, href := range cv.links {
		out.WriteString("[")
		out.WriteString(strconv.Itoa(i + 1))
		out.WriteString("] ")
		out.WriteString(href)

		if i < len(cv.links)-1 {
			out.WriteString("\n")
		}
	}

	return out.String()
}

// documentBase prefers a base element over the configured base url, matching
// how a browser resolves the document's links
func documentBase(root *html.Node, configured string) *url.URL {
	base, err := url.Parse(configured)
	if err != nil {
		base = nil
	}

	if href := findBaseHref(root); href != "" {
		if fromDocument, err := url.Parse(href); err == nil {
			if base != nil {
				base = base.ResolveReference(fromDocument)
			} else {
				base = fromDocument
			}
		}
	}

	if base == nil || !base.IsAbs() {
		return nil
	}

	return base
}

func findBaseHref(n *html.Node) string {
	if n.Type == html.ElementNode && n.DataAtom == atom.Base {
		if href := strings.TrimSpace(getAttr(n, "href")); href != "" {
			return href
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if href := findBaseHref(c); href != "" {
			return href
		}
	}

	return ""
}

// resolve turns a relative target into an absolute one when a base is known
func (cv *conversion) resolve(href string) string {
	if cv.base == nil || href == "" {
		return href
	}

	ref, err := url.Parse(href)
	if err != nil || ref.IsAbs() {
		return href
	}

	return cv.base.ResolveReference(ref).String()
}

func (cv *conversion) findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Body {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if body := cv.findBody(c); body != nil {
			return body
		}
	}

	return nil
}

func (cv *conversion) doConvert(n *html.Node) []string {
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
				more := cv.doConvert(c)

				if len(parts) > 0 {
					if p := strings.Trim(parts[len(parts)-1], " \t"); len(p) == 0 || p[len(p)-1] != '\n' {
						parts = append(parts, "\n")
					}
				}

				parts = append(parts, more...)
				parts = append(parts, "\n\n")

				continue
			case atom.Ul:
				parts = append(parts, cv.listItems(c, cv.unordered)...)

				continue
			case atom.Ol:
				parts = append(parts, cv.listItems(c, cv.ordered)...)

				continue
			case atom.Li:
				parts = append(parts, cv.listItem(c, cv.opts.bullet()))

				continue
			case atom.Dt, atom.Dd:
				parts = append(parts, cv.listItem(c, ""))

				continue
			case atom.Td, atom.Th:
				parts = append(parts, cv.doConvert(c)...)
				parts = append(parts, " ")

				continue
			case atom.Tr:
				parts = append(parts, cv.doConvert(c)...)
				parts = append(parts, "\n")

				continue
			case atom.Span:
				var more []string

				c, more = cv.wrapSpans(c)

				parts = append(parts, more...)

				if c == nil {
					return parts
				}

				continue
			case atom.Blockquote:
				inner := strings.Trim(strings.Join(cv.doConvert(c), ""), "\n")
				parts = append(parts, "\n\n", quoteOpen, inner, quoteClose, "\n\n")

				continue
			case atom.Br:
				parts = append(parts, "\n")

				continue
			case atom.Hr:
				parts = append(parts, "\n\n", horizontalRule, "\n\n")

				continue
			case atom.H1:
				parts = append(parts, cv.headerBlock(c, "*", true)...)

				continue
			case atom.H2:
				parts = append(parts, cv.headerBlock(c, "-", true)...)

				continue
			case atom.H3, atom.H4, atom.H5, atom.H6:
				parts = append(parts, cv.headerBlock(c, "-", false)...)

				continue
			case atom.Img:
				if alt := getAttr(c, "alt"); alt != "" {
					parts = append(parts, strings.TrimSpace(alt))
				}

				continue
			case atom.A:
				more := cv.doConvert(c)

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

				if !strings.HasPrefix(href, "mailto:") {
					href = cv.resolve(href)
				}

				href = strings.TrimPrefix(href, "mailto:")

				parts = append(parts, cv.link(text, strings.TrimSpace(href), containsImg(c))...)

				continue
			}
		}

		parts = append(parts, cv.doConvert(c)...)
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

// link renders an anchor according to the chosen style
func (cv *conversion) link(text, href string, hasImg bool) []string {
	switch cv.opts.Links {
	case LinksOmitted:
		if text == "" {
			return nil
		}

		return []string{text}

	case LinksFootnotes:
		if text == "" && !hasImg {
			return nil
		}

		cv.links = append(cv.links, href)
		marker := "[" + strconv.Itoa(len(cv.links)) + "]"

		if text == "" {
			return []string{marker}
		}

		return []string{text, " ", marker}
	}

	if text == href {
		return []string{href}
	}

	if text == "" {
		if hasImg {
			return []string{"( " + href + " )"}
		}

		return nil
	}

	return []string{text, " ( ", href, " )"}
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

func (cv *conversion) headerBlock(n *html.Node, blockChar string, prefix bool) []string {
	headerText := strings.TrimSpace(strings.Join(cv.doConvert(n), ""))

	var maxSize int
	for line := range strings.SplitSeq(headerText, "\n") {
		if l := len(strings.TrimSpace(line)); l > maxSize {
			maxSize = l
		}
	}

	if cv.opts.PlainHeadings {
		return []string{"\n\n", headerText, "\n\n"}
	}

	delimiter := strings.Repeat(blockChar, maxSize)

	block := []string{"\n\n"}
	if prefix {
		block = append(block, delimiter, "\n")
	}

	return append(block, headerText, "\n", delimiter, "\n\n")
}

func (cv *conversion) unordered(int) string { return cv.opts.bullet() }

func (cv *conversion) ordered(idx int) string {
	return strconv.Itoa(idx) + cv.opts.orderedSuffix()
}

// listStart reads the start attribute of an ol, which may be negative
func listStart(n *html.Node) int {
	if start, err := strconv.Atoi(strings.TrimSpace(getAttr(n, "start"))); err == nil {
		return start
	}

	return 1
}

func (cv *conversion) listItems(n *html.Node, prefixer func(int) string) []string {
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
			parts = append(parts, cv.listItem(c, prefixer(idx)))
			idx++
		default:
			parts = append(parts, cv.doConvert(c)...)
		}
	}

	return parts
}

func (cv *conversion) listItem(n *html.Node, prefix string) string {
	return strings.TrimSpace(prefix+strings.Join(cv.doConvert(n), "")) + "\n"
}

func (cv *conversion) wrapSpans(n *html.Node) (*html.Node, []string) {
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
			span = strings.Join(cv.doConvert(c), "")
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

func (cv *conversion) fixSpacing(rt string) string {
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
