// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// symChar maps a typographic-symbol name to the literal UTF-8 character(s)
// kramdown's HTML converter emits for it. The guillemet "_space" variants carry a
// non-breaking space adjacent to the bracket, mirroring kramdown's output.
func symChar(name string) string {
	switch name {
	case "lsquo":
		return "‘"
	case "rsquo":
		return "’"
	case "ldquo":
		return "“"
	case "rdquo":
		return "”"
	case "ndash":
		return "–"
	case "mdash":
		return "—"
	case "hellip":
		return "…"
	case "laquo":
		return "«"
	case "raquo":
		return "»"
	case "laquo_space":
		return "« "
	case "raquo_space":
		return " »"
	}
	return ""
}

// applyTypography walks the span tree and replaces straight quotes, dashes and
// ellipses in text nodes with the corresponding typographic symbols, when
// smart-quote / typographic processing is enabled. It mirrors kramdown's
// SmartQuotes/Typographic substitutions for the common cases.
func (c *htmlConverter) applyTypography(els []*Element) []*Element {
	if !c.doc.Opts.SmartQuotes && !c.doc.Opts.Typographic {
		return els
	}
	return c.typoWalk(els, "")
}

// typoWalk recursively substitutes typography in text nodes. prevChar carries the
// last non-markup character seen so quote direction can be decided across element
// boundaries.
func (c *htmlConverter) typoWalk(els []*Element, prevChar string) []*Element {
	var out []*Element
	for _, e := range els {
		switch e.Type {
		case ElText:
			if lit, _ := e.Options["literal"].(bool); lit {
				out = append(out, e)
				if lr := lastRune(e.Value); lr != "" {
					prevChar = lr
				}
				continue
			}
			subs, last := c.substituteText(e.Value, prevChar)
			out = append(out, subs...)
			if last != "" {
				prevChar = last
			}
		case ElCodespan, ElRawHTMLSpan:
			out = append(out, e)
			prevChar = "x" // a code span counts as a word char for following quotes
		default:
			e.Children = c.typoWalk(e.Children, prevChar)
			out = append(out, e)
			prevChar = "x"
		}
	}
	return out
}

var (
	reDashes   = regexp.MustCompile(`---|--`)
	reEllipsis = regexp.MustCompile(`\.\.\.`)
	reGuillem  = regexp.MustCompile(`<<( ?)|( ?)>>`)
)

// substituteText runs the typographic substitutions over one text run and returns
// the resulting element slice plus the final plain character (for cross-node quote
// direction).
func (c *htmlConverter) substituteText(s, prevChar string) ([]*Element, string) {
	if s == "" {
		return nil, ""
	}
	// First, dashes / ellipses / guillemets (Typographic).
	parts := []typoPart{{text: s}}
	if c.doc.Opts.Typographic {
		parts = splitSub(parts, reEllipsis, "hellip")
		parts = splitDashes(parts)
		parts = splitGuillemets(parts)
	}
	// Then smart quotes over the remaining text segments.
	var out []*Element
	last := lastRune(s)
	prev := prevChar
	for _, p := range parts {
		if p.sym != "" {
			out = append(out, typoSym(p.sym))
			// A dash/ellipsis/guillemet acts as a word boundary for the next quote, so
			// a quote following one opens rather than closes (matching kramdown).
			prev = " "
			continue
		}
		if c.doc.Opts.SmartQuotes {
			segEls, lp := smartQuotes(p.text, prev)
			out = append(out, segEls...)
			if lp != "" {
				prev = lp
			}
		} else {
			t := newEl(ElText)
			t.Value = p.text
			out = append(out, t)
			if lr := lastRune(p.text); lr != "" {
				prev = lr
			}
		}
	}
	return out, last
}

// typoPart is a fragment of a text run: either literal text or a resolved symbol.
type typoPart struct {
	text string
	sym  string
}

// typoSym builds an ElTypographicSym element for the named entity.
func typoSym(name string) *Element {
	e := newEl(ElTypographicSym)
	e.Value = name
	return e
}

// splitSub splits each literal part on a simple regexp, inserting the given symbol
// at each match.
func splitSub(parts []typoPart, re *regexp.Regexp, sym string) []typoPart {
	var out []typoPart
	for _, p := range parts {
		if p.sym != "" {
			out = append(out, p)
			continue
		}
		s := p.text
		for {
			loc := re.FindStringIndex(s)
			if loc == nil {
				if s != "" {
					out = append(out, typoPart{text: s})
				}
				break
			}
			if loc[0] > 0 {
				out = append(out, typoPart{text: s[:loc[0]]})
			}
			out = append(out, typoPart{sym: sym})
			s = s[loc[1]:]
		}
	}
	return out
}

// splitDashes resolves --- to mdash and -- to ndash.
func splitDashes(parts []typoPart) []typoPart {
	var out []typoPart
	for _, p := range parts {
		if p.sym != "" {
			out = append(out, p)
			continue
		}
		s := p.text
		for {
			loc := reDashes.FindStringIndex(s)
			if loc == nil {
				if s != "" {
					out = append(out, typoPart{text: s})
				}
				break
			}
			if loc[0] > 0 {
				out = append(out, typoPart{text: s[:loc[0]]})
			}
			if s[loc[0]:loc[1]] == "---" {
				out = append(out, typoPart{sym: "mdash"})
			} else {
				out = append(out, typoPart{sym: "ndash"})
			}
			s = s[loc[1]:]
		}
	}
	return out
}

// splitGuillemets resolves << and >> (with optional adjacent space) to the
// guillemet symbols, matching kramdown's laquo/raquo handling including the
// non-breaking space variant.
func splitGuillemets(parts []typoPart) []typoPart {
	var out []typoPart
	for _, p := range parts {
		if p.sym != "" {
			out = append(out, p)
			continue
		}
		s := p.text
		for {
			oi := strings.Index(s, "<<")
			ci := strings.Index(s, ">>")
			idx := -1
			open := false
			switch {
			case oi >= 0 && (ci < 0 || oi < ci):
				idx, open = oi, true
			case ci >= 0:
				idx, open = ci, false
			}
			if idx < 0 {
				if s != "" {
					out = append(out, typoPart{text: s})
				}
				break
			}
			if open {
				if idx > 0 {
					out = append(out, typoPart{text: s[:idx]})
				}
				rest := s[idx+2:]
				if strings.HasPrefix(rest, " ") {
					out = append(out, typoPart{sym: "laquo_space"})
					s = rest[1:]
				} else {
					out = append(out, typoPart{sym: "laquo"})
					s = rest
				}
			} else {
				before := s[:idx]
				if strings.HasSuffix(before, " ") {
					out = append(out, typoPart{text: before[:len(before)-1]})
					out = append(out, typoPart{sym: "raquo_space"})
				} else {
					out = append(out, typoPart{text: before})
					out = append(out, typoPart{sym: "raquo"})
				}
				s = s[idx+2:]
			}
		}
	}
	return out
}

// lastRune returns the last UTF-8 rune of s as a string ("" if empty).
func lastRune(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	return string(r[len(r)-1])
}

// smartQuotes converts straight ' and " to curly quotes, choosing opening vs
// closing based on the preceding character, and handling the apostrophe / decade
// (”80s”) cases kramdown special-cases.
func smartQuotes(s, prevChar string) ([]*Element, string) {
	var out []*Element
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			t := newEl(ElText)
			t.Value = buf.String()
			out = append(out, t)
			buf.Reset()
		}
	}
	runes := []rune(s)
	prevOf := func(i int) string {
		if i > 0 {
			return string(runes[i-1])
		}
		// At i==0 nothing has been buffered yet, so the previous character is
		// whatever the caller carried in from the preceding text node.
		return prevChar
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '\'':
			prev := prevOf(i)
			next := ""
			if i+1 < len(runes) {
				next = string(runes[i+1])
			}
			flush()
			if isCloseQuoteContext(prev, next) {
				out = append(out, typoSym("rsquo"))
			} else {
				out = append(out, typoSym("lsquo"))
			}
		case '"':
			prev := prevOf(i)
			next := ""
			if i+1 < len(runes) {
				next = string(runes[i+1])
			}
			flush()
			if isCloseQuoteContext(prev, next) {
				out = append(out, typoSym("rdquo"))
			} else {
				out = append(out, typoSym("ldquo"))
			}
		default:
			buf.WriteRune(r)
		}
	}
	flush()
	return out, lastRune(s)
}

// isCloseQuoteContext decides whether a quote at this position is a closing quote:
// a closing quote follows a word/punctuation character (or begins a decade like
// '80s when preceded by whitespace and followed by a digit).
func isCloseQuoteContext(prev, next string) bool {
	if prev == "" {
		return false
	}
	// Apostrophe-as-closing before a digit (decade): 'the '80s'.
	if next != "" && isDigitStr(next) && (prev == "" || isSpaceStr(prev)) {
		return true
	}
	if isSpaceStr(prev) {
		return false
	}
	// Following a word char or closing punctuation -> closing quote.
	return true
}

// isSpaceStr reports whether s is a single whitespace rune.
func isSpaceStr(s string) bool {
	return s == " " || s == "\t" || s == "\n"
}

// isDigitStr reports whether s starts with an ASCII digit.
func isDigitStr(s string) bool {
	return len(s) > 0 && s[0] >= '0' && s[0] <= '9'
}
