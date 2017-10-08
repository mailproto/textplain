package textplain

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Defaults
const (
	DefaultLineLength = 65
)

// Well-defined errors
var (
	ErrBodyNotFound = errors.New("Could not find a `body` element in your html document")
)

var (
	ignoredHTML = regexp.MustCompile(`(?ms)<!-- start text\/html -->.*?<!-- end text\/html -->`)

	comments = regexp.MustCompile(`(?ms)<!--.*?-->`)

	// imgAltDoubleQuotes replaces images with their alt tag when it is double quoted
	imgAltDoubleQuotes = submatchReplacer{
		regexp: regexp.MustCompile(`(?i)<img.+?alt=\"([^\"]*)\"[^>]*\>`),
		handler: func(t string, submatch []int) string {
			return t[submatch[2]:submatch[3]]
		},
	}

	// imgAltSingleQuotes replaces images with their alt tag when it is single quoted
	imgAltSingleQuotes = submatchReplacer{
		regexp: regexp.MustCompile(`(?i)<img.+?alt=\'([^\']*)\'[^>]*\>`),
		handler: func(t string, submatch []int) string {
			return t[submatch[2]:submatch[3]]
		},
	}

	// links replaces anchor links with one of "href" or "content ( href )"
	links = submatchReplacer{
		regexp: regexp.MustCompile(`(?i)<a\s.*?href=["'](mailto:)?([^"']*)["'][^>]*>((.|\s)*?)<\/a>`),
		handler: func(t string, submatch []int) string {
			href, value := strings.TrimSpace(t[submatch[4]:submatch[5]]), strings.TrimSpace(t[submatch[6]:submatch[7]])
			var replace string
			if strings.ToLower(href) == strings.ToLower(value) {
				replace = value
			} else if value != "" {
				replace = fmt.Sprintf("%s ( %s )", value, href)
			}
			return replace
		},
	}

	// headerClose moves `</h[1-6]>` tags to their own line as a preprocessing step for headerBlock
	headerClose = submatchReplacer{
		regexp: regexp.MustCompile(`(?i)(<\/h[1-6]>)`),
		handler: func(t string, submatch []int) string {
			return "\n" + t[submatch[2]:submatch[3]]
		},
	}

	// used in headerBlock to do some content replacement
	headerBlockBr   = regexp.MustCompile(`(?i)<br[\s]*\/?>`)
	headerBlockTags = regexp.MustCompile(`(?i)<\/?[^>]*>`)

	// headerBlock converts a `<h[1-6]>` block to plaintext
	headerBlock = submatchReplacer{
		regexp: regexp.MustCompile(`(?imsU)[\s]*<h([1-6]+)[^>]*>[\s]*(.*)[\s]*<\/h[1-6]+>`),
		handler: func(t string, submatch []int) string {
			headerLevel, _ := strconv.Atoi(t[submatch[2]:submatch[3]])
			headerText := t[submatch[4]:submatch[5]]

			headerText = headerBlockBr.ReplaceAllString(headerText, "\n")
			headerText = headerBlockTags.ReplaceAllString(headerText, "")

			var maxLength int
			var headerLines []string
			for _, line := range strings.Split(headerText, "\n") {
				if trimmed := strings.TrimSpace(line); len(trimmed) > 0 {
					headerLines = append(headerLines, trimmed)
					if l := len(headerLines[len(headerLines)-1]); l > maxLength {
						maxLength = l
					}
				}
			}

			headerText = strings.Join(headerLines, "\n")
			var header string

			// special case headers
			switch headerLevel {
			case 1:
				header = strings.Repeat("*", maxLength) + "\n" + headerText + "\n" + strings.Repeat("*", maxLength)
			case 2:
				header = strings.Repeat("-", maxLength) + "\n" + headerText + "\n" + strings.Repeat("-", maxLength)
			default:
				header = headerText + "\n" + strings.Repeat("-", maxLength)
			}

			return "\n\n" + header + "\n\n"
		},
	}

	// wrapSpans merges together contiguous span tags into a single line
	wrapSpans = submatchReplacer{
		regexp: regexp.MustCompile(`(?msi)(<\/span>)[\s]+(<span)`),
		handler: func(t string, submatch []int) string {
			return fmt.Sprintf("%s %s", t[submatch[2]:submatch[3]], t[submatch[4]:submatch[5]])
		},
	}

	// these are all used as direct replacements
	lists                 = regexp.MustCompile(`(?i)[\s]*(<li[^>]*>)[\s]*`)
	listsNoNewline        = regexp.MustCompile(`(?i)<\/li>[\s]*([\n]?)`)
	paragraphs            = regexp.MustCompile(`(?i)<\/p>`)
	lineBreaks            = regexp.MustCompile(`(?i)<br[\/ ]*>`)
	remainingTags         = regexp.MustCompile(`<\/?[^>]*>`)
	shortenSpaces         = regexp.MustCompile(` {2,}`)
	lineFeeds             = regexp.MustCompile(`\r\n?`)
	nonBreakingSpaces     = regexp.MustCompile(`[ \t]*\302\240+[ \t]*`)
	extraSpaceStartOfLine = regexp.MustCompile(`\n[ \t]+`)
	extraSpaceEndOfLine   = regexp.MustCompile(`[ \t]+\n`)
	consecutiveNewlines   = regexp.MustCompile(`[\n]{3,}`)

	// fixWordWrappedParens searches for links that got broken by word wrap and moves them
	// into a single line
	fixWordWrappedParens = submatchReplacer{
		regexp: regexp.MustCompile(`\(([ \n])(http[^)]+)([\n ])\)`),
		handler: func(t string, submatch []int) string {
			leadingSpace, content, trailingSpace := t[submatch[2]:submatch[3]], t[submatch[4]:submatch[5]], t[submatch[6]:submatch[7]]
			var out string
			if leadingSpace == "\n" {
				out += leadingSpace
			}
			out += "( " + content + " )"
			if trailingSpace == "\n" {
				out += leadingSpace
			}
			return out
		},
	}
)

// XXX: based on premailer/premailer@7c94e7a5a457b6710bada8186c6a41fccbfa08d1
// https://github.com/premailer/premailer/tree/7c94e7a5a457b6710bada8186c6a41fccbfa08d1

type submatchReplacer struct {
	regexp  *regexp.Regexp
	handler func(string, []int) string
}

func (s *submatchReplacer) Replace(text string) string {
	var start int
	var finalText string
	for _, submatch := range s.regexp.FindAllStringSubmatchIndex(text, -1) {
		finalText += text[start:submatch[0]] + s.handler(text, submatch)
		start = submatch[1]
	}
	return finalText + text[start:len(text)]
}

// Convert returns the text in UTF-8 format with all HTML tags removed
func Convert(document string, lineLength int) (string, error) {

	// Brutish way to get a fully formed html document
	doc, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return "", err
	}

	// Find the <body> tag within the document
	var bodyElement *html.Node
	if doc.Type == html.ElementNode && doc.Data == "body" {
		bodyElement = doc
	} else {
		var scanForBody func(n *html.Node, depth int)
		scanForBody = func(n *html.Node, depth int) {
			if n == nil {
				return
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if n.Type == html.ElementNode && n.Data == "body" {
					bodyElement = n
					return
				}
				if depth < 5 {
					scanForBody(c, depth+1)
				}
			}
		}
		scanForBody(doc, 0)
	}
	if bodyElement == nil {
		return "", ErrBodyNotFound
	}

	var dropNonContentTags func(*html.Node)
	dropNonContentTags = func(n *html.Node) {
		if n == nil {
			return
		}
		var toRemove []*html.Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.DataAtom == atom.Script || c.DataAtom == atom.Style {
				toRemove = append(toRemove, c)
			} else {
				dropNonContentTags(c)
			}
		}
		for _, r := range toRemove {
			n.RemoveChild(r)
		}
	}
	dropNonContentTags(bodyElement)

	// Reconstitute the cleaned HTML document for application
	// of plaintext-conversion logic
	var clean bytes.Buffer
	err = html.Render(&clean, bodyElement)
	if err != nil {
		return "", err
	}

	//  strip text ignored html. Useful for removing
	//  headers and footers that aren't needed in the
	//  text version
	txt := ignoredHTML.ReplaceAllString(string(clean.Bytes()), "")

	//  strip out html comments
	txt = comments.ReplaceAllString(txt, "")

	//  replace images with their alt attributes for img tags with "" for attribute quotes
	//  eg. the following formats:
	//  <img alt="" />
	//  <img alt="">
	txt = imgAltDoubleQuotes.Replace(txt)

	//  replace images with their alt attributes for img tags with '' for attribute quotes
	//  eg. the following formats:
	//  <img alt='' />
	//  <img alt=''>
	txt = imgAltSingleQuotes.Replace(txt)

	// links
	txt = links.Replace(txt)

	//  handle headings (H1-H6)
	txt = headerClose.Replace(txt)
	txt = headerBlock.Replace(txt)

	//  wrap spans
	txt = wrapSpans.Replace(txt)

	//  lists -- TODO: should handle ordered lists
	txt = lists.ReplaceAllString(txt, "* ")

	//  list not followed by a newline
	txt = listsNoNewline.ReplaceAllString(txt, "\n")

	//  paragraphs and line breaks
	txt = paragraphs.ReplaceAllString(txt, "\n\n")
	txt = lineBreaks.ReplaceAllString(txt, "\n")

	//  strip remaining tags
	txt = remainingTags.ReplaceAllString(txt, "")

	//  decode HTML entities
	txt = html.UnescapeString(txt)

	//  no more than two consecutive spaces
	txt = shortenSpaces.ReplaceAllString(txt, " ")

	//  apply word wrapping
	txt = WordWrap(txt, lineLength)

	//  remove linefeeds (\r\n and \r -> \n)
	txt = lineFeeds.ReplaceAllString(txt, "\n")

	//  strip extra spaces
	txt = nonBreakingSpaces.ReplaceAllString(txt, " ")
	txt = extraSpaceStartOfLine.ReplaceAllString(txt, "\n")
	txt = extraSpaceEndOfLine.ReplaceAllString(txt, "\n")

	// no more than two consecutive newlines
	txt = consecutiveNewlines.ReplaceAllString(txt, "\n\n")

	//  wordWrap messes up the parens
	txt = fixWordWrappedParens.Replace(txt)

	return strings.TrimSpace(txt), nil
}

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
		for (len(line) - endIndex) > lineLength {
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
		}
		final = append(final, line[startIndex:])
	}

	return strings.Join(final, "\n")
}
