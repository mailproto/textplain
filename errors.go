package textplain

import "errors"

var (
	ErrBodyNotFound = errors.New("could not find a `body` element in your html document")
)
