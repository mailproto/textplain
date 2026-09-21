# Textplain

This project began as a port of the html_to_plaintext logic from [github.com/premailer/premailer](https://github.com/premailer/premailer) and applies the same basic set of rules for generating a text/plain copy of an email, given the text/html version.

## Install

```bash
go get github.com/mailproto/textplain
```

## Usage

```go
myHTML := `<html><body>Hello World</body></html>`

myPlaintext, err := textplain.Convert(myHTML, textplain.DefaultLineLength)
if err != nil {
	// ErrBodyNotFound if the document has no body element
}
```

`DefaultLineLength` is 65. The word wrapping is exported for use on its own, and counts characters rather than bytes:

```go
wrapped := textplain.WordWrap("hello world, here is some text", 15)
```

Pass a line length of zero or less to skip wrapping entirely.
