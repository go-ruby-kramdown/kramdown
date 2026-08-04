// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// This file ports the low-level HTML tokenisation that kramdown's
// Kramdown::Parser::Html performs with a StringScanner and a handful of REXML-derived
// regexps. Go's RE2 engine rejects the atomic groups and back-references those regexps
// use (HTML_TAG_RE / HTML_ATTRIBUTE_RE), so the tag and attribute matchers are
// hand-written to reproduce the exact accept/reject behaviour, while the simpler
// delimiter-terminated constructs (comment, PI, CDATA) reuse straightforward scans.

// reUNAME mirrors REXML's UNAME_STR: an optional namespace prefix then a name, each
// beginning with a letter/underscore and continuing with letters, digits, '-', '.'
// or '_'.
const reUNAME = `(?:[[:alpha:]_][-[:alnum:]._]*:)?[[:alpha:]_][-[:alnum:]._]*`

var (
	reUNAMEAnchor = regexp.MustCompile(`^` + reUNAME)
	reWordAnchor  = regexp.MustCompile(`^\w+`) // an unquoted attribute value (\p{Word}+)
	// reHTMLRawStart marks the next position where raw-HTML tokenisation must stop to
	// examine a construct: a start/close tag, comment, PI or CDATA opener.
	reHTMLRawStart = regexp.MustCompile(`<(?:` + reUNAME + `|/|!--|\?|!\[CDATA\[)`)
	// reTrailingWS matches kramdown's TRAILING_WHITESPACE ("[ \t]*\n") at the scanner.
	reTrailingWS = regexp.MustCompile(`^[ \t]*\n`)
)

// htmlScanner is a minimal port of the Ruby StringScanner subset kramdown's HTML
// parser relies on.
type htmlScanner struct {
	s   string
	pos int
}

// eos reports whether the scanner has consumed the whole string.
func (sc *htmlScanner) eos() bool { return sc.pos >= len(sc.s) }

// rest returns the unconsumed remainder.
func (sc *htmlScanner) rest() string { return sc.s[sc.pos:] }

// terminate advances the scanner to the end of the string.
func (sc *htmlScanner) terminate() { sc.pos = len(sc.s) }

// getch consumes and returns the next byte as a string ("" at end of input).
func (sc *htmlScanner) getch() string {
	if sc.eos() {
		return ""
	}
	c := sc.s[sc.pos : sc.pos+1]
	sc.pos++
	return c
}

// scanRE matches the anchored regexp at the current position, advancing past and
// returning the match on success, or "" without moving otherwise.
func (sc *htmlScanner) scanRE(re *regexp.Regexp) (string, bool) {
	loc := re.FindStringIndex(sc.rest())
	if loc == nil || loc[0] != 0 {
		return "", false
	}
	m := sc.s[sc.pos : sc.pos+loc[1]]
	sc.pos += loc[1]
	return m, true
}

// scanUntilRawStart consumes text up to (but not including) the next reHTMLRawStart
// match, returning that text and true; if none remains it does not move and returns
// false (mirroring StringScanner#scan_until returning nil).
func (sc *htmlScanner) scanUntilRawStart() (string, bool) {
	loc := reHTMLRawStart.FindStringIndex(sc.rest())
	if loc == nil {
		return "", false
	}
	text := sc.s[sc.pos : sc.pos+loc[0]]
	sc.pos += loc[0]
	return text, true
}

// matchStartTag matches kramdown's HTML_TAG_RE anchored at s[0], returning the tag
// name, the raw attribute substring, whether it is self-closed, the total byte length
// of the match, and whether it matched.
func matchStartTag(s string) (name, attrs string, selfClose bool, n int, ok bool) {
	if len(s) < 2 || s[0] != '<' {
		return "", "", false, 0, false
	}
	nm := reUNAMEAnchor.FindString(s[1:])
	if nm == "" {
		return "", "", false, 0, false
	}
	name = nm
	p := 1 + len(nm)
	attrStart := p
	// Zero or more attributes, each introduced by at least one whitespace character.
	for {
		ws := skipHTMLSpace(s, p)
		if ws == p {
			break // no whitespace before the next attribute
		}
		am := reUNAMEAnchor.FindString(s[ws:])
		if am == "" {
			break
		}
		q := ws + len(am)
		// Optional "= value".
		afterEq := skipHTMLSpace(s, q)
		if afterEq < len(s) && s[afterEq] == '=' {
			v := skipHTMLSpace(s, afterEq+1)
			if v < len(s) && (s[v] == '"' || s[v] == '\'') {
				quote := s[v]
				end := strings.IndexByte(s[v+1:], quote)
				if end < 0 {
					// Unterminated quote: the attribute (with value) fails to match, so the
					// whole tag cannot close correctly — reject it.
					return "", "", false, 0, false
				}
				q = v + 1 + end + 1
			} else if wm := reWordAnchor.FindString(s[v:]); wm != "" {
				q = v + len(wm)
			} else {
				// "=" with no valid value: reject the tag.
				return "", "", false, 0, false
			}
		}
		p = q
	}
	attrs = s[attrStart:p]
	e := skipHTMLSpace(s, p)
	if e < len(s) && s[e] == '/' {
		selfClose = true
		e++
	}
	if e < len(s) && s[e] == '>' {
		return name, attrs, selfClose, e + 1, true
	}
	return "", "", false, 0, false
}

// skipHTMLSpace returns the index of the first non-whitespace byte at or after i,
// treating the ASCII whitespace set Ruby's \s matches (space, tab, newline, carriage
// return, form feed, vertical tab).
func skipHTMLSpace(s string, i int) int {
	for i < len(s) {
		switch s[i] {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			i++
		default:
			return i
		}
	}
	return i
}

// reHTMLAttr parses one attribute of HTML_ATTRIBUTE_RE. RE2 cannot express the
// back-reference for matched quotes, so a quoted value is captured by the manual
// parseHTMLAttributes below; this regexp only recognises the name and an optional
// unquoted value.
var reHTMLAttrName = regexp.MustCompile(`^\s*(` + reUNAME + `)`)

// parseHTMLAttributes reproduces kramdown's parse_html_attributes: it scans the
// attribute substring into an ordered attribute slice, lower-casing names when the
// element is a known HTML element (inHTMLTag), taking a bare name as an empty value,
// and letting a later duplicate overwrite an earlier one (keeping first position).
func parseHTMLAttributes(str string, inHTMLTag bool) []Attr {
	var attrs []Attr
	set := func(name, val string) {
		for i := range attrs {
			if attrs[i].Name == name {
				attrs[i].Val = val
				return
			}
		}
		attrs = append(attrs, Attr{Name: name, Val: val})
	}
	i := 0
	for i < len(str) {
		m := reHTMLAttrName.FindStringSubmatchIndex(str[i:])
		if m == nil {
			break
		}
		name := str[i+m[2] : i+m[3]]
		i += m[1]
		if inHTMLTag {
			name = strings.ToLower(name)
		}
		// Optional "= value".
		val := ""
		j := skipHTMLSpace(str, i)
		if j < len(str) && str[j] == '=' {
			v := skipHTMLSpace(str, j+1)
			if v < len(str) && (str[v] == '"' || str[v] == '\'') {
				quote := str[v]
				end := strings.IndexByte(str[v+1:], quote)
				if end < 0 {
					// Unterminated: consume the rest as the value (kramdown's non-greedy
					// match would fail this attribute, but the tag matcher already rejected
					// such tags, so this branch is defensive).
					val = str[v+1:]
					i = len(str)
				} else {
					val = str[v+1 : v+1+end]
					i = v + 1 + end + 1
				}
			} else if wm := reWordAnchor.FindString(str[v:]); wm != "" {
				val = wm
				i = v + len(wm)
			}
		}
		set(name, val)
	}
	return attrs
}
