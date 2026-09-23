package textplain

import "errors"

// ErrBodyNotFound is returned for a document with no body, such as a frameset.
var ErrBodyNotFound = errors.New("could not find a `body` element in your html document")
