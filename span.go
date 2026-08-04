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
	// dst points at the slice the current parse appends span elements to. It is
	// &out at the top level and &el.Children while recursing into an emphasis body.
	dst *[]*Element
	// stack holds the element types of the enclosing emphasis containers (the
	// current tree plus its ancestors), mirroring kramdown's @tree/@stack. It lets
	// parse_emphasis refuse to nest an :em inside an :em (or :strong inside :strong).
	stack []ElementType
	// htmlStopRE, when non-nil, is the close-tag regexp that terminates the current
	// span-HTML element body (kramdown's parse_spans stop_re). When the loop reaches a
	// "<" that begins this close tag it stops, leaving the tag for the caller to
	// consume — exactly parse_span_html's body parse.
	htmlStopRE *regexp.Regexp
	// rawMode reports that the current recursion parses a raw-content-model span-HTML
	// body (kramdown's parse_spans(..., [:span_html])): only nested span HTML is
	// recognised and every other character is literal text.
	rawMode bool
}

// parseSpans converts a block's raw text into span elements.
func (p *parser) parseSpans(raw string) []*Element {
	sp := &spanParser{p: p, src: raw}
	sp.dst = &sp.out
	sp.parseInto(nil)
	els := sp.out
	els = applyAbbreviations(els, p.abbrevs)
	return els
}

// parseInto is the main span-parsing loop: it scans for the next active span
// construct, flushing literal runs as ElText in between, and appends the results
// to *sp.dst. When stop is non-nil it mirrors kramdown's parse_spans stop_re: at
// each candidate position it checks whether the closing marker is accepted and, if
// so, flushes pending text and returns true with sp.pos left on the closer.
func (sp *spanParser) parseInto(stop *emphStop) bool {
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			sp.emitText(lit.String())
			lit.Reset()
		}
	}
	for sp.pos < len(sp.src) {
		// kramdown checks the stop regexp before the span parsers: an accepted closer
		// wins over opening a new span at the same position.
		if stop != nil && sp.src[sp.pos] == stop.delim[0] {
			if sp.acceptClose(stop, lit.Len() > 0 || len(*sp.dst) > 0) {
				flush()
				return true
			}
		}
		// A span-HTML element body stops at its close tag (kramdown's parse_spans
		// stop_re). Leave sp.pos on the "<" so the caller consumes the close tag.
		if sp.htmlStopRE != nil && sp.src[sp.pos] == '<' {
			if loc := sp.htmlStopRE.FindStringIndex(sp.src[sp.pos:]); loc != nil && loc[0] == 0 {
				flush()
				return true
			}
		}
		c := sp.src[sp.pos]
		// A raw-content-model body (kramdown's [:span_html] restriction) recognises only
		// nested span HTML; everything else — including backslashes, emphasis and code
		// markers — is literal text.
		if sp.rawMode {
			if c == '<' && sp.trySpanHTML(&lit, flush) {
				continue
			}
			lit.WriteByte(c)
			sp.pos++
			continue
		}
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
						*sp.dst = append(*sp.dst, newEl(ElBr))
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
			// tryEmphasis parses the whole construct (advancing sp.pos past the closer)
			// and returns the element, or nil plus the literal marker text to emit when
			// no emphasis can be formed.
			if el, litMarker := sp.tryEmphasis(); el != nil {
				flush()
				*sp.dst = append(*sp.dst, el)
				sp.consumeSpanIALs(el)
				continue
			} else {
				lit.WriteString(litMarker)
				sp.pos += len(litMarker)
			}
		case '[':
			if el, n := sp.tryFootnoteRef(); el != nil {
				flush()
				*sp.dst = append(*sp.dst, el)
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
				if el, n := sp.tryAutolink(); el != nil {
					flush()
					sp.push(el, n)
					continue
				}
				if sp.trySpanHTML(&lit, flush) {
					continue
				}
			}
			lit.WriteByte('<')
			sp.pos++
		case '{':
			if el, n, ok := sp.trySpanExtension(); ok {
				flush()
				if el != nil {
					*sp.dst = append(*sp.dst, el)
				}
				sp.pos += n
				continue
			}
			lit.WriteByte('{')
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
	return false
}

// reSpanIAL matches a span-level IAL ("{:...}") immediately following an inline
// element, capturing its attribute body.
var reSpanIAL = regexp.MustCompile(`^\{:([^}]*)\}`)

// reSpanExtStart matches an inline extension start tag "{::name attrs /?}".
var reSpanExtStart = regexp.MustCompile(`^\{::(\w+)(?:[ \t]((?:\\\}|[^}])*?))?(/)?\}`)

// trySpanExtension parses an inline "{::name …}…{:/…}" extension at the current
// position. It returns the element to emit (nil for none, e.g. a self-closing or
// non-html nomarkdown), the number of bytes consumed, and whether it was handled;
// a false return leaves the "{" to be emitted literally.
func (sp *spanParser) trySpanExtension() (*Element, int, bool) {
	s := sp.src[sp.pos:]
	m := reSpanExtStart.FindStringSubmatch(s)
	if m == nil {
		return nil, 0, false
	}
	name, attrRaw, selfClose := m[1], m[2], m[3] == "/"
	if name != "comment" && name != "nomarkdown" && name != "options" {
		return nil, 0, false // unknown extension: literal
	}
	if name == "options" {
		// A span-level "{::options …}" folds its recognised key="val" pairs into the
		// parser options for the remainder of the document and renders nothing.
		sp.p.applyInlineOptions(attrRaw)
	}
	if selfClose {
		return nil, len(m[0]), true // no body -> nothing rendered
	}
	rest := s[len(m[0]):]
	stopIdx, stopLen := findSpanStop(rest, name)
	if stopIdx < 0 {
		return nil, 0, false // unterminated: literal
	}
	body := rest[:stopIdx]
	consumed := len(m[0]) + stopIdx + stopLen
	switch name {
	case "options":
		// Options were already applied above; the body (up to "{:/options}") is
		// discarded, matching kramdown's span-extension 'options' handler.
		return nil, consumed, true
	case "comment":
		el := newEl(ElRawHTMLSpan)
		el.Value = "<!-- " + body + " -->"
		return el, consumed, true
	default: // nomarkdown
		if !rawForHTML(extAttrType(attrRaw)) {
			return nil, consumed, true
		}
		el := newEl(ElRawHTMLSpan)
		el.Value = body
		return el, consumed, true
	}
}

// findSpanStop returns the offset and length of the first "{:/name}" or "{:/}"
// stop tag in s, or -1 if none. A named stop tag matches only its own extension.
func findSpanStop(s, name string) (idx, length int) {
	for i := 0; i+2 < len(s); i++ {
		if s[i] != '{' || s[i+1] != ':' || s[i+2] != '/' {
			continue
		}
		j := i + 3
		for j < len(s) && s[j] != '}' {
			j++
		}
		if j < len(s) {
			if stop := s[i+3 : j]; stop == "" || stop == name {
				return i, j - i + 1
			}
		}
	}
	return -1, 0
}

// push appends a span element that consumed n source bytes, then consumes and
// applies a span IAL ("{:...}") if one immediately follows.
func (sp *spanParser) push(el *Element, n int) {
	*sp.dst = append(*sp.dst, el)
	sp.pos += n
	sp.consumeSpanIALs(el)
}

// consumeSpanIALs consumes every span IAL ("{:...}") that immediately follows the
// current position, accumulating each onto el; a "{::…}" extension or "{:/…}" stop
// tag is not a span IAL.
func (sp *spanParser) consumeSpanIALs(el *Element) {
	for {
		m := reSpanIAL.FindStringSubmatch(sp.src[sp.pos:])
		if m == nil || strings.HasPrefix(m[1], ":") || strings.HasPrefix(m[1], "/") {
			break
		}
		applyIALToElement(el, m[1], sp.p.aldDefs)
		sp.pos += len(m[0])
	}
}

// emitText pushes a literal text run, applying hard-break detection for a
// "  \n" sequence and storing the raw text for later typographic processing.
func (sp *spanParser) emitText(s string) {
	// Accumulate a single text run, breaking it only at hard line breaks. A soft
	// break keeps its newline inside the run so later passes (abbreviation
	// replacement) see the same contiguous text node kramdown produces.
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			t := newEl(ElText)
			t.Value = buf.String()
			*sp.dst = append(*sp.dst, t)
			buf.Reset()
		}
	}
	for {
		idx := strings.Index(s, "\n")
		if idx < 0 {
			break
		}
		before := s[:idx]
		trimmed := strings.TrimRight(before, " ")
		switch {
		case len(before)-len(trimmed) >= 2:
			// Two spaces before the newline are a hard break (kramdown's LINE_BREAK),
			// independent of hard_wrap: drop exactly those two spaces (keeping any
			// before them) and render <br />.
			buf.WriteString(before[:len(before)-2])
			flush()
			// The <br /> renders its own trailing newline, so don't add another.
			*sp.dst = append(*sp.dst, newEl(ElBr))
		case sp.p.opts.HardWrap:
			// hard_wrap turns every newline into a break.
			buf.WriteString(trimmed)
			flush()
			*sp.dst = append(*sp.dst, newEl(ElBr))
		default:
			// A soft break keeps the line verbatim (kramdown preserves a lone trailing
			// space) with its newline, as part of the same text run.
			buf.WriteString(before)
			buf.WriteByte('\n')
		}
		s = s[idx+1:]
	}
	buf.WriteString(s)
	flush()
}

// emitLiteral pushes a text node whose content is exempt from typographic
// substitution (used for backslash-escaped characters).
func (sp *spanParser) emitLiteral(s string) {
	t := newEl(ElText)
	t.Value = s
	t.Options["literal"] = true
	*sp.dst = append(*sp.dst, t)
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

// emphStop describes the closing marker a nested emphasis parse is looking for,
// mirroring the stop_re and its acceptance block in kramdown's parse_emphasis.
type emphStop struct {
	delim        string // the closing marker run, "*"/"_" (em) or "**"/"__" (strong)
	isEm         bool   // the enclosing element is an :em (single-char delim)
	isUnderscore bool   // the marker character is "_" (intraword-sensitive)
}

// reUnderscoreIntraword matches kramdown's guard that keeps an underscore literal
// when it directly follows a word: /[[:alpha:]]-?[[:alpha:]]*_*\z/ on the text
// preceding the marker.
var reUnderscoreIntraword = regexp.MustCompile(`[[:alpha:]]-?[[:alpha:]]*_*\z`)

// tryEmphasis is a faithful port of kramdown's parse_emphasis. It scans an
// EMPHASIS_START run ("**"/"__" greedily, else "*"/"_"), applies the opening bail
// conditions, then parses the body with a stop marker (falling back from :strong to
// :em when the strong run cannot be closed). On success it returns the built
// element with sp.pos advanced past the closer; otherwise it returns nil and the
// literal marker text (the EMPHASIS_START match) the caller should emit verbatim.
func (sp *spanParser) tryEmphasis() (*Element, string) {
	start := sp.pos
	s := sp.src[start:]
	marker := s[0]
	// EMPHASIS_START = /(?:\*\*?|__?)/ matches at most two marker characters.
	mlen := 1
	if len(s) >= 2 && s[1] == marker {
		mlen = 2
	}
	elemType := ElEm
	if mlen == 2 {
		elemType = ElStrong
	}
	isUnder := marker == '_'
	litMarker := s[:mlen]

	// Bail conditions (kramdown emits the whole EMPHASIS_START match as text):
	//  - an underscore directly following a word (intraword),
	//  - the marker being followed by whitespace (or end of input),
	//  - the same element type already being open (no :em in :em / :strong in :strong).
	if isUnder && reUnderscoreIntraword.MatchString(sp.src[:start]) {
		return nil, litMarker
	}
	if start+mlen >= len(sp.src) || isSpaceByte(sp.src[start+mlen]) {
		return nil, litMarker
	}
	if sp.stackHas(elemType) {
		return nil, litMarker
	}

	// Primary attempt with the full marker.
	el, found := sp.emphSubParse(start+mlen, litMarker, elemType, isUnder)
	// Fallback: a strong opener that cannot be closed as strong is retried as an em
	// whose opener is only the first marker character (the second becomes body text).
	if !found && elemType == ElStrong && sp.stackTop() != ElEm {
		el, found = sp.emphSubParse(start+1, s[:1], ElEm, isUnder)
	}
	if !found {
		sp.pos = start
		return nil, litMarker
	}
	return el, ""
}

// emphSubParse parses an emphasis body of type elemType starting at contentPos,
// stopping on delim. It pushes elemType onto the enclosing-type stack for the
// duration, and on success consumes the closer and leaves sp.pos past it.
func (sp *spanParser) emphSubParse(contentPos int, delim string, elemType ElementType, isUnder bool) (*Element, bool) {
	sp.pos = contentPos
	el := newEl(elemType)
	stop := &emphStop{delim: delim, isEm: elemType == ElEm, isUnderscore: isUnder}
	sp.stack = append(sp.stack, elemType)
	savedDst := sp.dst
	sp.dst = &el.Children
	// An emphasis body is a fresh parse_spans with its own stop_re: the enclosing
	// span-HTML close tag and raw mode do not apply inside it (a "</span>" here is a
	// stray close, not the container's terminator).
	savedStop, savedRaw := sp.htmlStopRE, sp.rawMode
	sp.htmlStopRE, sp.rawMode = nil, false
	found := sp.parseInto(stop)
	sp.htmlStopRE, sp.rawMode = savedStop, savedRaw
	sp.dst = savedDst
	sp.stack = sp.stack[:len(sp.stack)-1]
	if found {
		sp.pos += len(delim)
	}
	return el, found
}

// acceptClose reports whether the closer at sp.pos is a valid close for stop,
// porting the block passed to parse_spans in parse_emphasis:
//   - the preceding character is not whitespace,
//   - for an :em, the closer is not exactly a doubled marker (which belongs to a
//     strong close) unless it is tripled,
//   - for "_", the closer is not immediately followed by an alphanumeric,
//   - the element already has some content.
func (sp *spanParser) acceptClose(stop *emphStop, hasContent bool) bool {
	s := sp.src[sp.pos:]
	if !strings.HasPrefix(s, stop.delim) {
		return false
	}
	if sp.pos == 0 || isSpaceByte(sp.src[sp.pos-1]) {
		return false
	}
	if stop.isEm {
		dd := stop.delim + stop.delim
		if strings.HasPrefix(s, dd) && !strings.HasPrefix(s, dd+stop.delim) {
			return false
		}
	}
	if stop.isUnderscore {
		if after := s[len(stop.delim):]; len(after) > 0 && isAlnumByte(after[0]) {
			return false
		}
	}
	return hasContent
}

// stackHas reports whether an element of type t is currently open (the immediate
// tree or any ancestor), matching @tree.type == element || @stack.any {…}.
func (sp *spanParser) stackHas(t ElementType) bool {
	for _, e := range sp.stack {
		if e == t {
			return true
		}
	}
	return false
}

// stackTop returns the type of the immediately enclosing emphasis element, or a
// sentinel (ElRoot) when parsing at the top level.
func (sp *spanParser) stackTop() ElementType {
	if len(sp.stack) == 0 {
		return ElRoot
	}
	return sp.stack[len(sp.stack)-1]
}

// isAlnumByte reports whether b is an ASCII letter or digit ([[:alnum:]]).
func isAlnumByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// isSpaceByte reports whether b is an ASCII whitespace byte.
func isSpaceByte(b byte) bool { return b == ' ' || b == '\t' || b == '\n' }

// isWordByte reports whether b is an ASCII word character.
func isWordByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
