package textplain

// LinkStyle selects how the target of an anchor is rendered.
type LinkStyle int

const (
	// LinksInline writes the target after the text, as "text ( url )".
	LinksInline LinkStyle = iota

	// LinksOmitted keeps only the text of a link.
	LinksOmitted

	// LinksFootnotes numbers each target and lists them at the end.
	LinksFootnotes
)

// Options control how a document is rendered. The zero value renders the same
// way Convert does, except that it does not wrap.
type Options struct {
	// LineLength wraps at this many characters. Zero or less does not wrap.
	LineLength int

	// Bullet prefixes unordered list items. Empty means "* ".
	Bullet string

	// OrderedSuffix follows the number on ordered list items. Empty means ". ".
	OrderedSuffix string

	// Links selects how anchor targets are rendered.
	Links LinkStyle

	// PlainHeadings leaves off the rule characters drawn around headings.
	PlainHeadings bool
}

func (o Options) bullet() string {
	if o.Bullet == "" {
		return "* "
	}

	return o.Bullet
}

func (o Options) orderedSuffix() string {
	if o.OrderedSuffix == "" {
		return ". "
	}

	return o.OrderedSuffix
}
