// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"sort"
	"strings"
)

var reFootnoteMarker = regexp.MustCompile(`^\[\^([\w-]+)\]`)

// tryFootnoteRef parses a "[^id]" footnote marker, emitting an ElFootnoteRef when
// a matching definition exists (otherwise the marker is left literal).
func (sp *spanParser) tryFootnoteRef() (*Element, int) {
	m := reFootnoteMarker.FindStringSubmatch(sp.src[sp.pos:])
	if m == nil {
		return nil, 0
	}
	id := m[1]
	if _, ok := sp.p.footDefs[id]; !ok {
		// An undefined footnote reference is left literal, mirroring kramdown, which
		// also records a warning on the document.
		sp.p.warn("Footnote definition for '" + id + "' not found")
		return nil, 0
	}
	el := newEl(ElFootnoteRef)
	el.Options["name"] = id
	return el, len(m[0])
}

// applyAbbreviations walks a span tree's text nodes and wraps occurrences of any
// defined abbreviation in an ElAbbr element. Abbreviations are matched on word
// boundaries, longest-first, case-sensitively (as kramdown does).
func applyAbbreviations(els []*Element, defs map[string]abbrevDef) []*Element {
	if len(defs) == 0 {
		return els
	}
	// Order keys longest-first so overlapping abbreviations prefer the longer match.
	keys := make([]string, 0, len(defs))
	for k := range defs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	var out []*Element
	for _, e := range els {
		switch e.Type {
		case ElText:
			out = append(out, splitAbbrevText(e.Value, keys, defs)...)
		case ElCodespan, ElRawHTMLSpan, ElTypographicSym, ElBr, ElFootnoteRef, ElImg:
			out = append(out, e)
		default:
			e.Children = applyAbbreviations(e.Children, defs)
			out = append(out, e)
		}
	}
	return out
}

// splitAbbrevText splits a text run into text/abbr elements at every abbreviation
// occurrence (whitespace in the abbreviation matches any whitespace run).
func splitAbbrevText(s string, keys []string, defs map[string]abbrevDef) []*Element {
	for _, k := range keys {
		pat := abbrevPattern(k)
		if loc := pat.FindStringIndex(s); loc != nil && loc[1] > loc[0] {
			var out []*Element
			if loc[0] > 0 {
				t := newEl(ElText)
				t.Value = s[:loc[0]]
				out = append(out, splitAbbrevText(t.Value, keys, defs)...)
			}
			ab := newEl(ElAbbr)
			ab.Value = s[loc[0]:loc[1]]
			ab.Options["title"] = defs[k].title
			if defs[k].attr != "" {
				applyIALToElement(ab, defs[k].attr, nil)
			}
			out = append(out, ab)
			if loc[1] < len(s) {
				out = append(out, splitAbbrevText(s[loc[1]:], keys, defs)...)
			}
			return out
		}
	}
	t := newEl(ElText)
	t.Value = s
	return []*Element{t}
}

// abbrevPattern builds a word-boundary regexp for an abbreviation, allowing any
// whitespace run where the key has a space (so a line-wrapped abbreviation still
// matches).
func abbrevPattern(k string) *regexp.Regexp {
	parts := strings.Fields(k)
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	body := strings.Join(parts, `\s+`)
	// Word boundaries: kramdown requires the abbreviation to stand as a token.
	return regexp.MustCompile(`(?:\b|(?:^))` + body + `\b`)
}
