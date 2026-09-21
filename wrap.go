package textplain

import (
	"strings"
	"unicode/utf8"
)

// WordWrap searches for logical breakpoints in each line (whitespace) and tries to trim each
// line to the specified length
// Note: this diverges from the regex approach in premailer, which I found to be significantly
// slower in cases with long unbroken lines
// https://github.com/premailer/premailer/blob/7c94e7a/lib/premailer/html_to_plain_text.rb#L116
func WordWrap(txt string, lineLength int) string {
	// A line length of zero or less indicates no wrapping
	if lineLength <= 0 {
		return txt
	}

	var (
		out   strings.Builder
		wrote bool
	)

	out.Grow(len(txt) + len(txt)/lineLength + 1)

	for line := range strings.SplitSeq(txt, "\n") {
		total := utf8.RuneCountInString(line)

		// lineLength counts characters, so the horizon advances by runes while
		// everything else stays in byte offsets and slices the original string
		var (
			start, startRune int
			end, endRune     int
		)

		for total-endRune > lineLength && startRune < total {
			target := endRune + lineLength
			if target >= total {
				target = total - 1
			} else if target < startRune {
				target = startRune
			}

			for endRune < target {
				_, size := utf8.DecodeRuneInString(line[end:])
				end += size
				endRune++
			}

			_, size := utf8.DecodeRuneInString(line[end:])

			newIndex := strings.LastIndex(line[start:end+size], " ")
			if newIndex <= 0 {
				continue
			}

			if wrote {
				out.WriteByte('\n')
			}

			out.WriteString(line[start : start+newIndex])
			wrote = true

			prev := start
			start += newIndex
			endRune = startRune + utf8.RuneCountInString(line[prev:start])
			end, startRune = start, endRune

			// clear any extra space
			for ; start < len(line) && line[start] == ' '; start++ {
				startRune++
			}
		}

		if wrote {
			out.WriteByte('\n')
		}

		out.WriteString(line[start:])
		wrote = true
	}

	return out.String()
}
