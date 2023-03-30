# Textplain

This project began as a port of the html_to_plaintext logic from github.com/premailer/premailer and applies the same basic set of rules for generating a text/plain copy of an email, given the text/html version

## Usage

```golang
myHTML := `<html><body>Hello World</body></html>`
myPlaintext := textplain.Convert(myHTML, textplain.DefaultLineLength)
```

By default it applies a word wrapping algorithm that is also supplied standalone.

```golang
wrapped := textplain.WordWrap("hello world, here is some text", 15)
```

## Options

Two plaintexters are supplied:

```golang
converter := textplain.NewTreeConverter()
```

Uses the `x/net/html` package to parse the supplied html into a tree, and performs a single-pass conversion to plaintext. This is the best performing option, and recommended for general usage.

The library still includes the older converter option

```golang
converter := textplain.NewRegexpConverter()
```

is the most "true to premailer" implementation, and uses regular expressions, which is largely problematic as it needs to both compile those regexps **and** regular expressions in the Go world use mutexes which limit concurrency
