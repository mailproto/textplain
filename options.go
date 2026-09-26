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

// An Option changes how a document is rendered. Later options win.
type Option func(*options)

// options is deliberately unexported so that settings can be added without
// breaking callers.
type options struct {
	lineLength    int
	bullet        string
	orderedSuffix string
	links         LinkStyle
	plainHeadings bool
	markdown      bool
	hiddenContent bool
}

func newOptions(opts []Option) options {
	o := options{
		lineLength:    DefaultLineLength,
		bullet:        "* ",
		orderedSuffix: ". ",
	}

	for _, apply := range opts {
		apply(&o)
	}

	// wrapping could start a line with something that reads as markup
	if o.markdown {
		o.lineLength = 0
	}

	return o
}

// WithLineLength wraps at the given number of characters. Zero or less does
// not wrap.
func WithLineLength(chars int) Option {
	return func(o *options) { o.lineLength = chars }
}

// WithBullet sets the prefix on unordered list items, "* " by default.
func WithBullet(prefix string) Option {
	return func(o *options) { o.bullet = prefix }
}

// WithOrderedSuffix sets what follows the number on ordered list items,
// ". " by default.
func WithOrderedSuffix(suffix string) Option {
	return func(o *options) { o.orderedSuffix = suffix }
}

// WithLinks selects how anchor targets are rendered.
func WithLinks(style LinkStyle) Option {
	return func(o *options) { o.links = style }
}

// WithPlainHeadings leaves off the rule characters drawn around headings.
func WithPlainHeadings() Option {
	return func(o *options) { o.plainHeadings = true }
}

// WithHiddenContent keeps content that inline styles or attributes hide, such
// as preheaders.
func WithHiddenContent() Option {
	return func(o *options) { o.hiddenContent = true }
}

// WithMarkdown renders CommonMark instead of plain text. Output is not wrapped,
// so WithLineLength and WithPlainHeadings have no effect, and any WithBullet or
// WithOrderedSuffix must be a Markdown list marker.
func WithMarkdown() Option {
	return func(o *options) { o.markdown = true }
}
