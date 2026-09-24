package benchmarks

import (
	"fmt"
	"strings"
	"testing"
)

// TestFidelity is not an assertion; it prints what each implementation actually
// produces so the benchmark numbers can be read with feature coverage in mind.
func TestFidelity(t *testing.T) {
	probes := []struct{ label, html string }{
		{"link", `<a href="https://example.com/x">Click</a>`},
		{"image alt", `<img alt="A picture" src="p.png"/>`},
		{"heading", `<h1>Title</h1><p>body</p>`},
		{"ul", `<ul><li>a</li><li>b</li></ul>`},
		{"ol", `<ol><li>a</li><li>b</li></ol>`},
		{"nested ul", `<ul><li>top<ul><li>child</li></ul></li></ul>`},
		{"data table", `<table><tr><td>Jan</td><td>Feb</td></tr><tr><td>1</td><td>2</td></tr></table>`},
		{"inline markup", `<p><b>bold</b>, <i>italic</i> and <code>x()</code></p>`},
		{"line break", `<p>line one<br>line two</p>`},
		{"literal markdown", `<p>1. not a list, *not* _emphasis_ [x] # no</p>`},
		{"blockquote", `<blockquote>quoted</blockquote><p>after</p>`},
		{"pre", "<pre>a\n  b</pre>"},
		{"hr", `<p>a</p><hr/><p>b</p>`},
		{"entities", `<p>AT&amp;T &copy; caf&eacute;</p>`},
		{"display none", `<p>shown</p><p style="display:none">hidden</p>`},
		{"malformed", `<p>unclosed <b>bold <i>italic</p><div>next`},
		{"script/style", `<style>.a{color:red}</style><script>var x=1</script><p>only this</p>`},
	}

	for _, p := range probes {
		fmt.Printf("\n=== %s === %s\n", p.label, p.html)
		for _, impl := range implementations {
			if impl.name == "parse_floor" || strings.HasSuffix(impl.name, "_nowrap") {
				continue
			}

			out, err := impl.fn(p.html)
			if err != nil {
				fmt.Printf("  %-26s ERROR %v\n", impl.name, err)

				continue
			}

			fmt.Printf("  %-26s %q\n", impl.name, strings.TrimSpace(out))
		}
	}
}
