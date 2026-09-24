package textplain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// escapeMarkdown backslash-escapes text that would otherwise read as markup.
// Characters that are only markup at the start of a line get lineStartMark,
// which settleMarkdown resolves once lines are known.
// ponytail: & is not escaped, so literal entity text such as "&amp;copy;" renders as ©
func escapeMarkdown(s string) string {
	// a list number needs its . or ), so digits alone never need escaping
	if !strings.ContainsAny(s, "\\`*[_<#-+=>~.)") {
		return s
	}

	var out strings.Builder

	out.Grow(len(s) + 8)

	for i := 0; i < len(s); i++ {
		c := s[i]

		if i == 0 || s[i-1] == ' ' || s[i-1] == '\t' || s[i-1] == '\n' {
			switch {
			case strings.IndexByte("#-+=>~", c) >= 0:
				out.WriteString(lineStartMark)
			case c >= '0' && c <= '9':
				j := i
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}

				if j < len(s) && (s[j] == '.' || s[j] == ')') {
					out.WriteString(s[i:j])
					out.WriteString(lineStartMark)
					i = j
					c = s[i]
				}
			}
		}

		switch c {
		case '\\', '`', '*', '[':
			out.WriteByte('\\')
		case '_':
			if !intraword(s, i) {
				out.WriteByte('\\')
			}
		case '<':
			if i+1 < len(s) && (isASCIILetter(s[i+1]) || strings.IndexByte("/!?", s[i+1]) >= 0) {
				out.WriteByte('\\')
			}
		}

		out.WriteByte(c)
	}

	return out.String()
}

// intraword reports whether the byte at i sits between two letters or digits,
// where CommonMark never reads an underscore as emphasis
func intraword(s string, i int) bool {
	before, _ := utf8.DecodeLastRuneInString(s[:i])
	after, _ := utf8.DecodeRuneInString(s[i+1:])

	isWord := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

	return i > 0 && i+1 < len(s) && isWord(before) && isWord(after)
}

func isASCIILetter(c byte) bool {
	return (c|0x20) >= 'a' && (c|0x20) <= 'z'
}

// settleMarkdown resolves the markers that depend on where lines fall: an escape
// applies only at the start of a line, and a hard break only between two lines
// of text. Blank lines left behind by breaks are collapsed.
func settleMarkdown(text string) string {
	if !strings.Contains(text, lineStartMark) && !strings.Contains(text, hardBreak) {
		return text
	}

	strip := strings.NewReplacer(lineStartMark, "", hardBreak, "")
	lines := strings.Split(text, "\n")

	var (
		out   strings.Builder
		blank = true
	)

	for i, line := range lines {
		breaks := strings.HasSuffix(line, hardBreak)
		line = strip.Replace(escapeLead(line))

		if line == "" {
			if !blank {
				out.WriteByte('\n')
			}

			blank = true

			continue
		}

		if breaks && i+1 < len(lines) && hasText(lines[i+1]) {
			line = strings.TrimRight(line, " ") + `\`
		}

		out.WriteString(line)
		out.WriteByte('\n')

		blank = false
	}

	return out.String()
}

// escapeLead turns a lineStartMark at the start of a line, past any quote and
// indent markers and the digits of what would be a list number, into a backslash
func escapeLead(line string) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == quoteOpen[0] || line[i] == quoteClose[0] || line[i] == indentMark[0]) {
		i++
	}

	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}

	if strings.HasPrefix(line[i:], lineStartMark) {
		return line[:i] + `\` + line[i+len(lineStartMark):]
	}

	return line
}

// hasText reports whether a line holds anything other than markers and spaces
func hasText(line string) bool {
	return strings.ContainsFunc(line, func(r rune) bool { return r > ' ' })
}

// fenced wraps a preformatted block in a code fence longer than any run of
// backticks inside it
func fenced(block string) string {
	block = strings.Trim(block, "\n")
	fence := strings.Repeat("`", max(3, longestRun(block, '`')+1))

	return fence + "\n" + block + "\n" + fence
}

// codeSpan renders inline code, padding it when it starts or ends with a
// backtick so that the fence stays distinct
func codeSpan(code string) string {
	code = strings.ReplaceAll(code, "\n", " ")
	if strings.TrimSpace(code) == "" {
		return ""
	}

	fence := strings.Repeat("`", longestRun(code, '`')+1)
	if strings.HasPrefix(code, "`") || strings.HasSuffix(code, "`") {
		code = " " + code + " "
	}

	return fence + code + fence
}

func longestRun(s string, b byte) int {
	var longest, run int

	for i := range len(s) {
		if s[i] != b {
			run = 0

			continue
		}

		run++
		longest = max(longest, run)
	}

	return longest
}

// emphasized wraps text in delim, keeping surrounding whitespace outside it
// since "** x **" is not emphasis. Text spanning lines is left bare, as
// emphasis cannot cross blocks.
// ponytail: punctuation just inside the delimiters next to a letter outside, as in
// a**"b"**c, still defeats CommonMark's flanking rules
func emphasized(text, delim string) string {
	core := strings.TrimSpace(text)
	if core == "" || strings.Contains(core, "\n") {
		return text
	}

	start := strings.Index(text, core)

	return text[:start] + delim + core + delim + text[start+len(core):]
}

// destination makes a link target safe inside ( ), angle bracketing any with
// spaces or parentheses
func destination(href string) string {
	if strings.ContainsAny(href, "<>") {
		href = strings.NewReplacer("<", "%3C", ">", "%3E").Replace(href)
	}

	if strings.ContainsAny(href, " ()") {
		return "<" + href + ">"
	}

	return href
}

// oneLine collapses text onto a single line, for places Markdown cannot break
func oneLine(text string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(text, hardBreak, "")), " ")
}
