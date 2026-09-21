package benchmarks

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
	"testing"

	jaytaylor "github.com/jaytaylor/html2text"
	k3a "github.com/k3a/html2text"
	"github.com/mailproto/textplain"
	"golang.org/x/net/html"
)

//go:embed corpus/*.html
var corpusFS embed.FS

type document struct {
	name string
	body string
}

type implementation struct {
	name string
	// wraps reports whether the implementation also word wraps its output,
	// which is extra work the others do not do by default
	wraps bool
	fn    func(string) (string, error)
}

// textNodesOnly is a floor: parse the document and concatenate its text nodes.
// Nothing useful comes out of it, but it shows what the parse alone costs.
func textNodesOnly(doc string) (string, error) {
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return "", err
	}

	var (
		sb   strings.Builder
		walk func(*html.Node)
	)

	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "style") {
				continue
			}

			walk(c)
		}
	}
	walk(root)

	return sb.String(), nil
}

var implementations = []implementation{
	{"textplain", true, func(d string) (string, error) { return textplain.Convert(d, textplain.DefaultLineLength) }},
	{"textplain_nowrap", false, func(d string) (string, error) { return textplain.Convert(d, 0) }},
	{"jaytaylor", false, func(d string) (string, error) { return jaytaylor.FromString(d) }},
	{"jaytaylor_pretty", false, func(d string) (string, error) {
		return jaytaylor.FromString(d, jaytaylor.Options{PrettyTables: true})
	}},
	{"k3a", false, func(d string) (string, error) { return k3a.HTML2Text(d), nil }},
	{"k3a_lists", false, func(d string) (string, error) {
		return k3a.HTML2TextWithOptions(d, k3a.WithListSupport()), nil
	}},
	{"parse_floor", false, textNodesOnly},
}

func corpus(tb testing.TB) []document {
	tb.Helper()

	entries, err := fs.Glob(corpusFS, "corpus/*.html")
	if err != nil {
		tb.Fatal(err)
	}

	sort.Strings(entries)

	docs := make([]document, 0, len(entries))
	for _, e := range entries {
		body, err := corpusFS.ReadFile(e)
		if err != nil {
			tb.Fatal(err)
		}

		name := strings.TrimSuffix(strings.TrimPrefix(e, "corpus/"), ".html")
		docs = append(docs, document{name: name, body: string(body)})
	}

	return docs
}

var sink string

func BenchmarkConvert(b *testing.B) {
	for _, doc := range corpus(b) {
		for _, impl := range implementations {
			b.Run(doc.name+"/"+impl.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(doc.body)))
				b.ResetTimer()

				for range b.N {
					out, err := impl.fn(doc.body)
					if err != nil {
						b.Fatal(err)
					}

					sink = out
				}
			})
		}
	}
}
