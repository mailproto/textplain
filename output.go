package textplain

import (
	"bytes"
	"strings"
)

// output accumulates converted text in one buffer rather than a slice of
// fragments. It remembers the last chunk written, because a block element
// decides whether to break on whatever was written immediately before it.
type output struct {
	buf    bytes.Buffer
	last   string
	writes int
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

// take returns everything written since the mark and rewinds to it
func (o *output) take(m mark) string {
	taken := string(o.buf.Bytes()[m.length:])

	o.buf.Truncate(m.length)
	o.writes, o.last = m.writes, m.last

	return taken
}

// writeAll writes each fragment in order.
func (o *output) writeAll(parts []string) {
	for _, p := range parts {
		o.write(p)
	}
}
