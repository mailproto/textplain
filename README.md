# Textplain

This project began as a port of the html_to_plaintext logic from [github.com/premailer/premailer](https://github.com/premailer/premailer) and applies the same basic set of rules for generating a text/plain copy of an email, given the text/html version.

## Install

```bash
go get github.com/mailproto/textplain
```

## Usage

```go
myHTML := `<html><body>Hello World</body></html>`

myPlaintext, err := textplain.Convert(myHTML)
if err != nil {
	// ErrBodyNotFound if the document has no body element
}
```

`ConvertReader` does the same for an `io.Reader`, such as a MIME part, and also returns any error from reading it.

Output wraps at `DefaultLineLength`, which is 65. The word wrapping is exported for use on its own, and counts characters rather than bytes:

```go
wrapped := textplain.WordWrap("hello world, here is some text", 15)
```

Pass a line length of zero or less to skip wrapping entirely.

## Options

`Convert` takes any number of options after the document.

```go
myPlaintext, err := textplain.Convert(myHTML,
	textplain.WithLinks(textplain.LinksFootnotes),
	textplain.WithPlainHeadings(),
)
```

| option | default |
| --- | --- |
| `WithLineLength(chars)` | `DefaultLineLength`; zero or less does not wrap |
| `WithBullet(prefix)` | `"* "` on unordered list items |
| `WithOrderedSuffix(suffix)` | `". "` after the number on ordered list items |
| `WithLinks(style)` | `LinksInline` |
| `WithPlainHeadings()` | off, so headings are drawn with rule characters |
| `WithMarkdown()` | off; renders CommonMark instead of plain text |
| `WithHiddenContent()` | off, so content hidden by inline styles or attributes is dropped |

Later options win, so a caller can layer its own on top of a shared set.

Hidden content is judged from inline styles, so a layout can occasionally convert to an empty string. Callers that need text can retry with `WithHiddenContent()`.

`WithLinks` takes one of three styles:

| style | `<a href="https://example.com">Docs</a>` becomes |
| --- | --- |
| `LinksInline` | `Docs ( https://example.com )` |
| `LinksOmitted` | `Docs` |
| `LinksFootnotes` | `Docs [1]`, with the targets listed under the body |

`LinksFootnotes` numbers each target in the order it appears:

```
Read the Docs [1] or the changelog [2].

[1] https://example.com
[2] https://example.com/changelog
```

## Markdown

`WithMarkdown` renders the same content as CommonMark: `#` headings, `**bold**` and `*italic*`, code spans and fences, `![alt](src)` images and `[text](url)` links, or reference links with `LinksFootnotes`. Text that would otherwise read as markup is escaped. Layout tables are flattened as in plain text, with each row ending in a hard break.

Markdown output is never wrapped, so `WithLineLength` and `WithPlainHeadings` have no effect.
