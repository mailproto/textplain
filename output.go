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

	// floor is where the innermost open mark began, below which nothing may be
	// rewritten
	floor int

	// scratch is where text rewritten in place is assembled before going back
	scratch []byte
}

// maxPooled keeps one huge document from pinning its buffer in the pool
const maxPooled = 64 << 10

var outputs = sync.Pool{New: func() any { return new(output) }}

func newOutput() *output {
	return outputs.Get().(*output)
}

// release returns o to the pool; nothing may use it afterwards
func (o *output) release() {
	if o.buf.Cap() > maxPooled || cap(o.scratch) > maxPooled {
		return
	}

	o.buf.Reset()
	o.writes, o.last, o.floor = 0, "", 0
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

// mark records a position so that a subtree can be converted into the same
// buffer and then lifted back out as a string
type mark struct {
	length int
	writes int
	last   string
	floor  int
}

func (o *output) mark() mark {
	m := mark{length: o.buf.Len(), writes: o.writes, last: o.last, floor: o.floor}
	o.floor = m.length

	return m
}

// trimTail drops trailing whitespace written since the mark, reporting whether
// there was any, and closes the mark
func (o *output) trimTail(m mark) bool {
	o.floor = m.floor
	written := o.buf.Bytes()[m.length:]

	trimmed := bytes.TrimRight(written, "\n\t ")
	if len(trimmed) == len(written) {
		return false
	}

	o.buf.Truncate(m.length + len(trimmed))

	return true
}

// replace swaps everything written since the mark for b, counting it as one
// write, as if it had been taken and b written in its place
func (o *output) replace(m mark, b []byte) {
	o.floor = m.floor
	o.buf.Truncate(m.length)
	o.buf.Write(b)
	o.wrote(m)
}

// wrote counts everything written since the mark as a single write. Only
// whether that write ended a line matters to needsBreak, so last is set to
// stand in for it rather than copied out.
func (o *output) wrote(m mark) {
	o.writes = m.writes + 1
	o.last = ""

	if trimmed := bytes.Trim(o.buf.Bytes()[m.length:], " \t"); len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\n' {
		o.last = "\n"
	}
}

// take returns everything written since the mark and rewinds to it. When that
// is just the last string written, as for a link around one text node, the
// string is returned rather than copied.
func (o *output) take(m mark) string {
	taken := o.last
	if written := o.buf.Bytes()[m.length:]; string(written) != taken {
		taken = string(written)
	}

	o.buf.Truncate(m.length)
	o.writes, o.last, o.floor = m.writes, m.last, m.floor

	return taken
}
