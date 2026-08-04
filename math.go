// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// reBlockMathOpen matches the "$$" opener of a display-math block: up to three
// leading spaces, an optional backslash (which escapes the block, disabling it),
// then "$$".
var reBlockMathOpen = regexp.MustCompile(`^( {0,3})(\\?)\$\$`)

// parseBlockMath recognises a standalone display-math block ("$$ … $$") that sits
// at a block boundary and whose closing "$$" is the last non-space content before
// the next block boundary (kramdown's parse_block_math with after_block_boundary?
// and before_block_boundary?). It returns the number of lines consumed and true, or
// 0/false when the current position is not such a block (leaving it to the
// paragraph/span parser, where an inline "$$…$$" becomes span math instead). The
// caller only invokes it at a block boundary.
func (p *parser) parseBlockMath(lines []string, i int, parent *Element) (int, bool) {
	m := reBlockMathOpen.FindStringSubmatch(lines[i])
	if m == nil || m[2] == `\` {
		return 0, false // no "$$" opener, or an escaped "\$$"
	}
	open := len(m[0]) // index just past the opening "$$" on the first line
	var content strings.Builder
	rest := lines[i][open:]
	end := i
	closeCol := -1
	for {
		if idx := strings.Index(rest, "$$"); idx >= 0 {
			content.WriteString(rest[:idx])
			closeCol = idx
			break
		}
		content.WriteString(rest)
		content.WriteByte('\n')
		end++
		if end >= len(lines) {
			return 0, false // no closing "$$"
		}
		rest = lines[end]
	}
	// Anything but whitespace after the closing "$$" on its line means the math is
	// inline within a paragraph, not a standalone block.
	if strings.TrimSpace(rest[closeCol+2:]) != "" {
		return 0, false
	}
	// The math must be followed by a block boundary: EOF, a blank line, or an
	// end-of-block "^" marker.
	if end+1 < len(lines) {
		next := strings.TrimSpace(lines[end+1])
		if next != "" && strings.TrimRight(lines[end+1], " \t") != "^" {
			return 0, false
		}
	}
	el := newEl(ElMath)
	el.Value = strings.TrimSpace(content.String())
	parent.addChild(el)
	return end - i + 1, true
}

// escapeMathValue escapes a math element's LaTeX source the way kramdown's
// escape_html(:all) does before wrapping it in the engine delimiters: every "&",
// "<", ">" and '"' is replaced, with no entity passthrough.
func escapeMathValue(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
