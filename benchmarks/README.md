# Benchmarks

A separate module so that comparison dependencies stay out of `go.mod` for the library itself.

```bash
cd benchmarks
go test -bench BenchmarkConvert -benchmem .
go test -run TestFidelity -v .    # what each implementation actually produces
```

`parse_floor` is not a converter. It parses the document and concatenates its text nodes, which
is roughly the least work any parser-based implementation could do, and is there to show how much
of the runtime belongs to `html.Parse` rather than to conversion.

## Other languages

Libraries in other languages run as long-lived workers under `testdata/`, fed documents over
stdin, so their timings exclude process startup but include a pipe round trip of roughly
10–30µs per document. Any worker that is not set up is skipped with a message.

| implementation | library | setup, from `testdata/` |
| --- | --- | --- |
| `node_html_to_text` | [html-to-text](https://github.com/html-to-text/node-html-to-text) | `(cd node && npm ci)` |
| `python_inscriptis` | [inscriptis](https://github.com/weblyzard/inscriptis) | `python3 -m venv python/.venv && python/.venv/bin/pip install -r python/requirements.txt` |
| `rust_html2text` | [html2text](https://github.com/jugglerchris/rust-html2text) | `(cd rust && cargo build --release)` |

Where the library can wrap, it wraps at 65 like `textplain`.
