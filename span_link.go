// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// tryLink parses an inline or reference link/image starting at "[" (or "![" when
// image is true), returning the element and consumed length, or nil to fall back
// to literal text. It is a faithful port of kramdown's parse_link: a failed parse
// consumes nothing so the span loop emits the lone "[" (or "![") literally and
// continues from the following character, exactly as the gem reverts to saved_pos.
func (sp *spanParser) tryLink(image bool) (*Element, int) {
	start := sp.pos
	open := start
	if image {
		open = start + 1 // skip '!'
	}
	// No nested <a> inside a link (images are still allowed), matching the gem's
	// @tree.type == :a guard.
	if !image && sp.p.noLinks {
		return nil, 0
	}
	// Find the matching ']' for the link text, tracking nesting of [].
	textEnd := matchBracket(sp.src, open)
	if textEnd < 0 {
		return nil, 0
	}
	text := sp.src[open+1 : textEnd]
	after := textEnd + 1
	rest := sp.src[after:]

	// Reference style ("[text][id]", id may be empty) or shortcut ("[text]"): the
	// id part is "\s*?[...]" and, when absent, a following "(" means an inline link.
	if m := reInlineID.FindStringSubmatch(rest); m != nil {
		id := m[1]
		if id == "" {
			id = text // collapsed shortcut: [text][] -> use the text as the id
		}
		if el := sp.refLink(image, text, id); el != nil {
			return el, after + len(m[0]) - start
		}
		return nil, 0
	}
	if len(rest) == 0 || rest[0] != '(' {
		// Shortcut reference: [text] resolved against a matching definition.
		if el := sp.refLink(image, text, text); el != nil {
			return el, after - start
		}
		return nil, 0
	}

	// Inline link: "(url)" / "(url \"title\")" / "(<url> \"title\")".
	if el, end := sp.inlineLink(image, text, after); el != nil {
		return el, end - start
	}
	return nil, 0
}

// reInlineID matches kramdown's LINK_INLINE_ID_RE: optional whitespace then a
// bracketed reference id (which may be empty), used to detect a reference-style
// link's "[id]" that follows the link text.
var reInlineID = regexp.MustCompile(`(?s)^\s*\[([^\]]*)\]`)

// matchBracket returns the index of the ']' matching the '[' at openIdx, honouring
// nested brackets and backslash escapes, or -1 if unbalanced. A footnote marker
// ("[^id]") nested in the text is consumed whole by kramdown's footnote parser,
// which eats its ']' — so the marker's '[' is counted but its ']' is not, leaving
// the enclosing brackets unbalanced exactly as the gem's parse_link count does.
func matchBracket(s string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '[':
			if j := footnoteMarkerEnd(s, i); j >= 0 {
				depth++ // the marker's '[' still counts
				i = j   // skip its ']' without decrementing
				continue
			}
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// footnoteMarkerEnd reports the index of the ']' that closes a footnote marker
// "[^id]" (id being one or more word/hyphen characters) starting at i, or -1 if s
// at i is not a footnote marker.
func footnoteMarkerEnd(s string, i int) int {
	if i+2 >= len(s) || s[i] != '[' || s[i+1] != '^' {
		return -1
	}
	j := i + 2
	for j < len(s) && (isWordByte(s[j]) || s[j] == '-') {
		j++
	}
	if j > i+2 && j < len(s) && s[j] == ']' {
		return j
	}
	return -1
}

// reAngleDest matches an inline destination wrapped in angle brackets: "(<url>".
var reAngleDest = regexp.MustCompile(`(?s)^\(<(.*?)>`)

// parseInlineTitle ports kramdown's LINK_INLINE_TITLE_RE
// (/\s*?(["'])(.+?)\1\s*?\)/m): optional leading whitespace, a quote, a non-empty
// title up to the matching quote, optional whitespace, then the closing ')'. It
// returns the title, the offset in s just past the ')', and whether it matched. The
// title may span line breaks (as in a link broken across lines).
func parseInlineTitle(s string) (title string, end int, ok bool) {
	i := 0
	for i < len(s) && isTitleWS(s[i]) {
		i++
	}
	if i >= len(s) || (s[i] != '"' && s[i] != '\'') {
		return "", 0, false
	}
	q := s[i]
	contentStart := i + 1
	// Smallest closing quote (>=1 content char) that is followed, past optional
	// whitespace, by ')'. This mirrors the lazy .+? / \s*? backtracking.
	for c := contentStart + 1; c < len(s); c++ {
		if s[c] != q {
			continue
		}
		k := c + 1
		for k < len(s) && isTitleWS(s[k]) {
			k++
		}
		if k < len(s) && s[k] == ')' {
			return s[contentStart:c], k + 1, true
		}
	}
	return "", 0, false
}

// isTitleWS reports whether b is whitespace for the inline-title scanner (Ruby \s).
func isTitleWS(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}

// inlineLink parses the "(url)"/"(url \"title\")"/"(<url> \"title\")" tail of an
// inline link/image beginning at the '(' at parenIdx, returning the element and the
// absolute end position, or nil to fall back to literal text. It mirrors the gem's
// destination scanner: a parenthesised URL balances nested parens, and a title is
// introduced by whitespace immediately before a quote.
func (sp *spanParser) inlineLink(image bool, text string, parenIdx int) (*Element, int) {
	s := sp.src[parenIdx:]
	// Angle-bracketed destination: "(<url>", optionally followed immediately by ")".
	if m := reAngleDest.FindStringSubmatch(s); m != nil {
		url := m[1]
		off := len(m[0]) // position after '>'
		if off < len(s) && s[off] == ')' {
			return sp.buildLink(image, text, url, "", ""), parenIdx + off + 1
		}
		if title, tend, ok := parseInlineTitle(s[off:]); ok {
			return sp.buildLink(image, text, url, title, ""), parenIdx + off + tend
		}
		return nil, 0
	}
	// Bare destination: accumulate through the balancing ')' (tracking nested
	// parens) or stop at the whitespace introducing a quoted title.
	url, off, balanced := scanInlineDest(s)
	if balanced {
		return sp.buildLink(image, text, url, "", ""), parenIdx + off
	}
	if title, tend, ok := parseInlineTitle(s[off:]); ok {
		return sp.buildLink(image, text, url, title, ""), parenIdx + off + tend
	}
	return nil, 0
}

// scanInlineDest ports kramdown's parenthesis-balancing destination scanner. s
// begins at the '('. It returns the stripped destination, the offset within s just
// past the last consumed byte, and whether the parens balanced (a balanced scan is a
// complete titleless destination; an unbalanced one stopped before a quoted title).
func scanInlineDest(s string) (url string, off int, balanced bool) {
	var b strings.Builder
	nr := 0
	i := 0
	for {
		j, kind := scanParenStop(s, i)
		if j < 0 {
			break // no further stop token: mirror scan_until returning nil
		}
		b.WriteString(s[i:j])
		i = j
		switch kind {
		case ')':
			nr--
			if nr == 0 {
				goto done
			}
		case '(':
			nr++
		default: // whitespace before a quote
			goto done
		}
	}
done:
	raw := b.String()
	// Drop the leading '(' and the final consumed byte, then trim surrounding space,
	// exactly as "link_url[1..-2]; link_url.strip!".
	if len(raw) >= 2 {
		raw = raw[1 : len(raw)-1]
	} else {
		raw = ""
	}
	return strings.TrimSpace(raw), i, nr == 0
}

// scanParenStop finds, at or after index i in s, the next destination stop token:
// an unescaped '(' or ')', or a whitespace byte immediately followed by a quote. It
// returns the index just past the single-byte token and the token kind ('(' , ')'
// or ' ' for the pre-quote whitespace), or -1 if none is found.
func scanParenStop(s string, i int) (int, byte) {
	for k := i; k < len(s); k++ {
		switch s[k] {
		case '(':
			return k + 1, '('
		case ')':
			return k + 1, ')'
		case ' ', '\t', '\n':
			if k+1 < len(s) && (s[k+1] == '"' || s[k+1] == '\'') {
				return k + 1, ' '
			}
		}
	}
	return -1, 0
}

// refLink resolves a reference link/image against the harvested definitions.
func (sp *spanParser) refLink(image bool, text, id string) *Element {
	def, ok := sp.p.linkDefs[normalizeRef(id)]
	if !ok {
		return nil
	}
	return sp.buildLink(image, text, def.url, def.title, def.ial)
}

// buildLink builds an ElA or ElImg with the resolved destination/title; for a link
// the text is span-parsed into children (with nested links forbidden), for an image
// it becomes the raw alt text with backslash escapes reduced. A reference
// definition's IAL (ial) is applied first so its attributes precede href/title, as
// the gem's add_link does.
func (sp *spanParser) buildLink(image bool, text, url, title, ial string) *Element {
	if image {
		el := newEl(ElImg)
		if ial != "" {
			applyIALToElement(el, ial, sp.p.aldDefs)
		}
		el.setAttr("src", url)
		el.setAttr("alt", reduceEscaped(text))
		if title != "" {
			el.setAttr("title", title)
		}
		return el
	}
	el := newEl(ElA)
	if ial != "" {
		applyIALToElement(el, ial, sp.p.aldDefs)
	}
	el.setAttr("href", url)
	if title != "" {
		el.setAttr("title", title)
	}
	saved := sp.p.noLinks
	sp.p.noLinks = true
	el.Children = sp.p.parseSpans(text)
	sp.p.noLinks = saved
	return el
}

// reduceEscaped removes a backslash before any of kramdown's ESCAPED_CHARS,
// yielding the literal character (used for image alt text, which is the raw source
// between the brackets rather than the rendered spans).
func reduceEscaped(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isEscapedChar(s[i+1]) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// isEscapedChar reports whether c is one of kramdown's ESCAPED_CHARS
// (\.*_+`<>()[]{}#!:|"'$=-).
func isEscapedChar(c byte) bool {
	switch c {
	case '\\', '.', '*', '_', '+', '`', '<', '>', '(', ')', '[', ']',
		'{', '}', '#', '!', ':', '|', '"', '\'', '$', '=', '-':
		return true
	}
	return false
}

var (
	reAutoURL = regexp.MustCompile(`^<((?:https?|ftp|mailto):[^>\s]+)>`)
	// An email autolink's address excludes brackets and parentheses, so a bracketed
	// construct like <[a](b)> is left literal (and its inner markdown link parsed),
	// matching kramdown 2.5.2.
	reAutoEmail = regexp.MustCompile(`^<([^>\s@()\[\]]+@[^>\s()\[\]]+\.[^>\s()\[\]]+)>`)
)

// tryAutolink parses an <url>/<email> autolink at "<", returning the element and
// the number of source bytes consumed, or nil when the "<" opens neither. Raw
// inline HTML is handled separately by trySpanHTML (kramdown's parse_span_html).
func (sp *spanParser) tryAutolink() (*Element, int) {
	s := sp.src[sp.pos:]
	if m := reAutoURL.FindStringSubmatch(s); m != nil {
		url := m[1]
		el := newEl(ElA)
		el.setAttr("href", url)
		disp := url
		if strings.HasPrefix(url, "mailto:") {
			disp = url[len("mailto:"):]
		}
		t := newEl(ElText)
		t.Value = disp
		el.addChild(t)
		el.Options["autolink"] = true
		return el, len(m[0])
	}
	if m := reAutoEmail.FindStringSubmatch(s); m != nil {
		addr := m[1]
		el := newEl(ElA)
		el.setAttr("href", "mailto:"+addr)
		t := newEl(ElText)
		t.Value = addr
		el.addChild(t)
		el.Options["autolink"] = true
		el.Options["email"] = true
		return el, len(m[0])
	}
	return nil, 0
}
