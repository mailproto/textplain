package textplain

import "strings"

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

	var final []string

	for line := range strings.SplitSeq(txt, "\n") {
		// lineLength counts characters, so index by rune rather than by byte
		runes := []rune(line)

		var startIndex, endIndex int
		for (len(runes)-endIndex) > lineLength && startIndex < len(runes) {
			endIndex += lineLength
			if endIndex >= len(runes) {
				endIndex = len(runes) - 1
			} else if endIndex < startIndex {
				endIndex = startIndex
			}

			newIndex := lastSpace(runes[startIndex : endIndex+1])
			if newIndex <= 0 {
				continue
			}

			final = append(final, string(runes[startIndex:startIndex+newIndex]))
			startIndex += newIndex
			endIndex = startIndex

			// clear any extra space
			for ; startIndex < len(runes) && runes[startIndex] == ' '; startIndex++ {
			}
		}

		final = append(final, string(runes[startIndex:]))
	}

	return strings.Join(final, "\n")
}

func lastSpace(runes []rune) int {
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == ' ' {
			return i
		}
	}

	return -1
}
