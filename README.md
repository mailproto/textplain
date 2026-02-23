# Textplain

This project began as a port of the html_to_plaintext logic from [github.com/premailer/premailer](https://github.com/premailer/premailer) and applies the same basic set of rules for generating a text/plain copy of an email, given the text/html version.

## Usage

```golang
myHTML := `<html><body>Hello World</body></html>`
myPlaintext := textplain.Convert(myHTML, textplain.DefaultLineLength)
```

By default it applies a word wrapping algorithm that is also exported for use on its own.

```golang
wrapped := textplain.WordWrap("hello world, here is some text", 15)
```
