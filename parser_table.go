// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// reTableSep matches a kramdown table alignment/separator row, e.g. "|:-:|-:|" or
// "---: | :---:". A row qualifies if, after trimming, it consists only of pipes,
// plus signs, colons, hyphens, spaces and tabs and contains at least one hyphen.
var reTableSep = regexp.MustCompile(`^[ \t]*[|+]?[ \t]*[:-][ \t:|+-]*$`)

// isTableSepLine reports whether s is a valid table separator line (contains a
// dash and only the allowed separator characters).
func isTableSepLine(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" || !strings.Contains(t, "-") {
		return false
	}
	for _, r := range t {
		switch r {
		case '|', '+', ':', '-', ' ', '\t':
		default:
			return false
		}
	}
	return reTableSep.MatchString(s)
}

// tryTable parses a kramdown table starting at lines[start]: an optional leading
// separator, an optional header row, a mandatory separator, then body rows
// (further separators start new <tbody> sections). Returns nil if no table.
func (p *parser) tryTable(lines []string, start int) (*Element, int) {
	// A table must contain a pipe in its first row and have a separator line within
	// the first two lines.
	if !strings.Contains(lines[start], "|") {
		return nil, 0
	}
	// Scan the contiguous block of table lines (until a blank line).
	end := start
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
		end++
	}
	block := lines[start:end]
	// Find the first separator line; it must exist and be at index 0 or 1.
	sepIdx := -1
	for k := 0; k < len(block) && k < 2; k++ {
		if isTableSepLine(block[k]) {
			sepIdx = k
			break
		}
	}
	if sepIdx < 0 {
		return nil, 0
	}

	tbl := newEl(ElTable)
	var aligns []string
	i := 0
	// Optional leading separator line just sets nothing extra (kramdown ignores a
	// leading sep before any header for alignment of the header).
	leadingSep := false
	if sepIdx == 0 {
		aligns = parseAligns(block[0])
		leadingSep = true
		i = 1
	}
	// Header row + its separator.
	var head *Element
	if leadingSep {
		// A leading separator: the rows up to the next separator are the header only
		// if such a second separator exists; otherwise they are body rows (no header).
		hdrEnd := -1
		for k := i; k < len(block); k++ {
			if isTableSepLine(block[k]) {
				hdrEnd = k
				break
			}
		}
		if hdrEnd >= 0 {
			// Header is the single row immediately before the second separator (kramdown
			// treats the last pre-separator row as the header).
			head = p.tableRow(block[hdrEnd-1], true, aligns)
			aligns = parseAligns(block[hdrEnd])
			i = hdrEnd + 1
		}
		// No second separator: leave head nil and start body at i (the row after the
		// leading separator).
	} else {
		// sepIdx == 1: header is block[0], separator block[1].
		aligns = parseAligns(block[1])
		head = p.tableRow(block[0], true, aligns)
		i = 2
	}
	if head != nil {
		applyAligns(head, aligns)
		thead := newEl(ElThead)
		tr := newEl(ElTr)
		tr.Children = head.Children
		thead.addChild(tr)
		tbl.addChild(thead)
	}
	// Body rows, splitting into <tbody> sections at each further separator.
	tbody := newEl(ElTbody)
	for i < len(block) {
		line := block[i]
		if isTableSepLine(line) {
			if len(tbody.Children) > 0 {
				tbl.addChild(tbody)
				tbody = newEl(ElTbody)
			}
			i++
			continue
		}
		row := p.tableRow(line, false, aligns)
		applyAligns(row, aligns)
		tr := newEl(ElTr)
		tr.Children = row.Children
		tbody.addChild(tr)
		i++
	}
	if len(tbody.Children) > 0 {
		tbl.addChild(tbody)
	}
	return tbl, end - start
}

// parseAligns reads cell alignments from a separator line into "left"/"right"/
// "center"/"" per column.
func parseAligns(sep string) []string {
	sep = strings.TrimSpace(sep)
	sep = strings.Trim(sep, "|+")
	parts := splitTableCells(sep)
	aligns := make([]string, 0, len(parts))
	for _, c := range parts {
		c = strings.TrimSpace(c)
		left := strings.HasPrefix(c, ":")
		right := strings.HasSuffix(c, ":")
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

// tableRow splits a "| a | b |" line into cells, building ElTd children (a header
// row marks them for <th> rendering via Options["header"]).
func (p *parser) tableRow(line string, header bool, _ []string) *Element {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	cells := splitTableCells(line)
	row := newEl(ElTr)
	for _, c := range cells {
		td := newEl(ElTd)
		td.Options["raw"] = strings.TrimSpace(c)
		if header {
			td.Options["header"] = true
		}
		row.addChild(td)
	}
	return row
}

// splitTableCells splits on unescaped pipes ("\|" is a literal pipe in a cell).
func splitTableCells(s string) []string {
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '|' {
			cur.WriteString("\\|")
			i++
			continue
		}
		if s[i] == '|' {
			cells = append(cells, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	cells = append(cells, cur.String())
	return cells
}

// applyAligns records each cell's alignment style (if any) for the converter.
func applyAligns(row *Element, aligns []string) {
	for i, td := range row.Children {
		if i < len(aligns) && aligns[i] != "" {
			td.Options["align"] = aligns[i]
		}
	}
}
