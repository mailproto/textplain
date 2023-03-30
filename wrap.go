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
	for _, line := range strings.Split(txt, "\n") {
		var startIndex, endIndex int
		for (len(line)-endIndex) > lineLength && startIndex < len(line) {
			endIndex += lineLength
			if endIndex >= len(line) {
				endIndex = len(line) - 1
			}

			newIndex := strings.LastIndex(line[startIndex:endIndex+1], " ")
			if newIndex <= 0 {
				continue
			}

			final = append(final, line[startIndex:startIndex+newIndex])
			startIndex += newIndex

			// clear any extra space
			for ; startIndex < len(line) && line[startIndex] == ' '; startIndex++ {
			}
		}
		final = append(final, line[startIndex:])
	}

	return strings.Join(final, "\n")
}
