// textplain converts the HTML part of an email into a text/plain
// compatible format
package textplain

// Defaults.
const (
	DefaultLineLength = 65
)

type Converter interface {
	Convert(string, ...Option) (string, error)
}

var defaultConverter = NewTreeConverter()

// Convert is a wrapper around the default converter singleton.
// With no options it wraps at DefaultLineLength.
func Convert(document string, opts ...Option) (string, error) {
	return defaultConverter.Convert(document, opts...)
}
