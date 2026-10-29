package textplain

import "errors"

// ErrBodyNotFound is returned for a document with no body, such as a frameset.
var ErrBodyNotFound = errors.New("could not find a `body` element in your html document")

// ErrInternal is wrapped around a panic during conversion, a bug in this
// package, so that a caller can fall back rather than crash.
var ErrInternal = errors.New("textplain failed to convert the document")
