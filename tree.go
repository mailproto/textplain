package textplain

import (
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Markers stand in for things that cannot be settled until spacing and
// wrapping have run. Text and attributes are read through withoutMarkers so
// that content cannot collide with them. Allocated in order; take the next free
// value when adding another, and extend withoutMarkers.
const (
	// horizontalRule is drawn once Convert knows the line length
	horizontalRule = "\x00"

	// a blockquote's extent, turned into line prefixes after wrapping
	quoteOpen  = "\x01"
	quoteClose = "\x02"

	// one level of list nesting, since fixSpacing strips leading whitespace
	indentMark = "\x03"

	// preformatted text, lifted out so that nothing reflows it
	preOpen        = "\x04"
	preClose       = "\x05"
	prePlaceholder = "\x06"

	// Markdown only: a <br> that becomes a hard break unless a blank line follows,
	// and text that needs a backslash if it lands at the start of a line
	hardBreak     = "\x07"
	lineStartMark = "\x08"
)

// withoutMarkers drops marker bytes, where raw control characters and entities
// such as &#1; would otherwise land them. Markers are ASCII, so a byte scan
// cannot match inside a multi-byte rune.
func withoutMarkers(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] <= '\x08' {
			return strings.Map(func(r rune) rune {
				if r <= '\x08' {
					return -1
				}

				return r
			}, s)
		}
	}

	return s
}

// conversion holds the state of a single Convert call.
type conversion struct {
	opts       options
	links      []string
	showHidden bool
}

// Convert renders the body of document as plain text, wrapping at
// DefaultLineLength unless an option says otherwise. It returns
// ErrBodyNotFound if the document has no body.
func Convert(document string, opts ...Option) (string, error) {
	return ConvertReader(strings.NewReader(document), opts...)
}

// ConvertReader is Convert for a document read from r, returning any error
// from reading it.
func ConvertReader(r io.Reader, opts ...Option) (string, error) {
	root, err := html.Parse(r)
	if err != nil {
		return "", err
	}

	body := findBody(root)
	if body == nil {
		return "", ErrBodyNotFound
	}

	cv := &conversion{opts: newOptions(opts)}

	text := cv.convert(body)
	if text == "" {
		// hiding is guessed from inline styles, and a wrong guess must not empty the message
		cv = &conversion{opts: cv.opts, showHidden: true}
		text = cv.convert(body)
	}

	return text, nil
}

func (cv *conversion) convert(body *html.Node) string {
	var o output

	cv.doConvert(&o, body)

	preformatted, text := extractPre(o.String())

	text = fixSpacing(text)

	if cv.opts.markdown {
		text = settleMarkdown(text)
	}

	if strings.Contains(text, horizontalRule) {
		width := cv.opts.lineLength
		if width <= 0 {
			width = DefaultLineLength
		}

		text = strings.ReplaceAll(text, horizontalRule, strings.Repeat("-", width))
	}

	wrapped := WordWrap(strings.TrimSpace(text), cv.opts.lineLength)

	wrapped = restorePre(wrapped, preformatted)

	return applyQuotes(strings.ReplaceAll(wrapped, indentMark, "  ")) + cv.footnotes()
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

		if cv.opts.markdown {
			out.WriteString("]: ")
			out.WriteString(destination(href))
		} else {
			out.WriteString("] ")
			out.WriteString(href)
		}

		if i < len(cv.links)-1 {
			out.WriteString("\n")
		}
	}

	return out.String()
}

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Body {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if body := findBody(c); body != nil {
			return body
		}
	}

	return nil
}

func (cv *conversion) doConvert(o *output, n *html.Node) {
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
			// list items end their own line, so whitespace between them is layout
			if c.PrevSibling != nil && c.PrevSibling.DataAtom == atom.Li && strings.TrimSpace(c.Data) == "" {
				continue
			}

			o.write(cv.text(c.Data))
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

			if cv.isHidden(c) {
				continue
			}

			switch c.DataAtom {
			case atom.P, atom.Div,
				atom.Article, atom.Aside, atom.Section, atom.Header, atom.Footer, atom.Main, atom.Nav,
				atom.Figure, atom.Figcaption, atom.Address, atom.Details, atom.Summary,
				atom.Fieldset, atom.Legend, atom.Form, atom.Hgroup, atom.Caption, atom.Center:
				m := o.mark()
				cv.doConvert(o, c)
				more := o.take(m)

				if o.needsBreak() {
					o.write("\n")
				}

				o.write(more)
				o.write("\n\n")

				continue
			case atom.Ul, atom.Ol:
				if n.DataAtom == atom.Li {
					o.write("\n")
				}

				prefixer := cv.unordered
				if c.DataAtom == atom.Ol {
					prefixer = cv.ordered
				}

				cv.listItems(o, c, prefixer)

				continue
			case atom.Li:
				o.write(cv.listItem(o, c, cv.opts.bullet))

				continue
			case atom.Dt, atom.Dd:
				o.write(cv.listItem(o, c, ""))

				continue
			case atom.Td, atom.Th:
				cv.doConvert(o, c)
				o.write(" ")

				continue
			case atom.Tr:
				cv.doConvert(o, c)
				cv.lineBreak(o)

				continue
			case atom.Span:
				var done bool

				c, done = cv.wrapSpans(o, c)
				if done {
					return
				}

				continue
			case atom.Blockquote:
				m := o.mark()
				cv.doConvert(o, c)
				inner := strings.Trim(o.take(m), "\n")

				o.write("\n\n")
				o.write(quoteOpen)
				o.write(inner)
				o.write(quoteClose)
				o.write("\n\n")

				continue
			case atom.Pre:
				o.write("\n\n")
				o.write(preOpen)

				if cv.opts.markdown {
					o.write(fenced(textOf(c)))
				} else {
					o.write(textOf(c))
				}

				o.write(preClose)
				o.write("\n\n")

				continue
			case atom.Sub, atom.Sup:
				m := o.mark()
				cv.doConvert(o, c)
				text := shifted(strings.TrimSpace(o.take(m)), c.DataAtom == atom.Sup)
				if cv.opts.markdown && strings.HasPrefix(text, "_") {
					text = `\` + text
				}

				o.write(text)

				continue
			case atom.Abbr, atom.Acronym:
				m := o.mark()
				cv.doConvert(o, c)
				text := o.take(m)
				o.write(text)

				if title := strings.TrimSpace(getAttr(c, "title")); title != "" && !strings.EqualFold(title, strings.TrimSpace(text)) {
					o.write(" (" + cv.escape(title) + ")")
				}

				continue
			case atom.Br:
				cv.lineBreak(o)

				continue
			case atom.Code:
				if !cv.opts.markdown {
					break
				}

				o.write(codeSpan(textOf(c)))

				continue
			case atom.B, atom.Strong, atom.I, atom.Em:
				if !cv.opts.markdown {
					break
				}

				delim := "*"
				if c.DataAtom == atom.B || c.DataAtom == atom.Strong {
					delim = "**"
				}

				m := o.mark()
				cv.doConvert(o, c)
				o.write(emphasized(o.take(m), delim))

				continue
			case atom.Hr:
				o.write("\n\n")
				o.write(horizontalRule)
				o.write("\n\n")

				continue
			case atom.H1:
				cv.headerBlock(o, c, "*", true)

				continue
			case atom.H2:
				cv.headerBlock(o, c, "-", true)

				continue
			case atom.H3, atom.H4, atom.H5, atom.H6:
				cv.headerBlock(o, c, "-", false)

				continue
			case atom.Img:
				alt := strings.TrimSpace(getAttr(c, "alt"))

				switch src := strings.TrimSpace(getAttr(c, "src")); {
				case alt == "":
				case cv.opts.markdown && src != "":
					o.write("![" + cv.escape(oneLine(alt)) + "](" + destination(src) + ")")
				default:
					o.write(cv.escape(alt))
				}

				continue
			case atom.A:
				m := o.mark()
				cv.doConvert(o, c)
				more := o.take(m)

				href := strings.TrimSpace(getAttr(c, "href"))
				// a fragment only points within the document, so only its text carries over
				if href == "" || strings.HasPrefix(href, "#") {
					o.write(more)

					continue
				}

				text := strings.TrimSpace(more)
				if text == "" {
					text = cv.escape(strings.TrimSpace(getAttr(c, "alt")))
				}

				// Markdown needs the scheme, or the address reads as a relative link
				if !cv.opts.markdown && len(href) >= len("mailto:") && strings.EqualFold(href[:len("mailto:")], "mailto:") {
					href = href[len("mailto:"):]
				}

				cv.link(o, text, href, containsImg(c))

				continue
			}
		}

		cv.doConvert(o, c)
	}
}

// shifted marks sub and superscript text as _x and ^x. Symbols such as ® or †
// read the same unmarked, so they are left alone.
func shifted(text string, sup bool) string {
	if !strings.ContainsFunc(text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return text
	}

	if strings.ContainsAny(text, " \t\n") {
		text = "(" + text + ")"
	}

	if sup {
		return "^" + text
	}

	return "_" + text
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

// extractPre lifts each preformatted block out of the text, leaving a
// placeholder, so that spacing and wrapping do not touch it
func extractPre(text string) ([]string, string) {
	if !strings.Contains(text, preOpen) {
		return nil, text
	}

	var (
		blocks []string
		out    strings.Builder
	)

	for {
		before, rest, found := strings.Cut(text, preOpen)
		if !found {
			break
		}

		block, after, _ := strings.Cut(rest, preClose)

		out.WriteString(before)
		out.WriteString(prePlaceholder)
		blocks = append(blocks, strings.Trim(block, "\n"))

		text = after
	}

	out.WriteString(text)

	return blocks, out.String()
}

// restorePre puts each preformatted block back in place of its placeholder, in
// one pass
func restorePre(text string, blocks []string) string {
	if len(blocks) == 0 {
		return text
	}

	size := len(text)
	for _, block := range blocks {
		size += len(block)
	}

	var out strings.Builder

	out.Grow(size)

	for _, block := range blocks {
		before, after, _ := strings.Cut(text, prePlaceholder)

		out.WriteString(before)
		out.WriteString(block)
		text = after
	}

	out.WriteString(text)

	return out.String()
}

// textOf collects the raw text of a subtree, keeping whitespace as written
func textOf(n *html.Node) string {
	var (
		sb   strings.Builder
		walk func(*html.Node)
	)

	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch c.Type {
			case html.TextNode:
				sb.WriteString(withoutMarkers(c.Data))
			case html.ElementNode:
				if c.DataAtom == atom.Br {
					sb.WriteString("\n")
				}

				walk(c)
			}
		}
	}
	walk(n)

	return sb.String()
}

// text makes the content of a text node or attribute safe to write
func (cv *conversion) text(s string) string {
	return cv.escape(withoutMarkers(s))
}

func (cv *conversion) escape(s string) string {
	if cv.opts.markdown {
		return escapeMarkdown(s)
	}

	return s
}

// lineBreak ends a line within a block
func (cv *conversion) lineBreak(o *output) {
	if cv.opts.markdown {
		o.write(hardBreak)
	}

	o.write("\n")
}

// link renders an anchor according to the chosen style
func (cv *conversion) link(o *output, text, href string, hasImg bool) {
	if cv.opts.markdown && cv.opts.links != LinksOmitted {
		cv.markdownLink(o, text, href, hasImg)

		return
	}

	switch cv.opts.links {
	case LinksOmitted:
		if text != "" {
			o.write(text)
		}

		return

	case LinksFootnotes:
		if text == "" && !hasImg {
			return
		}

		cv.links = append(cv.links, href)

		if text != "" {
			o.write(text)
			o.write(" ")
		}

		o.write("[")
		o.write(strconv.Itoa(len(cv.links)))
		o.write("]")

		return
	}

	switch {
	case sameTarget(text, href):
		o.write(href)
	case text != "":
		o.write(text)
		o.write(" ( ")
		o.write(href)
		o.write(" )")
	case hasImg:
		o.write("( ")
		o.write(href)
		o.write(" )")
	}
}

// markdownLink writes an inline or reference link, falling back to the target as
// the text when an image inside the link has no alt
func (cv *conversion) markdownLink(o *output, text, href string, hasImg bool) {
	if text == "" && !hasImg {
		return
	}

	if text == "" {
		text = escapeMarkdown(href)
	}

	o.write("[")
	o.write(oneLine(text))

	if cv.opts.links == LinksFootnotes {
		cv.links = append(cv.links, href)

		o.write("][")
		o.write(strconv.Itoa(len(cv.links)))
		o.write("]")

		return
	}

	o.write("](")
	o.write(destination(href))
	o.write(")")
}

// sameTarget reports whether link text is just its href, give or take the scheme
// and a trailing slash
func sameTarget(text, href string) bool {
	trim := func(s string) string {
		s = strings.TrimSuffix(s, "/")
		if _, rest, ok := strings.Cut(s, "://"); ok {
			return rest
		}

		return s
	}

	return text != "" && trim(text) == trim(href)
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

func (cv *conversion) headerBlock(o *output, n *html.Node, blockChar string, prefix bool) {
	m := o.mark()
	cv.doConvert(o, n)
	headerText := strings.TrimSpace(o.take(m))

	if cv.opts.markdown {
		if headerText = oneLine(headerText); headerText != "" {
			o.write("\n\n")
			o.write(strings.Repeat("#", int(n.Data[1]-'0')))
			o.write(" ")
			o.write(headerText)
			o.write("\n\n")
		}

		return
	}

	if cv.opts.plainHeadings {
		o.write("\n\n")
		o.write(headerText)
		o.write("\n\n")

		return
	}

	var maxSize int
	for line := range strings.SplitSeq(headerText, "\n") {
		if l := len(strings.TrimSpace(line)); l > maxSize {
			maxSize = l
		}
	}

	delimiter := strings.Repeat(blockChar, maxSize)

	o.write("\n\n")

	if prefix {
		o.write(delimiter)
		o.write("\n")
	}

	o.write(headerText)
	o.write("\n")
	o.write(delimiter)
	o.write("\n\n")
}

func (cv *conversion) unordered(int) string { return cv.opts.bullet }

func (cv *conversion) ordered(idx int) string {
	return strconv.Itoa(idx) + cv.opts.orderedSuffix
}

// listStart reads the start attribute of an ol, which may be negative
func listStart(n *html.Node) int {
	if start, err := strconv.Atoi(strings.TrimSpace(getAttr(n, "start"))); err == nil {
		return start
	}

	return 1
}

func (cv *conversion) listItems(o *output, n *html.Node, prefixer func(int) string) {
	idx := listStart(n)

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && cv.isHidden(c) {
			continue
		}

		switch c.DataAtom {
		case atom.Li:
			o.write(cv.listItem(o, c, prefixer(idx)))
			idx++
		default:
			cv.doConvert(o, c)
		}
	}
}

func (cv *conversion) listItem(o *output, n *html.Node, prefix string) string {
	m := o.mark()
	cv.doConvert(o, n)

	content := strings.Trim(o.take(m), "\n")

	// everything after the first line came from a nested list and belongs one
	// level further in; marks already present deepen as they bubble up
	first, nested, _ := strings.Cut(content, "\n")

	// Markdown nests under the item's content, so the indent covers the whole marker
	indent := indentMark
	if cv.opts.markdown {
		first = escapeLead(strings.TrimLeft(first, " \t"))
		indent = strings.Repeat(indentMark, (utf8.RuneCountInString(prefix)+1)/2)
	}

	var out strings.Builder

	out.WriteString(strings.TrimSpace(prefix + first))
	out.WriteString("\n")

	for line := range strings.SplitSeq(nested, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		out.WriteString(indent)
		out.WriteString(strings.TrimLeft(line, " \t"))
		out.WriteString("\n")
	}

	return out.String()
}

// wrapSpans consumes a run of sibling spans, returning the node to resume from
// and whether the run reached the end of the siblings
func (cv *conversion) wrapSpans(o *output, n *html.Node) (*html.Node, bool) {
	var c *html.Node

	for c = n; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom != atom.Span {
			return c.PrevSibling, false
		}

		if c.Type == html.ElementNode && cv.isHidden(c) {
			continue
		}

		var span string

		switch c.Type {
		case html.ElementNode:
			m := o.mark()
			cv.doConvert(o, c)
			span = o.take(m)
		case html.TextNode:
			span = cv.text(c.Data)
		}

		if trimmed := strings.TrimRight(span, "\n\t "); len(trimmed) != len(span) {
			span = trimmed + " "
		}

		o.write(span)
	}

	return nil, true
}

func fixSpacing(rt string) string {
	first, firstSize := utf8.DecodeRuneInString(rt)
	if firstSize == 0 {
		return rt
	}

	second, secondSize := utf8.DecodeRuneInString(rt[firstSize:])
	if secondSize == 0 {
		return rt
	}

	var out strings.Builder

	out.Grow(len(rt))
	out.WriteRune(first)

	// out holds everything already settled; last is held back because a run of
	// spaces can still turn it into a newline, and beforeLast is only read
	var (
		beforeLast = first
		last       = second
	)

	for i := firstSize + secondSize; i < len(rt); {
		v, size := utf8.DecodeRuneInString(rt[i:])
		keep := true

		switch {
		case last == '\n' && (v == '\t' || v == ' '):
			keep = false
		case last == '\n' && beforeLast == '\n' && v == '\n':
			keep = false
		case last == ' ' && v == ' ':
			keep = false
		case last == ' ' && (v == '\t' || v == '\n'):
			last = '\n'
			keep = false
		}

		if keep && !isPreheaderMark(v) {
			out.WriteRune(last)

			beforeLast, last = last, v
		}

		i += size
	}

	out.WriteRune(last)

	return out.String()
}

func isPreheaderMark(r rune) bool {
	return strings.ContainsRune("\u034f\u00ad\u2007\u200b\u200c\ufeff", r)
}

// isHidden reports whether an element is kept out of the rendered message.
// Preheader text meant only for the inbox preview is the usual case.
func (cv *conversion) isHidden(n *html.Node) bool {
	if cv.showHidden {
		return false
	}

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

// font-size:0 is left out: it is inherited and descendants reset it, as in the
// inline-block gap fix that wraps whole layouts
func hiddenByStyle(style string) bool {
	var zeroHeight, overflowHidden bool

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
		case "opacity":
			if isZeroValue(value) {
				return true
			}
		case "max-height":
			zeroHeight = isZeroValue(value)
		case "overflow", "overflow-y":
			overflowHidden = value == "hidden"
		}
	}

	return zeroHeight && overflowHidden
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
			return withoutMarkers(a.Val)
		}
	}

	return ""
}
