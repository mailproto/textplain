package textplain

import (
	"math/bits"
	"strings"
	"unicode"
	"unicode/utf8"
)

// WordWrap breaks each line at spaces, or between Chinese, Japanese and Korean
// characters, so that it fits within lineLength columns. Wide characters take
// two columns. Zero or less does not wrap.
func WordWrap(txt string, lineLength int) string {
	if lineLength <= 0 || fits(txt, lineLength) {
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

		// no rune is wider than its encoding, so a line of at most lineLength
		// bytes fits
		if len(line) <= lineLength {
			out.WriteString(line)
			continue
		}

		if containsWide(line) {
			wrapWide(&out, line, lineLength)
			continue
		}

		// without wide runes a column is a rune, so the horizon advances by runes while
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

// fits reports whether no line of txt is longer than lineLength bytes, so that
// nothing needs wrapping
func fits(txt string, lineLength int) bool {
	for {
		line, rest, more := strings.Cut(txt, "\n")
		if len(line) > lineLength {
			return false
		}

		if !more {
			return true
		}

		txt = rest
	}
}

// runeOffset returns the byte offset n runes past i, or len(s) if there are
// fewer. Runes are counted by their leading bytes, eight bytes at a time.
func runeOffset(s string, i, n int) int {
	for ; len(s)-i >= 8; i += 8 {
		word := word8(s, i)
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

// high has the top bit of every byte set; a word without any is ASCII
const high = 0x8080808080808080

// word8 loads the eight bytes at s[i:] as one little-endian word
func word8[T string | []byte](s T, i int) uint64 {
	_ = s[i+7]

	return uint64(s[i]) | uint64(s[i+1])<<8 | uint64(s[i+2])<<16 | uint64(s[i+3])<<24 |
		uint64(s[i+4])<<32 | uint64(s[i+5])<<40 | uint64(s[i+6])<<48 | uint64(s[i+7])<<56
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

// wrapWide wraps a line holding wide characters by columns. East Asian text has
// no spaces, so the line may also break beside a CJK character. A run with no
// break inside the width is left whole, as a long word is.
func wrapWide(out *strings.Builder, line string, lineLength int) {
	for {
		var (
			brk, width int
			prev       rune
			over       bool
		)

		for i, r := range line {
			if i > 0 && prev != ' ' {
				at := i
				switch {
				case r == ' ':
					at = keepBrackets(line, i)
				case !breaksBetween(prev, r):
					at = 0
				}

				if at > 0 {
					brk = at

					if over {
						break
					}
				}
			}

			width += runeWidth(r)
			if over = width > lineLength; over && brk > 0 {
				break
			}

			prev = r
		}

		if !over || brk == 0 {
			out.WriteString(line)

			return
		}

		out.WriteString(strings.TrimRight(line[:brk], " "))
		out.WriteByte('\n')

		line = strings.TrimLeft(line[brk:], " ")
	}
}

// breaksBetween reports whether a line may break between two runes with no
// space. Closing punctuation and small kana stay off the start of a line, and
// opening brackets off the end of one.
func breaksBetween(before, after rune) bool {
	return (isCJK(before) || isCJK(after)) &&
		!strings.ContainsRune("、。，．・：；！？）」』】〕〉》ー々ぁぃぅぇぉっゃゅょゎァィゥェォッャュョヮヵヶ", after) &&
		!strings.ContainsRune("（「『【〔〈《", before)
}

// isCJK covers Han, kana, Hangul and their punctuation and fullwidth forms
func isCJK(r rune) bool {
	return r >= 0x2e80 && r <= 0x9fff || r >= 0xac00 && r <= 0xd7af || r >= 0x1100 && r <= 0x11ff ||
		r >= 0xf900 && r <= 0xfaff || r >= 0xff00 && r <= 0xffef || r >= 0x20000 && r <= 0x3ffff
}

// containsWide skips ASCII eight bytes at a time and decodes only runes that
// could be wide: U+1100 to U+115F, or U+2E80 and up, which passes over the
// typographic punctuation between them without decoding it
func containsWide(s string) bool {
	for i := 0; i < len(s); {
		if len(s)-i >= 8 && word8(s, i)&high == 0 {
			i += 8

			continue
		}

		if c := s[i]; c < 0xe1 || i+1 < len(s) && (c == 0xe1 && s[i+1] < 0x84 || c == 0xe2 && s[i+1] < 0xba) {
			i++

			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if isWide(r) {
			return true
		}

		i += size
	}

	return false
}

// displayWidth is how many columns s takes in a monospaced font
func displayWidth(s string) int {
	var width int
	for _, r := range s {
		width += runeWidth(r)
	}

	return width
}

func runeWidth(r rune) int {
	switch {
	case r < 0x300:
		return 1
	case isWide(r):
		return 2
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		return 0
	}

	return 1
}

// ponytail: approximates East Asian Width and emoji presentation, and counts each
// part of an emoji sequence joined by U+200D; golang.org/x/text/width if exact
// widths matter
func isWide(r rune) bool {
	return r >= 0x1100 && r <= 0x115f || r >= 0x2e80 && r <= 0x303e || r >= 0x3041 && r <= 0x33ff ||
		r >= 0x3400 && r <= 0x4dbf || r >= 0x4e00 && r <= 0x9fff || r >= 0xa000 && r <= 0xa4cf ||
		r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff || r >= 0xfe30 && r <= 0xfe4f ||
		r >= 0xff00 && r <= 0xff60 || r >= 0xffe0 && r <= 0xffe6 || r >= 0x1f300 && r <= 0x1f64f ||
		r >= 0x1f900 && r <= 0x1f9ff || r >= 0x20000 && r <= 0x3fffd
}
