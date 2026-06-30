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
var reEntity = regexp.MustCompile(`&(?:[a-zA-Z][a-zA-Z0-9]*|#[0-9]+|#[xX][0-9a-fA-F]+);`)

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
