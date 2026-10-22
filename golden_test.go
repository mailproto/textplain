package textplain_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/golden")

// TestGolden pins the output for each benchmark corpus document, so that a
// change meant only to be faster shows any difference in output as a diff.
func TestGolden(t *testing.T) {
	variants := map[string][]textplain.Option{
		"plain":              nil,
		"width20":            {textplain.WithLineLength(20)},
		"footnotes":          {textplain.WithLinks(textplain.LinksFootnotes)},
		"hidden":             {textplain.WithHiddenContent()},
		"markdown":           {textplain.WithMarkdown()},
		"markdown_footnotes": {textplain.WithMarkdown(), textplain.WithLinks(textplain.LinksFootnotes)},
	}

	docs, err := filepath.Glob("benchmarks/corpus/*.html")
	require.NoError(t, err)
	require.NotEmpty(t, docs)

	for _, doc := range docs {
		body, err := os.ReadFile(doc)
		require.NoError(t, err)

		for variant, opts := range variants {
			name := strings.TrimSuffix(filepath.Base(doc), ".html") + "." + variant
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				result, err := textplain.Convert(string(body), opts...)
				require.NoError(t, err)

				golden := filepath.Join("testdata", "golden", name+".txt")
				if *update {
					require.NoError(t, os.WriteFile(golden, []byte(result), 0o644))
				}

				want, err := os.ReadFile(golden)
				require.NoError(t, err, "run go test -run TestGolden -update to create it")
				assert.Equal(t, string(want), result)
			})
		}
	}
}
