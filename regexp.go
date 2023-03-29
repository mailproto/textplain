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

type RegexpConverter struct {
	ignoredHTML           *regexp.Regexp
	comments              *regexp.Regexp
	imgAltDoubleQuotes    submatchReplacer
	imgAltSingleQuotes    submatchReplacer
	links                 submatchReplacer
	headerClose           submatchReplacer
	headerBlockBr         *regexp.Regexp
	headerBlockTags       *regexp.Regexp
	headerBlock           submatchReplacer
	wrapSpans             submatchReplacer
	lists                 *regexp.Regexp
	listsNoNewline        *regexp.Regexp
	paragraphs            *regexp.Regexp
	lineBreaks            *regexp.Regexp
	remainingTags         *regexp.Regexp
	shortenSpaces         *regexp.Regexp
	lineFeeds             *regexp.Regexp
	nonBreakingSpaces     *regexp.Regexp
	extraSpaceStartOfLine *regexp.Regexp
	extraSpaceEndOfLine   *regexp.Regexp
	consecutiveNewlines   *regexp.Regexp
	fixWordWrappedParens  submatchReplacer
}

// New textplain converter object
func NewRegexpConverter() Converter {

	headerBlockBr := regexp.MustCompile(`(?i)<br[\s]*\/?>`)
	headerBlockTags := regexp.MustCompile(`(?i)<\/?[^>]*>`)

	return &RegexpConverter{
		ignoredHTML: regexp.MustCompile(`(?ms)<!-- start text\/html -->.*?<!-- end text\/html -->`),

		comments: regexp.MustCompile(`(?ms)<!--.*?-->`),

		// imgAltDoubleQuotes replaces images with their alt tag when it is double quoted
		imgAltDoubleQuotes: submatchReplacer{
			regexp: regexp.MustCompile(`(?i)<img.+?alt=\"([^\"]*)\"[^>]*\>`),
			handler: func(t string, submatch []int) string {
				return t[submatch[2]:submatch[3]]
			},
		},

		// imgAltSingleQuotes replaces images with their alt tag when it is single quoted
		imgAltSingleQuotes: submatchReplacer{
			regexp: regexp.MustCompile(`(?i)<img.+?alt=\'([^\']*)\'[^>]*\>`),
			handler: func(t string, submatch []int) string {
				return t[submatch[2]:submatch[3]]
			},
		},

		// links replaces anchor links with one of "href" or "content ( href )"
		links: submatchReplacer{
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
		},

		// headerClose moves `</h[1-6]>` tags to their own line as a preprocessing step for headerBlock
		headerClose: submatchReplacer{
			regexp: regexp.MustCompile(`(?i)(<\/h[1-6]>)`),
			handler: func(t string, submatch []int) string {
				return "\n" + t[submatch[2]:submatch[3]]
			},
		},

		// used in headerBlock to do some content replacement
		headerBlockBr:   headerBlockBr,
		headerBlockTags: headerBlockTags,

		// headerBlock converts a `<h[1-6]>` block to plaintext
		headerBlock: submatchReplacer{
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
		},

		// wrapSpans merges together contiguous span tags into a single line
		wrapSpans: submatchReplacer{
			regexp: regexp.MustCompile(`(?msi)(<\/span>)[\s]+(<span)`),
			handler: func(t string, submatch []int) string {
				return fmt.Sprintf("%s %s", t[submatch[2]:submatch[3]], t[submatch[4]:submatch[5]])
			},
		},

		// these are all used as direct replacements
		lists:                 regexp.MustCompile(`(?i)[\s]*(<li[^>]*>)[\s]*`),
		listsNoNewline:        regexp.MustCompile(`(?i)<\/li>[\s]*([\n]?)`),
		paragraphs:            regexp.MustCompile(`(?i)<\/p>`),
		lineBreaks:            regexp.MustCompile(`(?i)<br[\/ ]*>`),
		remainingTags:         regexp.MustCompile(`<\/?[^>]*>`),
		shortenSpaces:         regexp.MustCompile(` {2,}`),
		lineFeeds:             regexp.MustCompile(`\r\n?`),
		nonBreakingSpaces:     regexp.MustCompile(`[ \t]*\302\240+[ \t]*`),
		extraSpaceStartOfLine: regexp.MustCompile(`\n[ \t]+`),
		extraSpaceEndOfLine:   regexp.MustCompile(`[ \t]+\n`),
		consecutiveNewlines:   regexp.MustCompile(`[\n]{3,}`),

		// fixWordWrappedParens searches for links that got broken by word wrap and moves them
		// into a single line
		fixWordWrappedParens: submatchReplacer{
			regexp: regexp.MustCompile(`\(([ \n])([^)]+)([\n ])\)`),
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
		},
	}
}

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

// Convert returns a text-only version of supplied document in UTF-8 format with all HTML tags removed
func (t *RegexpConverter) Convert(document string, lineLength int) (string, error) {
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
	txt := t.ignoredHTML.ReplaceAllString(string(clean.Bytes()), "")

	//  strip out html comments
	txt = t.comments.ReplaceAllString(txt, "")

	//  replace images with their alt attributes for img tags with "" for attribute quotes
	//  eg. the following formats:
	//  <img alt="" />
	//  <img alt="">
	txt = t.imgAltDoubleQuotes.Replace(txt)

	//  replace images with their alt attributes for img tags with '' for attribute quotes
	//  eg. the following formats:
	//  <img alt='' />
	//  <img alt=''>
	txt = t.imgAltSingleQuotes.Replace(txt)

	// links
	txt = t.links.Replace(txt)

	//  handle headings (H1-H6)
	txt = t.headerClose.Replace(txt)
	txt = t.headerBlock.Replace(txt)

	//  wrap spans
	txt = t.wrapSpans.Replace(txt)

	//  lists -- TODO: should handle ordered lists
	txt = t.lists.ReplaceAllString(txt, "* ")

	//  list not followed by a newline
	txt = t.listsNoNewline.ReplaceAllString(txt, "\n")

	//  paragraphs and line breaks
	txt = t.paragraphs.ReplaceAllString(txt, "\n\n")
	txt = t.lineBreaks.ReplaceAllString(txt, "\n")

	//  strip remaining tags
	txt = t.remainingTags.ReplaceAllString(txt, "")

	//  decode HTML entities
	txt = html.UnescapeString(txt)

	//  no more than two consecutive spaces
	txt = t.shortenSpaces.ReplaceAllString(txt, " ")

	//  apply word wrapping
	txt = WordWrap(txt, lineLength)

	//  remove linefeeds (\r\n and \r -> \n)
	txt = t.lineFeeds.ReplaceAllString(txt, "\n")

	//  strip extra spaces
	txt = t.nonBreakingSpaces.ReplaceAllString(txt, " ")
	txt = t.extraSpaceStartOfLine.ReplaceAllString(txt, "\n")
	txt = t.extraSpaceEndOfLine.ReplaceAllString(txt, "\n")

	// no more than two consecutive newlines
	txt = t.consecutiveNewlines.ReplaceAllString(txt, "\n\n")

	//  wordWrap messes up the parens
	txt = t.fixWordWrappedParens.Replace(txt)

	return strings.TrimSpace(txt), nil
}
