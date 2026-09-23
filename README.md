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

Later options win, so a caller can layer its own on top of a shared set.

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
