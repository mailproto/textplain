package textplain

// Defaults.
const (
	DefaultLineLength = 65
)

type Converter interface {
	Convert(string, int) (string, error)
	ConvertWithOptions(string, ...Option) (string, error)
}

var defaultConverter = NewTreeConverter()

// Convert is a wrapper around the default converter singleton.
func Convert(document string, lineLength int) (string, error) {
	return defaultConverter.Convert(document, lineLength)
}

// ConvertWithOptions is a wrapper around the default converter singleton.
// With no options it wraps at DefaultLineLength.
func ConvertWithOptions(document string, opts ...Option) (string, error) {
	return defaultConverter.ConvertWithOptions(document, opts...)
}
