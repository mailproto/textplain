package textplain

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	ignoredHTML           = regexp.MustCompile(`(?ms)<!-- start text\/html -->.*?<!-- end text\/html -->`)
	imgAltDoubleQuotes    = regexp.MustCompile(`(?i)<img.+?alt=\"([^\"]*)\"[^>]*\>`)
	imgAltSingleQuotes    = regexp.MustCompile(`(?i)<img.+?alt=\'([^\']*)\'[^>]*\>`)
	links                 = regexp.MustCompile(`(?i)<a\s.*?href=["'](mailto:)?([^"']*)["'][^>]*>((.|\s)*?)<\/a>`)
	headerClose           = regexp.MustCompile(`(?i)(<\/h[1-6]>)`)
	headerBlock           = regexp.MustCompile(`(?i)[\s]*<h([1-6]+)[^>]*>[\s]*(.*)[\s]*<\/h[1-6]+>`)
	headerBlockBr         = regexp.MustCompile(`(?i)<br[\s]*\/?>`)
	headerBlockTags       = regexp.MustCompile(`(?i)<\/?[^>]*>`)
	wrapSpans             = regexp.MustCompile(`(?msi)(<\/span>)[\s]+(<span)`)
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
	fixWordWrappedParens  = regexp.MustCompile(`\(([ \n])(http[^)]+)([\n ])\)`)
)

// Defaults
const (
	DefaultLineLength = 65
)

// XXX: based on premailer/premailer@7c94e7a5a457b6710bada8186c6a41fccbfa08d1

func submatchReplace(text string, regex *regexp.Regexp, replace func(string, []int) string) string {
	var start int
	var finalText string
	for _, submatch := range regex.FindAllStringSubmatchIndex(text, -1) {
		finalText += text[start:submatch[0]] + replace(text, submatch)
		start = submatch[1]
	}
	return finalText + text[start:len(text)]
}

func eachElement(root *html.Node, callback func(n *html.Node) bool) {
	var iter func(*html.Node)
	iter = func(n *html.Node) {
		if n == nil {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if !callback(c) {
				return
			}
			iter(c)
		}
	}
	iter(root)
	callback(root)
}

// XXX: terrible but functional
func findElement(root *html.Node, element string) *html.Node {
	var found *html.Node
	eachElement(root, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == element {
			found = n
			return false
		}
		return true
	})

	return found
}

// Convert returns the text in UTF-8 format with all HTML tags removed
func Convert(document string, lineLength int) (string, error) {

	// Brutish way to get a fully formed html document
	doc, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return "", err
	}

	element := findElement(doc, "body")

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
	dropNonContentTags(element)

	// Reconstitute the cleaned HTML document for application
	// of plaintext-conversion logic
	var clean bytes.Buffer
	err = html.Render(&clean, element)
	if err != nil {
		return "", err
	}

	//  strip text ignored html. Useful for removing
	//  headers and footers that aren't needed in the
	//  text version
	txt := ignoredHTML.ReplaceAllString(string(clean.Bytes()), "")

	//  replace images with their alt attributes
	//  for img tags with "" for attribute quotes
	//  with or without closing tag
	//  eg. the following formats:
	//  <img alt="" />
	//  <img alt="">
	txt = submatchReplace(txt, imgAltDoubleQuotes, func(t string, submatch []int) string {
		return t[submatch[2]:submatch[3]]
	})

	//  for img tags with '' for attribute quotes
	//  with or without closing tag
	//  eg. the following formats:
	//  <img alt='' />
	//  <img alt=''>
	txt = submatchReplace(txt, imgAltSingleQuotes, func(t string, submatch []int) string {
		return t[submatch[2]:submatch[3]]
	})

	// links
	txt = submatchReplace(txt, links, func(t string, submatch []int) string {
		href, value := strings.TrimSpace(txt[submatch[4]:submatch[5]]), strings.TrimSpace(txt[submatch[6]:submatch[7]])
		var replace string
		if strings.ToLower(href) == strings.ToLower(value) {
			replace = value
		} else if value != "" {
			replace = fmt.Sprintf("%s ( %s )", value, href)
		}
		return replace
	})

	//  handle headings (H1-H6)
	txt = submatchReplace(txt, headerClose, func(t string, submatch []int) string {
		// move closing tags to new lines
		return "\n" + t[submatch[2]:submatch[3]]
	})

	txt = submatchReplace(txt, headerBlock, func(t string, submatch []int) string {
		headerLevel, _ := strconv.Atoi(t[submatch[2]:submatch[3]])
		headerText := t[submatch[4]:submatch[5]]

		headerText = headerBlockBr.ReplaceAllString(headerText, "\n")
		headerText = headerBlockTags.ReplaceAllString(headerText, "")

		// XXX: added this, seems to be necessary
		headerText = strings.TrimSpace(headerText)

		var maxLength int
		for _, line := range strings.Split(headerText, "\n") {
			if l := len(line); l > maxLength {
				maxLength = l
			}
		}

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
	})

	//  wrap spans
	txt = submatchReplace(txt, wrapSpans, func(t string, submatch []int) string {
		return fmt.Sprintf("%s %s", t[submatch[2]:submatch[3]], t[submatch[4]:submatch[5]])
	})

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

	txt = wordWrap(txt, lineLength)

	//  remove linefeeds (\r\n and \r -> \n)
	txt = lineFeeds.ReplaceAllString(txt, "\n")

	//  strip extra spaces
	txt = nonBreakingSpaces.ReplaceAllString(txt, " ")
	txt = extraSpaceStartOfLine.ReplaceAllString(txt, "\n")
	txt = extraSpaceEndOfLine.ReplaceAllString(txt, "\n")

	// no more than two consecutive newlines
	txt = consecutiveNewlines.ReplaceAllString(txt, "\n\n")

	//  the word messes up the parens
	txt = submatchReplace(txt, fixWordWrappedParens, func(t string, submatch []int) string {
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
	})

	return strings.TrimSpace(txt), nil
}

func wordWrap(text string, lineLength int) string {
	var final []string
	for _, line := range strings.Split(text, "\n") {
		if len(line) > lineLength {
			// XXX: cacheme
			lineBreakRegex := regexp.MustCompile(fmt.Sprintf(`(?ms)(.{1,%v})(\s+|$)`, lineLength))
			line = submatchReplace(line, lineBreakRegex, func(t string, submatch []int) string {
				return t[submatch[2]:submatch[3]] + "\n"
			})
		}
		final = append(final, line)
	}
	return strings.Join(final, "\n")
}
