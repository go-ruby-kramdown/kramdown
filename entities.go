// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"strconv"
	"strings"
	"unicode"
)

// defaultSmartQuotes is the kramdown default :smart_quotes array — the entity each
// smart-quote position (lsquo, rsquo, ldquo, rdquo) maps to unless overridden.
var defaultSmartQuotes = [4]string{"lsquo", "rsquo", "ldquo", "rdquo"}

// smartQuoteIndex maps a smart-quote element's value to its :smart_quotes array
// index (kramdown's SMART_QUOTE_INDICES).
var smartQuoteIndex = map[string]int{"lsquo": 0, "rsquo": 1, "ldquo": 2, "rdquo": 3}

// entityLookup resolves an entity reference — exactly one of name / dec / hex is
// non-empty — to its code point and canonical name, mirroring the gem's
// Kramdown::Utils::Entities.entity. It returns ok=false for an unknown named entity,
// or for a numeric reference that overflows or names an invalid (beyond U+10FFFF)
// code point, so the caller applies kramdown's amp fallback. For a resolved numeric
// reference the canonical name is the last table row for that code point (empty when
// the point has no named entity, i.e. the gem's nil name).
func entityLookup(name, dec, hex string) (cp int, cname string, ok bool) {
	switch {
	case dec != "":
		return numericEntity(dec, 10)
	case hex != "":
		return numericEntity(hex, 16)
	default:
		if p, found := entityNameToCP[name]; found {
			return p, name, true
		}
		return 0, "", false
	}
}

// numericEntity resolves a numeric character reference (digits in the given base) to
// its code point and canonical name, rejecting an overflowing or out-of-Unicode-range
// value. The reference is unsigned (the entity regex admits no sign), so the only
// failure modes are overflow and a code point above U+10FFFF.
func numericEntity(digits string, base int) (cp int, cname string, ok bool) {
	n, err := strconv.ParseInt(digits, base, 32)
	if err != nil || n > unicode.MaxRune {
		return 0, "", false
	}
	return int(n), entityCPToName[int(n)], true
}

// isEscapeMapKey reports whether s is one of the four characters kramdown keeps in
// its ESCAPE_MAP ('<', '>', '&', '"'), which decide when entity_to_str must fall
// back from :as_char to the named/numeric form.
func isEscapeMapKey(s string) bool {
	switch s {
	case "<", ">", "&", "\"":
		return true
	}
	return false
}

// entityToStr renders one entity (code point cp, canonical name cname — empty when
// the entity has no name) under the given entity_output mode, mirroring
// Kramdown::Utils::Html#entity_to_str. original is the entity's literal input form
// (empty for a generated entity such as a smart quote or the amp fallback).
func entityToStr(cp int, cname, original, mode string) string {
	c := string(rune(cp))
	if mode == "as_char" && (c == "\"" || !isEscapeMapKey(c)) {
		return c
	}
	if (mode == "as_input" || mode == "as_char") && original != "" {
		return original
	}
	if (mode == "symbolic" || isEscapeMapKey(c)) && cname != "" {
		return "&" + cname + ";"
	}
	return "&#" + strconv.Itoa(cp) + ";"
}

// applyEntityOutput rewrites, in place, every recognised HTML entity inside the
// :text nodes of a span tree to its entity_output form (kramdown parses each such
// reference to an :entity element and renders it via entity_to_str). A
// backslash-escaped (literal) text run is a verbatim character, not a parsed entity,
// so it is left untouched, as is an autolink's already-escaped display text.
func applyEntityOutput(els []*Element, mode string) {
	for _, e := range els {
		if e.Type == ElText {
			if lit, _ := e.Options["literal"].(bool); !lit {
				e.Value = rewriteEntityText(e.Value, mode)
			}
			continue
		}
		if auto, _ := e.Options["autolink"].(bool); auto {
			continue
		}
		if len(e.Children) > 0 {
			applyEntityOutput(e.Children, mode)
		}
	}
}

// rewriteEntityText applies the entity_output substitution to one text run: each
// substring matching HTML_ENTITY_RE is resolved and rendered through entityToStr,
// while an unresolvable reference (an unknown named entity or an out-of-range code
// point) degrades to the amp entity followed by the remainder of the match — exactly
// kramdown's parse_html_entity rescue (Element(:entity, entity('amp')) + the matched
// text minus its leading '&'). All other characters, including a bare '&', are left
// for the text escaper.
func rewriteEntityText(s, mode string) string {
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
		matched := s[loc[0]:loc[1]]
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
		if cp, cname, ok := entityLookup(name, dec, hex); ok {
			b.WriteString(entityToStr(cp, cname, matched, mode))
		} else {
			b.WriteString(entityToStr(entityNameToCP["amp"], "amp", "", mode))
			b.WriteString(matched[1:])
		}
		s = s[loc[1]:]
	}
	return b.String()
}
