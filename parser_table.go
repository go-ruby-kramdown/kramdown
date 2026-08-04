// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// This file ports kramdown 2.5.2's parser/kramdown/table.rb: a block that begins
// (after up to three spaces, at a non-space) with a line carrying a pipe is a
// candidate table. Rows are split into cells on unescaped pipes that lie outside
// code spans; a "-"-only line is a separator (the row above it becomes the header
// and sets column alignment), a "="-only line opens the footer, and a strict
// second pass rejects the candidate unless every line carries an unescaped pipe
// outside a code span. A table with no body is discarded.

var (
	// reCodeHTML matches an inline <code>…</code> HTML element (single line): the
	// pipes it contains never split a cell. It is deliberately non-greedy.
	reCodeHTML = regexp.MustCompile(`(?s)<code.*?>.*?</code>`)
	// reHSepAlign scans a separator line for per-column alignment markers.
	reHSepAlign = regexp.MustCompile(`[ \t]?(:?)-+(:?)[ \t]?`)
	// reLeadingPipe reports a leading (optionally space-indented) pipe.
	reLeadingPipe = regexp.MustCompile(`^\s*\|`)
)

// codeToken is a slice of a row/line flagged as lying inside a code span/element
// (its pipes are protected) or ordinary, splittable text.
type codeToken struct {
	s    string
	code bool
}

// splitCodeHTML splits s into alternating ordinary-text and <code>…</code>-HTML
// tokens (the latter protected from cell splitting).
func splitCodeHTML(s string) []codeToken {
	locs := reCodeHTML.FindAllStringIndex(s, -1)
	if locs == nil {
		return []codeToken{{s, false}}
	}
	var toks []codeToken
	prev := 0
	for _, l := range locs {
		if l[0] > prev {
			toks = append(toks, codeToken{s[prev:l[0]], false})
		}
		toks = append(toks, codeToken{s[l[0]:l[1]], true})
		prev = l[1]
	}
	if prev < len(s) {
		toks = append(toks, codeToken{s[prev:], false})
	}
	return toks
}

// splitCodespans splits s into alternating text and backtick code-span tokens,
// matching kramdown's code-span rule: a maximal run of N backticks opens a span
// that closes at the next run of exactly N backticks; an unterminated run is
// literal text.
func splitCodespans(s string) []codeToken {
	var toks []codeToken
	prev, i := 0, 0
	for i < len(s) {
		if s[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == '`' {
			j++
		}
		n := j - i
		if end := findBacktickClose(s, j, n); end >= 0 {
			if i > prev {
				toks = append(toks, codeToken{s[prev:i], false})
			}
			toks = append(toks, codeToken{s[i:end], true})
			prev, i = end, end
			continue
		}
		i = j // unterminated run: literal text
	}
	if prev < len(s) {
		toks = append(toks, codeToken{s[prev:], false})
	}
	return toks
}

// findBacktickClose returns the index just past a closing run of exactly n
// backticks at or after from, or -1 if none.
func findBacktickClose(s string, from, n int) int {
	for k := from; k < len(s); {
		if s[k] != '`' {
			k++
			continue
		}
		m := k
		for m < len(s) && s[m] == '`' {
			m++
		}
		if m-k == n {
			return m
		}
		k = m
	}
	return -1
}

// tryTable parses a kramdown table starting at lines[start]. It returns nil when
// the block is not a table (the caller then treats it as a paragraph).
func (p *parser) tryTable(lines []string, start int) (*Element, int) {
	if !strings.Contains(lines[start], "|") {
		return nil, 0
	}
	// The table block runs to the next blank line or standalone block IAL. A
	// trailing "{:…}" line is NOT part of the table; kramdown ends the table there
	// and lets the block IAL attach to the table as a separate element (so
	// "| … |\n{:.cls}" yields <table class="cls">).
	end := start
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
		if end > start {
			if _, ok := matchBlockIAL(lines[end]); ok {
				break
			}
		}
		end++
	}
	block := lines[start:end]
	leadingPipe := reLeadingPipe.MatchString(block[0])

	tbl := newEl(ElTable)
	var sections []*Element
	var rows []*Element
	var aligns []string
	hasFooter := false
	columns := 0

	// addContainer flushes the accumulated rows into a new section of the given
	// type. Once a footer has begun, a plain tbody flush is suppressed unless forced
	// (so footer-region separators merge their rows into the single tfoot).
	addContainer := func(t ElementType, force bool) {
		if hasFooter && t == ElTbody && !force {
			return
		}
		sec := newEl(t)
		sec.Children = rows
		rows = nil
		sections = append(sections, sec)
	}

	i := 0
	// A leading separator line is consumed and discarded (it only precedes the
	// header; it never sets alignment).
	if len(block) > 0 && isTableSepLine(block[0]) {
		i = 1
	}
	reverted := false
	for ; i < len(block); i++ {
		line := block[i]
		switch {
		case isTableSepLine(line):
			switch {
			case len(rows) == 0:
				// consecutive/leading separators: ignore
			case len(aligns) == 0 && !hasFooter:
				addContainer(ElThead, false)
				aligns = tableAligns(line)
			default:
				addContainer(ElTbody, false)
			}
		case isTableFsepLine(line):
			if len(rows) > 0 {
				addContainer(ElTbody, true)
			}
			hasFooter = true
		case strings.Contains(line, "|"):
			row := p.tableRow(line, leadingPipe)
			if n := len(row.Children); n > columns {
				columns = n
			}
			rows = append(rows, row)
		default:
			// A non-blank line without a pipe: the candidate runs into a paragraph.
			reverted = true
		}
		if reverted {
			break
		}
	}
	if reverted {
		return nil, 0
	}
	// Strict pass: reject unless every line carries an unescaped pipe outside a code
	// span (kramdown's pipe_on_line check over the whole block).
	if !tableBlockHasPipe(block) {
		return nil, 0
	}
	if len(rows) > 0 {
		if hasFooter {
			addContainer(ElTfoot, false)
		} else {
			addContainer(ElTbody, false)
		}
	}
	// A table must have a body.
	if !hasSection(sections, ElTbody) {
		return nil, 0
	}
	// Pad every row to the column count and normalise the alignment length.
	for _, sec := range sections {
		for _, row := range sec.Children {
			for len(row.Children) < columns {
				row.addChild(newEl(ElTd))
			}
		}
	}
	if len(aligns) > columns {
		aligns = aligns[:columns]
	}
	for len(aligns) < columns {
		aligns = append(aligns, "")
	}
	for _, sec := range sections {
		for _, row := range sec.Children {
			applyAligns(row, aligns)
		}
	}
	tbl.Children = sections
	return tbl, end - start
}

// hasSection reports whether sections contains one of the given type.
func hasSection(sections []*Element, t ElementType) bool {
	for _, s := range sections {
		if s.Type == t {
			return true
		}
	}
	return false
}

// isTableSepLine reports whether s is an alignment/separator line: only
// "+|: \t-" characters (trailing whitespace ignored) and at least one dash.
func isTableSepLine(s string) bool {
	return isSepClass(s, '-', "+|: \t-")
}

// isTableFsepLine reports whether s is a footer separator: only "+|: \t=" and at
// least one "=".
func isTableFsepLine(s string) bool {
	return isSepClass(s, '=', "+|: \t=")
}

// isSepClass reports whether s (minus trailing whitespace) is non-empty, contains
// the required rune and consists only of the allowed characters.
func isSepClass(s string, required byte, allowed string) bool {
	t := strings.TrimRight(s, " \t")
	if t == "" || !strings.ContainsRune(t, rune(required)) {
		return false
	}
	for i := 0; i < len(t); i++ {
		if strings.IndexByte(allowed, t[i]) < 0 {
			return false
		}
	}
	return true
}

// tableAligns reads per-column alignments from a separator line into
// "left"/"right"/"center"/"" (default).
func tableAligns(sep string) []string {
	sep = strings.TrimRight(sep, " \t")
	ms := reHSepAlign.FindAllStringSubmatch(sep, -1)
	aligns := make([]string, 0, len(ms))
	for _, m := range ms {
		left, right := m[1] == ":", m[2] == ":"
		switch {
		case left && right:
			aligns = append(aligns, "center")
		case right:
			aligns = append(aligns, "right")
		case left:
			aligns = append(aligns, "left")
		default:
			aligns = append(aligns, "")
		}
	}
	return aligns
}

// tableRow splits one row line into cells (respecting code spans and escaped
// pipes) and builds a <tr> of <td> elements holding each cell's raw text.
func (p *parser) tableRow(line string, leadingPipe bool) *Element {
	cells := splitTableCells(line, leadingPipe)
	row := newEl(ElTr)
	for _, c := range cells {
		td := newEl(ElTd)
		td.Options["raw"] = strings.TrimSpace(c)
		row.addChild(td)
	}
	return row
}

// splitTableCells splits a row line into cell texts. Pipes inside a <code> HTML
// element or a backtick code span do not split; "\|" is a literal pipe; a leading
// empty cell is dropped when the row began with a pipe and a trailing empty cell
// is always dropped.
func splitTableCells(line string, leadingPipe bool) []string {
	line = strings.TrimRight(line, " \t")
	cells := []string{""}
	appendLast := func(s string) { cells[len(cells)-1] += s }
	for _, seg := range splitCodeHTML(line) {
		if seg.code {
			appendLast(seg.s)
			continue
		}
		for _, ct := range splitCodespans(seg.s) {
			if ct.code {
				appendLast(ct.s)
				continue
			}
			for k := 0; k < len(ct.s); k++ {
				if ct.s[k] == '\\' && k+1 < len(ct.s) && ct.s[k+1] == '|' {
					appendLast("|")
					k++
					continue
				}
				if ct.s[k] == '|' {
					cells = append(cells, "")
					continue
				}
				// Append the raw byte: string(ct.s[k]) would reinterpret a single UTF-8
				// byte as a code point and double-encode multibyte characters (ö -> Ã¶).
				appendLast(ct.s[k : k+1])
			}
		}
	}
	if leadingPipe && len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// tableBlockHasPipe is kramdown's post-parse gate: parsing the whole block for
// code spans (which may span row boundaries), every line must carry an unescaped
// pipe outside a code span. It returns whether the candidate is a real table.
func tableBlockHasPipe(block []string) bool {
	whole := strings.Join(block, "\n") + "\n"
	var toks []codeToken
	for _, seg := range splitCodeHTML(whole) {
		if seg.code {
			continue // <code> HTML carries no value in kramdown's check: skipped
		}
		toks = append(toks, splitCodespans(seg.s)...)
	}
	pipeOnLine := false
	for _, t := range toks {
		lines := splitStripTrailingEmpty(t.s)
		if len(lines) == 0 {
			continue
		}
		if t.code {
			if len(lines) > 2 || (len(lines) == 2 && !pipeOnLine) {
				break
			}
			if len(lines) == 2 && pipeOnLine {
				pipeOnLine = false
			}
			continue
		}
		if len(lines) > 1 && !pipeOnLine && !lineHasTablePipe(lines[0]) {
			break
		}
		base := pipeOnLine
		if len(lines) > 1 {
			base = false
		}
		pipeOnLine = base || lineHasTablePipe(lines[len(lines)-1])
	}
	return pipeOnLine
}

// splitStripTrailingEmpty splits s on newlines and drops trailing empty fields
// (Ruby's String#split default).
func splitStripTrailingEmpty(s string) []string {
	parts := strings.Split(s, "\n")
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// lineHasTablePipe reports whether s starts with a pipe or contains an unescaped
// pipe (kramdown's ^TABLE_PIPE_CHECK).
func lineHasTablePipe(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '|' && (i == 0 || s[i-1] != '\\') {
			return true
		}
	}
	return false
}

// applyAligns records each cell's alignment style (if any) for the converter.
func applyAligns(row *Element, aligns []string) {
	for i, td := range row.Children {
		if i < len(aligns) && aligns[i] != "" {
			td.Options["align"] = aligns[i]
		}
	}
}
