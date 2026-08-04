// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// reALDName matches an ALD definition "{:name: ...}" capturing the name and the
// remaining attribute string.
var reALDName = regexp.MustCompile(`^([\w-]+):\s*(.*)$`)

// splitALD reports whether ial is an ALD definition ("name: attrs"), returning the
// name and the raw attribute string. A bare class/id IAL (".cls", "#id", "key=v")
// is not an ALD.
func splitALD(ial string) (name, attrs string, ok bool) {
	// reALDName requires a leading bare word followed by a colon, so a class/id IAL
	// (".cls", "#id") never matches and is not misread as an ALD.
	m := reALDName.FindStringSubmatch(strings.TrimSpace(ial))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// reLeadingItemIAL matches an inline-attribute list at the very start of a
// list-item, definition or term line, capturing its body.
var reLeadingItemIAL = regexp.MustCompile(`^[ \t]*\{:((?:\\\}|[^}])*)\}[ \t]*`)

// stripLeadingItemIAL detects a leading "{:…}" IAL on a list-item/definition/term
// line, returning its attribute body and the remaining line text. A "{::…}"
// extension, a "{:/…}" stop tag and an ALD definition ("name: …") are not
// item IALs.
func stripLeadingItemIAL(s string) (body, rest string, ok bool) {
	m := reLeadingItemIAL.FindStringSubmatch(s)
	if m == nil {
		return "", "", false
	}
	body = m[1]
	if strings.HasPrefix(body, ":") || strings.HasPrefix(body, "/") ||
		reALDName.MatchString(strings.TrimSpace(body)) {
		return "", "", false
	}
	return body, s[len(m[0]):], true
}

// ialToken is one parsed attribute-list token.
type ialToken struct {
	kind string // "class", "id", "key", "ref", "ignore"
	name string
	val  string
}

// reIALKey matches a key="value" or key='value' pair.
var reIALKey = regexp.MustCompile(`^([\w:-]+)=("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')`)

// parseIAL tokenises a raw attribute-list string into ordered tokens, resolving
// ALD references against the supplied table. The token order mirrors kramdown's
// (later classes/ids/keys override earlier; classes accumulate).
func parseIAL(raw string, alds map[string]string) []ialToken {
	var toks []ialToken
	s := strings.TrimSpace(raw)
	for {
		s = strings.TrimLeft(s, " \t")
		if s == "" {
			break
		}
		switch {
		case s[0] == '.':
			j := 1
			for j < len(s) && !isIALSpace(s[j]) && s[j] != '.' && s[j] != '#' {
				j++
			}
			toks = append(toks, ialToken{kind: "class", val: s[1:j]})
			s = s[j:]
		case s[0] == '#':
			j := 1
			for j < len(s) && !isIALSpace(s[j]) && s[j] != '.' && s[j] != '#' {
				j++
			}
			toks = append(toks, ialToken{kind: "id", val: s[1:j]})
			s = s[j:]
		default:
			if m := reIALKey.FindStringSubmatch(s); m != nil {
				toks = append(toks, ialToken{kind: "key", name: m[1], val: unquoteIAL(m[2])})
				s = s[len(m[0]):]
				continue
			}
			// Bare word: an ALD reference if known, else ignored.
			j := 0
			for j < len(s) && !isIALSpace(s[j]) {
				j++
			}
			word := s[:j]
			s = s[j:]
			// Record the bare word as a reference (kramdown keeps every IAL ref in
			// ial[:refs]); a matching ALD is additionally expanded in place, an unknown
			// one contributes no attributes but is still remembered (e.g. "standalone").
			toks = append(toks, ialToken{kind: "ref", val: word})
			if def, ok := alds[word]; ok {
				toks = append(toks, parseIAL(def, alds)...)
			}
		}
	}
	return toks
}

// isIALSpace reports whether c terminates an IAL token.
func isIALSpace(c byte) bool { return c == ' ' || c == '\t' }

// unquoteIAL strips the surrounding quotes from a key value and unescapes \" / \'.
// The caller (reIALKey) only ever passes a fully quoted token, so s is at least
// the two delimiter characters long.
func unquoteIAL(s string) string {
	q := s[0]
	inner := s[1 : len(s)-1]
	inner = strings.ReplaceAll(inner, "\\"+string(q), string(q))
	return inner
}

// applyIALToElement applies a raw IAL string to el's HTML attributes in kramdown's
// order: each attribute keeps the position of its first appearance in the IAL (or
// its pre-existing position on the element), a class accumulates space-separated
// values, and an id or key takes its last value.
func applyIALToElement(el *Element, raw string, alds map[string]string) {
	for _, t := range parseIAL(raw, alds) {
		switch t.kind {
		case "class":
			if existing, ok := el.getAttr("class"); ok {
				el.setAttr("class", existing+" "+t.val)
			} else {
				el.setAttr("class", t.val)
			}
		case "id":
			el.setAttr("id", t.val)
		case "key":
			el.setAttr(t.name, t.val)
		case "ref":
			refs, _ := el.Options["ial_refs"].([]string)
			el.Options["ial_refs"] = append(refs, t.val)
		}
	}
}

// ialHasRef reports whether el's applied IAL recorded the given bare-word
// reference (e.g. "standalone").
func ialHasRef(el *Element, ref string) bool {
	refs, _ := el.Options["ial_refs"].([]string)
	for _, r := range refs {
		if r == ref {
			return true
		}
	}
	return false
}

// isHTMLBlockStart reports whether line, considered on its own, opens a block-level
// raw HTML construct (a comment or a non-span-only start tag). It mirrors kramdown's
// parse_block_html accept test for a single line; callers needing multi-line accuracy
// (paragraph interruption, block dispatch) use parser.blockHTMLStart instead.
func isHTMLBlockStart(line string) bool {
	return htmlBlockStartsAt(line + "\n")
}
