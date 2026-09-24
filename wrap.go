package textplain

import (
	"math/bits"
	"strings"
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
		if wrote {
			out.WriteByte('\n')
		}

		wrote = true

		// a line of at most lineLength bytes cannot hold more runes
		if len(line) <= lineLength {
			out.WriteString(line)
			continue
		}

		// lineLength counts characters, so the horizon advances by runes while
		// everything else stays in byte offsets and slices the original string
		var (
			start, end int
			// line[start:scanned] has been searched; its last space is at lastSpace
			scanned   int
			lastSpace = -1
		)

		for start < len(line) {
			next := runeOffset(line, end, lineLength)
			if next >= len(line) {
				break
			}

			// spaces skipped after a break may run past the horizon
			end = max(next, start)

			// a space is one byte, so only the byte at end can extend the window
			if i := strings.LastIndexByte(line[scanned:end+1], ' '); i >= 0 {
				lastSpace = scanned + i
			}

			scanned = end + 1

			newIndex := -1
			if lastSpace >= 0 {
				newIndex = keepBrackets(line[start:], lastSpace-start)
			}

			if newIndex <= 0 {
				continue
			}

			out.WriteString(line[start : start+newIndex])
			out.WriteByte('\n')

			start += newIndex
			end = start

			for start < len(line) && line[start] == ' ' {
				start++
			}

			scanned, lastSpace = start, -1
		}

		out.WriteString(line[start:])
	}

	return out.String()
}

// runeOffset returns the byte offset n runes past i, or len(s) if there are
// fewer. Runes are counted by their leading bytes, eight bytes at a time.
func runeOffset(s string, i, n int) int {
	const high = 0x8080808080808080

	for ; len(s)-i >= 8; i += 8 {
		w := s[i : i+8]
		word := uint64(w[0]) | uint64(w[1])<<8 | uint64(w[2])<<16 | uint64(w[3])<<24 |
			uint64(w[4])<<32 | uint64(w[5])<<40 | uint64(w[6])<<48 | uint64(w[7])<<56
		leading := 8
		if word&high != 0 {
			// continuation bytes are 10xxxxxx
			leading -= bits.OnesCount64(word & high &^ (word << 1))
		} else if n < 8 {
			return i + n
		}

		if leading > n {
			break
		}

		n -= leading
	}

	for ; i < len(s); i++ {
		if s[i]&0xC0 != 0x80 {
			if n == 0 {
				return i
			}

			n--
		}
	}

	return len(s)
}

// keepBrackets moves a break at i so that a lone "(" never ends a line and a
// ")" never starts one, keeping link targets together
func keepBrackets(s string, i int) int {
	j := i
	for j < len(s) && s[j] == ' ' {
		j++
	}

	if j < len(s) && s[j] == ')' {
		if i = strings.LastIndex(s[:i], " "); i <= 0 {
			return i
		}
	}

	if seg := s[:i]; seg == "(" || strings.HasSuffix(seg, " (") {
		i = len(strings.TrimRight(seg[:len(seg)-1], " "))
	}

	return i
}
