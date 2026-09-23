package textplain

import (
	"strings"
	"unicode/utf8"
)

// WordWrap breaks each line at spaces so that it fits within lineLength
// characters. Zero or less does not wrap.
func WordWrap(txt string, lineLength int) string {
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

			newIndex := keepBrackets(line[start:], strings.LastIndex(line[start:end+size], " "))
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

// keepBrackets moves a break at i so that a lone "(" never ends a line and a
// ")" never starts one, keeping link targets together
func keepBrackets(s string, i int) int {
	if i <= 0 {
		return i
	}

	if strings.HasPrefix(strings.TrimLeft(s[i:], " "), ")") {
		if i = strings.LastIndex(s[:i], " "); i <= 0 {
			return i
		}
	}

	if seg := s[:i]; seg == "(" || strings.HasSuffix(seg, " (") {
		i = len(strings.TrimRight(seg[:len(seg)-1], " "))
	}

	return i
}
