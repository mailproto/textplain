package textplain

import (
	"bytes"
	"io"
	"slices"
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
		// eight bytes at a time: subtracting 9 from each byte sets the high bit
		// only of those below 9
		for len(s)-i >= 8 {
			if w := word8(s, i); (w-0x0909090909090909)&^w&high != 0 {
				break
			}

			i += 8
		}

		if i < len(s) && s[i] <= '\x08' {
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
	opts   options
	links  []string
	quotes int
	last   writtenLink
}

// writtenLink is the most recent link written and where it sits in the output
type writtenLink struct {
	text, href string
	hasImg     bool
	start, end int
}

// Convert renders the body of document as plain text, wrapping at
// DefaultLineLength unless an option says otherwise. It returns
// ErrBodyNotFound if the document has no body.
//
// The result is empty when the body has no visible text, which can mean
// hidden content was misjudged; callers that need text can retry with
// WithHiddenContent.
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

	return ConvertNode(root, opts...)
}

// ConvertNode is Convert for a document already parsed with html.Parse, for a
// caller that has the tree and would otherwise render it only to have it parsed
// again. It does not modify the tree.
func ConvertNode(root *html.Node, opts ...Option) (string, error) {
	body := findBody(root)
	if body == nil {
		return "", ErrBodyNotFound
	}

	cv := &conversion{}
	cv.opts.set(opts)

	o := newOutput()
	defer o.release()

	cv.doConvert(o, body)

	// read straight from the buffer, which is not released until this returns
	preformatted, raw := extractPre(o.buf.Bytes())

	text := fixSpacing(raw)

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

	return applyQuotes(strings.ReplaceAll(wrapped, indentMark, "  ")) + cv.footnotes(), nil
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
				if o.needsBreak() {
					o.write("\n")
				}

				cv.doConvert(o, c)
				o.write("\n\n")

				continue
			case atom.Ul, atom.Ol:
				if n.DataAtom == atom.Li {
					o.write("\n")
				}

				prefixer := cv.unordered
				if c.DataAtom == atom.Ol {
					kind := strings.TrimSpace(getAttr(c, "type"))
					prefixer = func(idx int) string { return cv.listNumber(idx, kind) + cv.opts.orderedSuffix }
				}

				cv.listItems(o, c, prefixer)

				continue
			case atom.Li:
				cv.listItem(o, c, cv.opts.bullet)

				continue
			case atom.Dt, atom.Dd:
				cv.listItem(o, c, "")

				continue
			case atom.Table:
				if rows := cv.dataRows(c); rows != nil && cv.dataTable(o, c, rows) {
					continue
				}
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
			case atom.Del, atom.S, atom.Strike:
				m := o.mark()
				cv.doConvert(o, c)
				o.write(emphasized(o.take(m), "~~"))

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
			case atom.Q:
				// nested quotes alternate, as browsers render them
				mark := `"`
				if cv.quotes%2 == 1 {
					mark = "'"
				}

				cv.quotes++
				o.write(mark)
				cv.doConvert(o, c)
				o.write(mark)
				cv.quotes--

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
			case atom.Input:
				switch strings.ToLower(strings.TrimSpace(getAttr(c, "type"))) {
				case "submit", "button", "reset":
					o.write(cv.escape(strings.TrimSpace(getAttr(c, "value"))))
				}

				continue
			case atom.Img:
				alt := strings.TrimSpace(getAttr(c, "alt"))

				switch src := strings.TrimSpace(getAttr(c, "src")); {
				case alt == "":
				// an inline image would copy its whole encoding into the text
				case cv.opts.markdown && src != "" && !hasScheme(src, "data:"):
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
				// a fragment only points within the document and a script only runs in a
				// browser, so only the text carries over
				if href == "" || strings.HasPrefix(href, "#") || hasScheme(href, "javascript:") {
					o.write(more)

					continue
				}

				text := strings.TrimSpace(more)
				if text == "" {
					text = cv.escape(strings.TrimSpace(getAttr(c, "alt")))
				}

				// Markdown needs the scheme, or the address reads as a relative link
				if !cv.opts.markdown && hasScheme(href, "mailto:") {
					href = href[len("mailto:"):]
				}

				cv.link(o, text, href, contains(c, atom.Img))

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
func extractPre(text []byte) ([][]byte, []byte) {
	if !bytes.Contains(text, []byte(preOpen)) {
		return nil, text
	}

	var (
		blocks [][]byte
		out    = make([]byte, 0, len(text))
	)

	for {
		before, rest, found := bytes.Cut(text, []byte(preOpen))
		if !found {
			break
		}

		block, after, _ := bytes.Cut(rest, []byte(preClose))

		out = append(append(out, before...), prePlaceholder...)
		blocks = append(blocks, bytes.Trim(block, "\n"))

		text = after
	}

	return blocks, append(out, text...)
}

// restorePre puts each preformatted block back in place of its placeholder, in
// one pass
func restorePre(text string, blocks [][]byte) string {
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
		out.Write(block)
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

// link writes an anchor unless the link just before it, with only space between,
// already points at the same target, as an image and the caption under it often
// do. A link that is only an image gives way to a text link that follows it,
// unless an element opened since would have its start cut away.
func (cv *conversion) link(o *output, text, href string, hasImg bool) {
	if cv.last.href == href && cv.adjacent(o) {
		switch {
		case text == "" || strings.EqualFold(text, cv.last.text):
			return
		case cv.last.text == "" && cv.last.start >= o.floor:
			gap := string(o.buf.Bytes()[cv.last.end:])
			o.buf.Truncate(cv.last.start)
			o.write(gap)
		}
	}

	start := o.buf.Len()
	cv.renderLink(o, text, href, hasImg)

	if o.buf.Len() > start {
		cv.last = writtenLink{text: text, href: href, hasImg: hasImg, start: start, end: o.buf.Len()}
	}
}

// adjacent reports whether the last link written still ends the output, give or
// take space. Blocks rewrite their contents, so it is rendered again to check.
func (cv *conversion) adjacent(o *output) bool {
	if cv.last.end > o.buf.Len() || !onlySpace(o.buf.Bytes()[cv.last.end:]) {
		return false
	}

	m := o.mark()
	cv.renderLink(o, cv.last.text, cv.last.href, cv.last.hasImg)
	rendered := o.take(m)

	return string(o.buf.Bytes()[cv.last.start:cv.last.end]) == rendered
}

func onlySpace(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\n' && c != hardBreak[0] {
			return false
		}
	}

	return true
}

// renderLink writes an anchor according to the chosen style
func (cv *conversion) renderLink(o *output, text, href string, hasImg bool) {
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

		if text != "" {
			o.write(text)
			o.write(" ")
		}

		o.write("[")
		o.write(strconv.Itoa(cv.footnote(href)))
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

// footnote numbers a link target, reusing the number of a target seen before
// ponytail: linear scan, fine for the links in an email; index by map if documents get huge
func (cv *conversion) footnote(href string) int {
	if i := slices.Index(cv.links, href); i >= 0 {
		return i + 1
	}

	cv.links = append(cv.links, href)

	return len(cv.links)
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

	if cv.opts.links != LinksFootnotes && text == escapeMarkdown(href) && autolinks(href) {
		o.write("<" + href + ">")

		return
	}

	o.write("[")
	o.write(oneLine(text))

	if cv.opts.links == LinksFootnotes {
		o.write("][")
		o.write(strconv.Itoa(cv.footnote(href)))
		o.write("]")

		return
	}

	o.write("](")
	o.write(destination(href))
	o.write(")")
}

// autolinks reports whether href can be written as a CommonMark autolink, <href>,
// which allows no spaces, controls or angle brackets
func autolinks(href string) bool {
	return (hasScheme(href, "http://") || hasScheme(href, "https://")) &&
		!strings.ContainsFunc(href, func(r rune) bool { return r <= ' ' || r == '<' || r == '>' })
}

// sameTarget reports whether link text is just its href, give or take the scheme,
// a trailing slash and case
func sameTarget(text, href string) bool {
	trim := func(s string) string {
		s = strings.TrimSuffix(s, "/")
		if _, rest, ok := strings.Cut(s, "://"); ok {
			return rest
		}

		return s
	}

	return text != "" && strings.EqualFold(trim(text), trim(href))
}

func hasScheme(href, scheme string) bool {
	return len(href) >= len(scheme) && strings.EqualFold(href[:len(scheme)], scheme)
}

func contains(n *html.Node, a atom.Atom) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.DataAtom == a {
			return true
		}

		if contains(c, a) {
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
		if l := displayWidth(strings.TrimSpace(line)); l > maxSize {
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

// listNumber renders a number in the style the type attribute of an ol asks for.
// Digits stand in where that style has no form for the number, and in Markdown,
// which only numbers lists with digits.
func (cv *conversion) listNumber(idx int, kind string) string {
	switch {
	case cv.opts.markdown || idx <= 0:
	case kind == "a" || kind == "A":
		var letters []byte
		for n := idx; n > 0; n = (n - 1) / 26 {
			letters = append([]byte{byte('a' + (n-1)%26)}, letters...)
		}

		if kind == "A" {
			return strings.ToUpper(string(letters))
		}

		return string(letters)
	case (kind == "i" || kind == "I") && idx < 4000:
		var numeral strings.Builder
		for i, v := range []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1} {
			for ; idx >= v; idx -= v {
				numeral.WriteString([]string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}[i])
			}
		}

		if kind == "I" {
			return strings.ToUpper(numeral.String())
		}

		return numeral.String()
	}

	return strconv.Itoa(idx)
}

// listStart reads the start attribute of an ol, which may be negative
func listStart(n *html.Node, fallback int) int {
	// Atoi allocates its error, so the usual missing attribute is checked first
	if v := strings.TrimSpace(getAttr(n, "start")); v != "" {
		if start, err := strconv.Atoi(v); err == nil {
			return start
		}
	}

	return fallback
}

func (cv *conversion) listItems(o *output, n *html.Node, prefixer func(int) string) {
	idx, step := listStart(n, 1), 1

	// a reversed list counts down to one unless start says otherwise
	if n.DataAtom == atom.Ol && slices.ContainsFunc(n.Attr, func(a html.Attribute) bool { return a.Key == "reversed" }) {
		var items int

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.DataAtom == atom.Li && !cv.isHidden(c) {
				items++
			}
		}

		idx, step = listStart(n, items), -1
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && cv.isHidden(c) {
			continue
		}

		switch c.DataAtom {
		case atom.Li:
			cv.listItem(o, c, prefixer(idx))
			idx += step
		default:
			cv.doConvert(o, c)
		}
	}
}

// listItem converts an item in place, assembling it in the output's scratch
// buffer so that no level of nesting allocates a copy of its contents
func (cv *conversion) listItem(o *output, n *html.Node, prefix string) {
	m := o.mark()
	cv.doConvert(o, n)

	content := bytes.Trim(o.buf.Bytes()[m.length:], "\n")

	// everything after the first line came from a nested list and belongs one
	// level further in; marks already present deepen as they bubble up
	first, nested, _ := bytes.Cut(content, []byte("\n"))

	// Markdown nests under the item's content, so the indent covers the whole marker
	indent := indentMark
	if cv.opts.markdown {
		first = []byte(escapeLead(string(bytes.TrimLeft(first, " \t"))))
		indent = strings.Repeat(indentMark, (utf8.RuneCountInString(prefix)+1)/2)
	}

	out := append(append(o.scratch[:0], prefix...), first...)
	out = append(append(out[:0], bytes.TrimSpace(out)...), '\n')

	for line := range bytes.SplitSeq(nested, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		out = append(append(append(out, indent...), bytes.TrimLeft(line, " \t")...), '\n')
	}

	o.scratch = out
	o.replace(m, out)
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
			// converted in place, since copying it out costs a copy per level of
			// nesting. A block after any span breaks: text not ending in space
			// already ensures that, so only a space or an empty span is written,
			// leaving a span's own text as the last write for take to reuse.
			m := o.mark()
			cv.doConvert(o, c)

			// builders strike a former price with a style rather than <del>
			if struckByStyle(getAttr(c, "style")) {
				o.write(emphasized(o.take(m), "~~"))
			}

			switch {
			case o.trimTail(m):
				o.write(" ")
			case o.buf.Len() == m.length:
				o.write("")
			}

			continue
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

func fixSpacing(rt []byte) string {
	first, firstSize := utf8.DecodeRune(rt)
	if firstSize == 0 {
		return ""
	}

	second, secondSize := utf8.DecodeRune(rt[firstSize:])
	if secondSize == 0 {
		return string(rt)
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
		// after a space or a line break every further space or tab is dropped, and
		// a tab makes the pending space a line break, so indentation is skipped
		if c := rt[i]; (c == ' ' || c == '\t') && (isSpaceOrNBSP(last) || last == '\n') {
			for ; i < len(rt) && (rt[i] == ' ' || rt[i] == '\t'); i++ {
				if rt[i] == '\t' {
					last = '\n'
				}
			}

			continue
		}

		// ASCII other than spacing can't trigger any rule below, so a run of it is
		// copied whole, leaving its last byte pending
		if c := rt[i]; c > ' ' && c < utf8.RuneSelf {
			j := plainRun(rt, i+1)
			out.WriteRune(last)
			out.Write(rt[i : j-1])

			if j-i >= 2 {
				beforeLast = rune(rt[j-2])
			} else {
				beforeLast = last
			}

			last, i = rune(rt[j-1]), j

			continue
		}

		v, size := rune(rt[i]), 1
		if v >= utf8.RuneSelf {
			v, size = utf8.DecodeRune(rt[i:])
		}

		keep := true

		// a no-break space only means something between words, so at the edge of
		// a line or in a run of spaces it is spacing like any other
		switch {
		case last == '\n' && (v == '\t' || isSpaceOrNBSP(v)):
			keep = false
		case last == '\n' && beforeLast == '\n' && v == '\n':
			keep = false
		case isSpaceOrNBSP(last) && isSpaceOrNBSP(v):
			keep = false
		case isSpaceOrNBSP(last) && (v == '\t' || v == '\n'):
			last = '\n'
			keep = false
		}

		if keep && (!isPreheaderMark(v) || v == '\u200c' && joinsLetters(last, rt[i+size:])) {
			out.WriteRune(last)

			beforeLast, last = last, v
		}

		i += size
	}

	out.WriteRune(last)

	return out.String()
}

// plainRun returns the end of the run of printable ASCII, other than space,
// starting at i, eight bytes at a time while no byte is at or below a space or
// outside ASCII
func plainRun(s []byte, i int) int {
	for len(s)-i >= 8 {
		if w := word8(s, i); (w|(w-0x2121212121212121)&^w)&high != 0 {
			break
		}

		i += 8
	}

	for i < len(s) && s[i] > ' ' && s[i] < utf8.RuneSelf {
		i++
	}

	return i
}

func isSpaceOrNBSP(r rune) bool {
	return r == ' ' || r == ' '
}

func isPreheaderMark(r rune) bool {
	return r >= '­' && strings.ContainsRune("\u034f\u00ad\u2007\u200b\u200c\ufeff", r)
}

// joinsLetters reports whether a zero width non-joiner sits inside a word, as in
// Persian and Indic scripts, rather than padding a preheader
func joinsLetters(before rune, rest []byte) bool {
	after, _ := utf8.DecodeRune(rest)

	return unicode.In(before, unicode.L, unicode.M) && unicode.In(after, unicode.L, unicode.M)
}

// isHidden reports whether an element is kept out of the rendered message.
// Preheader text meant only for the inbox preview is the usual case.
func (cv *conversion) isHidden(n *html.Node) bool {
	if cv.opts.hiddenContent {
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
func struckByStyle(style string) bool {
	for style != "" {
		var declaration string

		declaration, style, _ = strings.Cut(style, ";")
		property, value, _ := strings.Cut(declaration, ":")

		if property = strings.TrimSpace(property); strings.EqualFold(property, "text-decoration") || strings.EqualFold(property, "text-decoration-line") {
			if strings.Contains(strings.ToLower(value), "line-through") {
				return true
			}
		}
	}

	return false
}

func hiddenByStyle(style string) bool {
	var zeroHeight, overflowHidden bool

	// every element of a built template carries a style, so nothing is lowered
	// or allocated for declarations that cannot hide it
	for style != "" {
		var declaration string

		declaration, style, _ = strings.Cut(style, ";")

		property, value, ok := strings.Cut(declaration, ":")
		if !ok {
			continue
		}

		property, value = strings.TrimSpace(property), cssValue(value)

		switch {
		case strings.EqualFold(property, "display"):
			if strings.EqualFold(value, "none") {
				return true
			}
		case strings.EqualFold(property, "visibility"):
			if strings.EqualFold(value, "hidden") || strings.EqualFold(value, "collapse") {
				return true
			}
		case strings.EqualFold(property, "opacity"):
			if isZeroValue(value) {
				return true
			}
		case strings.EqualFold(property, "max-height"):
			zeroHeight = isZeroValue(value)
		case strings.EqualFold(property, "overflow"), strings.EqualFold(property, "overflow-y"):
			overflowHidden = strings.EqualFold(value, "hidden")
		}
	}

	return zeroHeight && overflowHidden
}

// cssValue trims a declaration's value and any !important after it
func cssValue(value string) string {
	value = strings.TrimSpace(value)
	if n := len(value) - len("!important"); n >= 0 && strings.EqualFold(value[n:], "!important") {
		value = strings.TrimSpace(value[:n])
	}

	return value
}

// isZeroValue reports whether a css number or length is zero, with or without a unit,
// so that opacity:0.5 and font-size:0.9em are not mistaken for zero
func isZeroValue(value string) bool {
	size, err := strconv.ParseFloat(strings.TrimRight(strings.ToLower(value), "abcdefghijklmnopqrstuvwxyz%"), 64)

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
