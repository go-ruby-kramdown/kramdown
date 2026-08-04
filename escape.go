// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// reEntity matches an already-formed HTML entity (named, decimal or hex) which is
// passed through verbatim rather than having its "&" escaped.
var reEntity = regexp.MustCompile(`&(?:\w+|#[0-9]+|#[xX][0-9a-fA-F]+);`)

// escapeHTMLText escapes text content for HTML, leaving existing entities intact
// and converting <, >, and bare & the way kramdown does.
func escapeHTMLText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			if loc := reEntity.FindStringIndex(s[i:]); loc != nil && loc[0] == 0 {
				b.WriteString(s[i : i+loc[1]])
				i += loc[1] - 1
				continue
			}
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// escapeHTMLTextAll escapes text with kramdown's :all mode (escape_html's default):
// every <, > and & is escaped, with no entity pass-through. This is the mode the gem
// uses for code-span and code-block bodies, whose value is a literal string (an
// already-formed "&name;" is escaped to "&amp;name;", not preserved).
func escapeHTMLTextAll(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// convertTextEntities rewrites, in place, every recognised HTML entity inside the
// :text nodes of a span tree to the character it names, mirroring kramdown's default
// entity_output :as_char (an entity in span text is parsed to an :entity element and
// rendered as its character). Entities whose character is <, > or & keep their input
// form (kramdown emits the original for those), and an unrecognised entity is left
// verbatim (this port does not carry the gem's full ~2000-entry table). Code spans and
// raw-HTML spans hold their text in Value, not in :text children, so they are
// untouched — their contents stay literal exactly as the gem's escape_html leaves them.
func convertTextEntities(els []*Element) {
	for _, e := range els {
		if e.Type == ElText {
			// A backslash-escaped run (marked literal) is a verbatim character, not a
			// span-parsed entity, so it is left untouched.
			if lit, _ := e.Options["literal"].(bool); !lit {
				e.Value = convertEntityText(e.Value)
			}
			continue
		}
		// An autolink's display text is the pre-escaped URL/address the gem emits
		// verbatim (its "&amp;"/"&quot;" are already final), so its subtree is skipped.
		if auto, _ := e.Options["autolink"].(bool); auto {
			continue
		}
		if len(e.Children) > 0 {
			convertTextEntities(e.Children)
		}
	}
}

// convertEntityText applies the :as_char substitution to one text run.
func convertEntityText(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	for len(s) > 0 {
		loc := reHTMLEntity.FindStringSubmatchIndex(s)
		if loc == nil {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:loc[0]])
		name, dec, hex := "", "", ""
		if loc[2] >= 0 {
			name = s[loc[2]:loc[3]]
		}
		if loc[4] >= 0 {
			dec = s[loc[4]:loc[5]]
		}
		if loc[6] >= 0 {
			hex = s[loc[6]:loc[7]]
		}
		if ch, ok := entityDisplayChar(name, dec, hex); ok {
			b.WriteString(ch)
		} else {
			b.WriteString(s[loc[0]:loc[1]])
		}
		s = s[loc[1]:]
	}
	return b.String()
}

// entityDisplayChar returns the as_char character for a recognised entity, or false to
// keep its literal form: an unresolvable entity, or one whose character is <, > or &
// (which the gem emits in its original input form).
func entityDisplayChar(name, dec, hex string) (string, bool) {
	ch, ok := decodeEntity(name, dec, hex)
	if !ok {
		return "", false
	}
	switch ch {
	case "<", ">", "&":
		return "", false
	}
	return ch, true
}

// escapeHTMLAttr escapes an attribute value: &, < and " (kramdown escapes the
// double quote as &quot; and leaves single quotes literal).
func escapeHTMLAttr(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			if loc := reEntity.FindStringIndex(s[i:]); loc != nil && loc[0] == 0 {
				b.WriteString(s[i : i+loc[1]])
				i += loc[1] - 1
				continue
			}
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// escapeHref escapes a URL for an href/src attribute: bare & becomes &amp; (but an
// existing entity is preserved) and " becomes &quot;.
func escapeHref(s string) string {
	return escapeHTMLAttr(s)
}
