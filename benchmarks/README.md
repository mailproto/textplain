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
