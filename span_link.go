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
// to literal text.
func (sp *spanParser) tryLink(image bool) (*Element, int) {
	start := sp.pos
	open := start
	if image {
		open = start + 1 // skip '!'
	}
	// Find the matching ']' for the link text, tracking nesting of [].
	textEnd := matchBracket(sp.src, open)
	if textEnd < 0 {
		return nil, 0
	}
	text := sp.src[open+1 : textEnd]
	after := textEnd + 1
	if after < len(sp.src) && sp.src[after] == '(' {
		// Inline link: (url "title").
		if el, n := sp.inlineLink(image, text, after); el != nil {
			return el, n - start
		}
		return nil, 0
	}
	if after < len(sp.src) && sp.src[after] == '[' {
		// Reference link: [text][id].
		idEnd := matchBracket(sp.src, after)
		if idEnd < 0 {
			return nil, 0
		}
		id := sp.src[after+1 : idEnd]
		if id == "" {
			id = text
		}
		if el := sp.refLink(image, text, id); el != nil {
			return el, idEnd + 1 - start
		}
		return nil, 0
	}
	// Shortcut reference: [text] with a matching definition.
	if el := sp.refLink(image, text, text); el != nil {
		return el, after - start
	}
	return nil, 0
}

// matchBracket returns the index of the ']' matching the '[' at openIdx, honouring
// nested brackets and backslash escapes, or -1 if unbalanced.
func matchBracket(s string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '[':
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

var reInlineDest = regexp.MustCompile(`^\((.*)$`)

// inlineLink parses the "(url "title")" portion of an inline link starting at the
// '(' at parenIdx.
func (sp *spanParser) inlineLink(image bool, text string, parenIdx int) (*Element, int) {
	// Find the matching ')' tracking nested parens; allow a "(url "title")" with a
	// quoted title that may itself contain parens.
	rest := sp.src[parenIdx+1:]
	closeRel := matchParenClose(rest)
	if closeRel < 0 {
		return nil, 0
	}
	inside := rest[:closeRel]
	url, title := splitDestTitle(inside)
	end := parenIdx + 1 + closeRel + 1
	el := sp.buildLink(image, text, unescapeLinkText(url), title)
	return el, end
}

// matchParenClose returns the index of the ')' that closes the destination,
// tracking nested parens and skipping a quoted title.
func matchParenClose(s string) int {
	depth := 0
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		switch c {
		case '\\':
			i++
		case '"', '\'':
			inQuote = c
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// splitDestTitle splits "url" or "url \"title\"" into the destination and the
// title (quotes stripped), handling leading/trailing spaces.
func splitDestTitle(s string) (string, string) {
	s = strings.TrimSpace(s)
	// Find a title introduced by an unescaped quote with a preceding space.
	for i := 0; i < len(s); i++ {
		if (s[i] == '"' || s[i] == '\'') && i > 0 && s[i-1] == ' ' {
			q := s[i]
			rest := s[i+1:]
			if end := strings.IndexByte(rest, q); end >= 0 {
				url := strings.TrimSpace(s[:i])
				title := rest[:end]
				return url, title
			}
		}
	}
	return strings.TrimSpace(s), ""
}

// refLink resolves a reference link/image against the harvested definitions.
func (sp *spanParser) refLink(image bool, text, id string) *Element {
	def, ok := sp.p.linkDefs[normalizeRef(id)]
	if !ok {
		return nil
	}
	return sp.buildLink(image, text, def.url, def.title)
}

// buildLink builds an ElA or ElImg with the resolved destination/title; for a link
// the text is span-parsed into children, for an image it becomes the alt text.
func (sp *spanParser) buildLink(image bool, text, url, title string) *Element {
	if image {
		el := newEl(ElImg)
		el.setAttr("src", url)
		el.setAttr("alt", plainText(sp.p.parseSpans(text)))
		if title != "" {
			el.setAttr("title", title)
		}
		return el
	}
	el := newEl(ElA)
	el.setAttr("href", url)
	if title != "" {
		el.setAttr("title", title)
	}
	el.Children = sp.p.parseSpans(text)
	return el
}

// unescapeLinkText removes backslash escapes from a link destination.
func unescapeLinkText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isEscapable(s[i+1]) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

var (
	reAutoURL   = regexp.MustCompile(`^<((?:https?|ftp|mailto):[^>\s]+)>`)
	reAutoEmail = regexp.MustCompile(`^<([^>\s@]+@[^>\s]+\.[^>\s]+)>`)
	reHTMLSpan  = regexp.MustCompile(`^<(/?[a-zA-Z][a-zA-Z0-9]*(?:\s[^>]*)?/?)>`)
)

// tryAutolinkOrHTML parses an <url>/<email> autolink or a raw inline HTML tag at
// "<".
func (sp *spanParser) tryAutolinkOrHTML() (*Element, int) {
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
	if m := reHTMLSpan.FindStringSubmatch(s); m != nil {
		el := newEl(ElRawHTMLSpan)
		el.Value = "<" + m[1] + ">"
		return el, len(m[0])
	}
	return nil, 0
}

// plainText renders span elements to their plain-text (alt) form, used for image
// alt attributes and abbreviation scanning.
func plainText(els []*Element) string {
	var b strings.Builder
	for _, e := range els {
		switch e.Type {
		case ElText, ElCodespan:
			b.WriteString(e.Value)
		case ElFootnoteRef:
			if name, ok := e.Options["name"].(string); ok {
				b.WriteString(name)
			}
		default:
			b.WriteString(plainText(e.Children))
		}
	}
	return b.String()
}
