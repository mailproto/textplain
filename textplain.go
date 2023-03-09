package textplain

import (
	"errors"
)

// Defaults
const (
	DefaultLineLength = 65
)

// Well-defined errors
var (
	ErrBodyNotFound = errors.New("could not find a `body` element in your html document")
)

var defaultConverter = NewRegexpConverter()

// Convert is a convenience method so the library can be used without initializing a converter
// because this library relies heavily on regexp objects, it may act as a bottlneck to concurrency
// due to thread-safety mutexes in *regexp.Regexp internals
func Convert(document string, lineLength int) (string, error) {
	return defaultConverter.Convert(document, lineLength)
}

func MustConvert(document string, lineLength int) string {
	result, _ := Convert(document, lineLength)
	return result
}
