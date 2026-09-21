package textplain

// Defaults.
const (
	DefaultLineLength = 65
)

type Converter interface {
	Convert(string, int) (string, error)
	ConvertWithOptions(string, Options) (string, error)
}

var defaultConverter = NewTreeConverter()

// Convert is a wrapper around the default converter singleton.
func Convert(document string, lineLength int) (string, error) {
	return defaultConverter.Convert(document, lineLength)
}

// ConvertWithOptions is a wrapper around the default converter singleton.
func ConvertWithOptions(document string, opts Options) (string, error) {
	return defaultConverter.ConvertWithOptions(document, opts)
}
