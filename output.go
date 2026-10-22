package textplain

import (
	"bytes"
	"strings"
	"sync"
)

// output accumulates converted text in one buffer rather than a slice of
// fragments. It remembers the last chunk written, because a block element
// decides whether to break on whatever was written immediately before it.
type output struct {
	buf    bytes.Buffer
	last   string
	writes int
}

// maxPooled keeps one huge document from pinning its buffer in the pool
const maxPooled = 64 << 10

var outputs = sync.Pool{New: func() any { return new(output) }}

func newOutput() *output {
	return outputs.Get().(*output)
}

// release returns o to the pool; nothing may use it afterwards
func (o *output) release() {
	if o.buf.Cap() > maxPooled {
		return
	}

	o.buf.Reset()
	o.writes, o.last = 0, ""
	outputs.Put(o)
}

func (o *output) write(s string) {
	o.writes++
	o.last = s
	o.buf.WriteString(s)
}

// needsBreak reports whether a block should start a line of its own
func (o *output) needsBreak() bool {
	if o.writes == 0 {
		return false
	}

	trimmed := strings.Trim(o.last, " \t")

	return len(trimmed) == 0 || trimmed[len(trimmed)-1] != '\n'
}

func (o *output) String() string {
	return o.buf.String()
}

// mark records a position so that a subtree can be converted into the same
// buffer and then lifted back out as a string
type mark struct {
	length int
	writes int
	last   string
}

func (o *output) mark() mark {
	return mark{length: o.buf.Len(), writes: o.writes, last: o.last}
}

// trimTail drops trailing whitespace written since the mark, reporting whether
// there was any
func (o *output) trimTail(m mark) bool {
	written := o.buf.Bytes()[m.length:]

	trimmed := bytes.TrimRight(written, "\n\t ")
	if len(trimmed) == len(written) {
		return false
	}

	o.buf.Truncate(m.length + len(trimmed))

	return true
}

// take returns everything written since the mark and rewinds to it
func (o *output) take(m mark) string {
	taken := string(o.buf.Bytes()[m.length:])

	o.buf.Truncate(m.length)
	o.writes, o.last = m.writes, m.last

	return taken
}
