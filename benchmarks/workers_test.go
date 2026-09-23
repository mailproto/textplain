package benchmarks

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Libraries in other languages run as long-lived workers so that timings exclude
// process startup. Each frame is a big-endian uint32 length followed by that many bytes.
var workers = []struct {
	name  string
	wraps bool
	cmd   []string
}{
	{"node_html_to_text", true, []string{"node", "testdata/node/worker.mjs"}},
	{"python_inscriptis", false, []string{"testdata/python/.venv/bin/python", "testdata/python/worker.py"}},
	{"rust_html2text", true, []string{"testdata/rust/target/release/worker"}},
}

type worker struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    *bufio.Reader
	stderr *bytes.Buffer
}

func startWorker(args []string) (*worker, error) {
	w := &worker{cmd: exec.Command(args[0], args[1:]...), stderr: &bytes.Buffer{}}
	w.cmd.Stderr = w.stderr

	in, err := w.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	out, err := w.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	w.in, w.out = in, bufio.NewReader(out)

	if err := w.cmd.Start(); err != nil {
		return nil, err
	}

	if _, err := w.convert("<p>ready</p>"); err != nil {
		w.stop()

		return nil, err
	}

	return w, nil
}

func (w *worker) convert(doc string) (string, error) {
	if err := binary.Write(w.in, binary.BigEndian, uint32(len(doc))); err != nil {
		return "", w.failed(err)
	}

	if _, err := io.WriteString(w.in, doc); err != nil {
		return "", w.failed(err)
	}

	var n uint32
	if err := binary.Read(w.out, binary.BigEndian, &n); err != nil {
		return "", w.failed(err)
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(w.out, buf); err != nil {
		return "", w.failed(err)
	}

	return string(buf), nil
}

func (w *worker) failed(err error) error {
	if msg := strings.TrimSpace(w.stderr.String()); msg != "" {
		return fmt.Errorf("%w: %s", err, msg)
	}

	return err
}

func (w *worker) stop() {
	w.in.Close()
	w.cmd.Wait()
}

func TestMain(m *testing.M) {
	var started []*worker

	for _, spec := range workers {
		w, err := startWorker(spec.cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s, see README.md for setup: %v\n", spec.name, err)

			continue
		}

		started = append(started, w)
		implementations = append(implementations, implementation{spec.name, spec.wraps, w.convert})
	}

	code := m.Run()

	for _, w := range started {
		w.stop()
	}

	os.Exit(code)
}
