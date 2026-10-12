package textplain

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// dataRows returns the visible cells of a table that holds data rather than
// layout, row by row, or nil. A data table opens with a row of at least two
// header cells and has no nested tables or cells spanning rows.
func (cv *conversion) dataRows(table *html.Node) [][]*html.Node {
	switch strings.ToLower(strings.TrimSpace(getAttr(table, "role"))) {
	case "presentation", "none":
		return nil
	}

	var rows [][]*html.Node

	for _, tr := range cv.visibleRows(table) {
		var cells []*html.Node

		for c := tr.FirstChild; c != nil; c = c.NextSibling {
			if c.DataAtom != atom.Td && c.DataAtom != atom.Th || cv.isHidden(c) {
				continue
			}

			if span(c, "rowspan") > 1 || contains(c, atom.Table) {
				return nil
			}

			cells = append(cells, c)
		}

		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}

	if len(rows) == 0 || len(rows[0]) < 2 {
		return nil
	}

	for _, c := range rows[0] {
		if c.DataAtom != atom.Th {
			return nil
		}
	}

	return rows
}

func (cv *conversion) visibleRows(n *html.Node) []*html.Node {
	var rows []*html.Node

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || cv.isHidden(c) {
			continue
		}

		switch c.DataAtom {
		case atom.Tr:
			rows = append(rows, c)
		case atom.Thead, atom.Tbody, atom.Tfoot:
			rows = append(rows, cv.visibleRows(c)...)
		}
	}

	return rows
}

func span(n *html.Node, attr string) int {
	if v := strings.TrimSpace(getAttr(n, attr)); v != "" {
		if count, err := strconv.Atoi(v); err == nil {
			return count
		}
	}

	return 1
}

// dataTable writes rows as aligned columns, or as a Markdown table. It reports
// false, having written nothing, when a cell holds more than a line or the
// columns would not fit the line length.
func (cv *conversion) dataTable(o *output, table *html.Node, rows [][]*html.Node) bool {
	cells := make([][]string, len(rows))

	var caption string

	for c := table.FirstChild; c != nil; c = c.NextSibling {
		if c.DataAtom == atom.Caption && !cv.isHidden(c) {
			m := o.mark()
			cv.doConvert(o, c)
			caption += o.take(m)
		}
	}

	var cols int

	for i, row := range rows {
		for _, c := range row {
			m := o.mark()
			cv.doConvert(o, c)
			text := strings.TrimSpace(strings.Map(func(r rune) rune {
				if r == rune(lineStartMark[0]) || r == rune(hardBreak[0]) {
					return -1
				}

				return r
			}, o.take(m)))

			if strings.ContainsFunc(text, func(r rune) bool { return r <= '\x08' || r == '\n' }) {
				return false
			}

			if strings.Contains(text, "  ") || strings.ContainsAny(text, "\t\r\f") {
				text = strings.Join(strings.Fields(text), " ")
			}

			if cv.opts.markdown {
				text = strings.ReplaceAll(text, "|", `\|`)
			}

			cells[i] = append(cells[i], text)
			for range span(c, "colspan") - 1 {
				cells[i] = append(cells[i], "")
			}
		}

		cols = max(cols, len(cells[i]))
	}

	var text string
	if cv.opts.markdown {
		text = markdownTable(cells, cols)
	} else if text = alignedTable(cells, cols); cv.opts.lineLength > 0 && longestLine(text) > cv.opts.lineLength {
		return false
	}

	o.write("\n\n")
	o.write(caption)
	o.write("\n\n")
	o.write(preOpen)
	o.write(text)
	o.write(preClose)
	o.write("\n\n")

	return true
}

// alignedTable pads each column to its widest cell and rules off the header row
func alignedTable(cells [][]string, cols int) string {
	widths := make([]int, cols)

	for _, row := range cells {
		for j, cell := range row {
			widths[j] = max(widths[j], utf8.RuneCountInString(cell))
		}
	}

	rule := make([]string, cols)
	for j, w := range widths {
		rule[j] = strings.Repeat("-", w)
	}

	var out strings.Builder

	// padding is held back until a later cell needs it, so lines carry no trailing spaces
	line := func(row []string) {
		var pad int

		for j, text := range row {
			if text != "" {
				for range pad {
					out.WriteByte(' ')
				}

				out.WriteString(text)
				pad = 0
			}

			pad += widths[j] - utf8.RuneCountInString(text) + 2
		}

		out.WriteByte('\n')
	}

	for i, row := range cells {
		line(row)

		if i == 0 {
			line(rule)
		}
	}

	return out.String()
}

func markdownTable(cells [][]string, cols int) string {
	var out strings.Builder

	for i, row := range cells {
		out.WriteString("|")

		for j := range cols {
			out.WriteString(" ")

			if j < len(row) {
				out.WriteString(row[j])
			}

			out.WriteString(" |")
		}

		out.WriteString("\n")

		if i == 0 {
			out.WriteString(strings.Repeat("| --- ", cols))
			out.WriteString("|\n")
		}
	}

	return out.String()
}

func longestLine(text string) int {
	var longest int

	for line := range strings.SplitSeq(text, "\n") {
		longest = max(longest, utf8.RuneCountInString(line))
	}

	return longest
}
