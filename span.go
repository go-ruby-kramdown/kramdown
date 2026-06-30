// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// spanParser parses inline (span-level) markup of one block's raw text into a
// slice of span Elements, resolving links/abbreviations/footnotes against the
// document parser's definition tables.
type spanParser struct {
	p   *parser
	src string
	pos int
	out []*Element
}

// parseSpans converts a block's raw text into span elements.
func (p *parser) parseSpans(raw string) []*Element {
	sp := &spanParser{p: p, src: raw}
	sp.run()
	els := sp.out
	els = applyAbbreviations(els, p.abbrevs)
	return els
}

// run is the main span-parsing loop: it scans for the next active span construct,
// flushing literal runs as ElText in between.
func (sp *spanParser) run() {
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			sp.emitText(lit.String())
			lit.Reset()
		}
	}
	for sp.pos < len(sp.src) {
		c := sp.src[sp.pos]
		switch c {
		case '\\':
			if sp.pos+1 < len(sp.src) && isEscapable(sp.src[sp.pos+1]) {
				next := sp.src[sp.pos+1]
				// "\\" followed by newline/end is a hard line break; otherwise it is
				// an escaped (literal) backslash.
				if next == '\\' {
					rest := sp.src[sp.pos+2:]
					// A "\\" immediately before a newline (that has content after it) is a
					// hard line break; a "\\" followed by spaces is a literal backslash
					// (the trailing spaces, if any, form their own break in emitText).
					if strings.HasPrefix(rest, "\n") && strings.TrimSpace(rest) != "" {
						flush()
						sp.out = append(sp.out, newEl(ElBr))
						// Skip past the "\\" and the newline it consumes; the ElBr renders
						// its own trailing newline so the source "\n" must not be re-emitted.
						sp.pos += 3
						continue
					}
				}
				// Emit the escaped character as a literal text node exempt from later
				// typographic substitution.
				flush()
				sp.emitLiteral(string(next))
				sp.pos += 2
				continue
			}
			lit.WriteByte('\\')
			sp.pos++
		case '`':
			if el, n := sp.tryCodespan(); el != nil {
				flush()
				sp.push(el, n)
				continue
			}
			lit.WriteByte('`')
			sp.pos++
		case '*', '_':
			if el, n := sp.tryEmphasis(); el != nil {
				flush()
				sp.push(el, n)
				continue
			}
			lit.WriteByte(c)
			sp.pos++
		case '[':
			if el, n := sp.tryFootnoteRef(); el != nil {
				flush()
				sp.out = append(sp.out, el)
				sp.pos += n
				continue
			}
			if el, n := sp.tryLink(false); el != nil {
				flush()
				sp.push(el, n)
				continue
			}
			lit.WriteByte('[')
			sp.pos++
		case '!':
			if sp.pos+1 < len(sp.src) && sp.src[sp.pos+1] == '[' {
				if el, n := sp.tryLink(true); el != nil {
					flush()
					sp.push(el, n)
					continue
				}
			}
			lit.WriteByte('!')
			sp.pos++
		case '<':
			// A "<<" guillemet opener is not an HTML tag: skip autolink/HTML when this
			// "<" is the second of a pair or is immediately followed by another "<".
			doubled := (sp.pos > 0 && sp.src[sp.pos-1] == '<') ||
				(sp.pos+1 < len(sp.src) && sp.src[sp.pos+1] == '<')
			if !doubled {
				if el, n := sp.tryAutolinkOrHTML(); el != nil {
					flush()
					sp.push(el, n)
					continue
				}
			}
			lit.WriteByte('<')
			sp.pos++
		case '\n':
			// Soft line break, possibly hard if preceded by two spaces (handled at
			// block level via trailing markers); here keep newline literal.
			lit.WriteByte('\n')
			sp.pos++
		default:
			lit.WriteByte(c)
			sp.pos++
		}
	}
	flush()
}

// reSpanIAL matches a span-level IAL ("{:...}") immediately following an inline
// element, capturing its attribute body.
var reSpanIAL = regexp.MustCompile(`^\{:([^}]*)\}`)

// push appends a span element that consumed n source bytes, then consumes and
// applies a span IAL ("{:...}") if one immediately follows.
func (sp *spanParser) push(el *Element, n int) {
	sp.out = append(sp.out, el)
	sp.pos += n
	if m := reSpanIAL.FindStringSubmatch(sp.src[sp.pos:]); m != nil {
		applyIALToElement(el, m[1], sp.p.aldDefs)
		sp.pos += len(m[0])
	}
}

// emitText pushes a literal text run, applying hard-break detection for a
// "  \n" sequence and storing the raw text for later typographic processing.
func (sp *spanParser) emitText(s string) {
	// Handle hard line breaks: two-or-more trailing spaces before a newline.
	for {
		idx := strings.Index(s, "\n")
		if idx < 0 {
			break
		}
		before := s[:idx]
		trimmed := strings.TrimRight(before, " ")
		hard := sp.p.opts.HardWrap && len(before)-len(trimmed) >= 2
		t := newEl(ElText)
		// In both the hard- and soft-break cases the trailing spaces before the
		// newline are dropped (kramdown collapses end-of-line whitespace).
		t.Value = trimmed
		sp.out = append(sp.out, t)
		if hard {
			// The <br /> renders its own trailing newline, so don't add another.
			sp.out = append(sp.out, newEl(ElBr))
		} else {
			nl := newEl(ElText)
			nl.Value = "\n"
			sp.out = append(sp.out, nl)
		}
		s = s[idx+1:]
	}
	if s != "" {
		t := newEl(ElText)
		t.Value = s
		sp.out = append(sp.out, t)
	}
}

// emitLiteral pushes a text node whose content is exempt from typographic
// substitution (used for backslash-escaped characters).
func (sp *spanParser) emitLiteral(s string) {
	t := newEl(ElText)
	t.Value = s
	t.Options["literal"] = true
	sp.out = append(sp.out, t)
}

// isEscapable reports whether c may follow a backslash as a kramdown escape.
func isEscapable(c byte) bool {
	switch c {
	case '\\', '.', '*', '_', '+', '-', '`', '(', ')', '[', ']', '{', '}',
		'#', '!', '<', '>', ':', '|', '"', '\'', '=', '~', '^', '&':
		return true
	}
	return false
}

// tryCodespan parses a `...` / “...“ code span, returning the element and the
// number of source bytes consumed, or nil if the run is not a valid code span.
func (sp *spanParser) tryCodespan() (*Element, int) {
	s := sp.src[sp.pos:]
	n := 0
	for n < len(s) && s[n] == '`' {
		n++
	}
	open := s[:n]
	rest := s[n:]
	simple := n == 1
	// A single backtick that is preceded by whitespace (or at the start) and
	// followed by whitespace is literal, not a code-span opener (kramdown's
	// `pre_match =~ /\s\Z|\A\Z/ && match?(/\s/)` guard).
	if simple {
		precededBySpace := sp.pos == 0 || isSpaceByte(sp.src[sp.pos-1])
		followedBySpace := len(rest) > 0 && isSpaceByte(rest[0])
		if precededBySpace && followedBySpace {
			return nil, 0
		}
	}
	idx := findCodeClose(rest, open)
	if idx < 0 {
		return nil, 0
	}
	content := rest[:idx]
	// For non-simple spans kramdown strips a single leading and trailing space.
	trimmed := content
	if !simple {
		if strings.HasPrefix(trimmed, " ") {
			trimmed = trimmed[1:]
		}
		if strings.HasSuffix(trimmed, " ") {
			trimmed = trimmed[:len(trimmed)-1]
		}
	}
	el := newEl(ElCodespan)
	el.Value = trimmed
	return el, n + idx + len(open)
}

// findCodeClose finds the closing backtick run equal to open in s. Backslashes
// inside a code span are literal (kramdown does not process escapes within code
// spans); an escaped backtick that would have opened a span is handled earlier by
// the span loop, so it never reaches here as an opener.
func findCodeClose(s, open string) int {
	i := 0
	for i < len(s) {
		if s[i] == '`' {
			j := i
			for j < len(s) && s[j] == '`' {
				j++
			}
			if j-i == len(open) {
				return i
			}
			i = j
			continue
		}
		i++
	}
	return -1
}

// tryEmphasis parses *em*, **strong**, _em_, __strong__ and the ***both***
// nesting, applying kramdown's intraword-underscore and flanking rules for the
// common cases. Returns the element and consumed length, or nil to fall through.
func (sp *spanParser) tryEmphasis() (*Element, int) {
	s := sp.src[sp.pos:]
	marker := s[0]
	// Count run length (1 => em, 2 => strong, 3 => both).
	run := 0
	for run < len(s) && s[run] == marker {
		run++
	}
	if run > 3 {
		run = 1 // overly long runs: treat opener as single
	}
	// Opening flank: the char after the marker run must not be whitespace.
	if run >= len(s) || s[run] == ' ' || s[run] == '\t' || s[run] == '\n' {
		return nil, 0
	}
	// Underscore intraword: a "_" with a word char immediately before is literal.
	if marker == '_' && sp.pos > 0 {
		prev := sp.src[sp.pos-1]
		if isWordByte(prev) {
			return nil, 0
		}
	}
	// Find the matching closing run of the same marker.
	closeIdx, closeLen := findEmphClose(s, marker, run)
	if closeIdx < 0 {
		return nil, 0
	}
	inner := s[run:closeIdx]
	if strings.TrimSpace(inner) == "" {
		return nil, 0
	}
	// Recurse into inner content.
	innerEls := sp.p.parseSpans(inner)
	consumed := closeIdx + closeLen
	switch closeLen {
	case 3:
		strong := newEl(ElStrong)
		em := newEl(ElEm)
		em.Children = innerEls
		strong.addChild(em)
		return strong, consumed
	case 2:
		el := newEl(ElStrong)
		el.Children = innerEls
		return el, consumed
	default:
		el := newEl(ElEm)
		el.Children = innerEls
		return el, consumed
	}
}

// findEmphClose locates the closing marker run for an emphasis opener of the given
// length, returning the byte offset of the closer and its length (1/2/3). The
// closer must be non-space-flanked on its left and (for "_") not intraword.
func findEmphClose(s string, marker byte, openRun int) (int, int) {
	i := openRun
	for i < len(s) {
		if s[i] != marker {
			i++
			continue
		}
		// count run
		j := i
		for j < len(s) && s[j] == marker {
			j++
		}
		runLen := j - i
		// left flank: preceding char must not be whitespace
		if i > 0 && (s[i-1] == ' ' || s[i-1] == '\t' || s[i-1] == '\n') {
			i = j
			continue
		}
		// underscore intraword on the right
		if marker == '_' && j < len(s) && isWordByte(s[j]) {
			i = j
			continue
		}
		// Match the requested closing length.
		want := openRun
		if runLen >= want {
			return i, want
		}
		i = j
	}
	return -1, 0
}

// isSpaceByte reports whether b is an ASCII whitespace byte.
func isSpaceByte(b byte) bool { return b == ' ' || b == '\t' || b == '\n' }

// isWordByte reports whether b is an ASCII word character.
func isWordByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
