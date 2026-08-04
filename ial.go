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

// parseIAL tokenises a raw attribute-list string into ordered tokens. Bare words
// become "ref" tokens; ALD references are NOT expanded here — expansion is done by
// applyIALToElement, which mirrors kramdown's update_attr_with_ial by resolving all
// referenced ALDs first (so their attributes precede the IAL's own). The alds
// parameter is unused and kept only for call-site compatibility. The token order
// mirrors kramdown's (later classes/ids/keys override earlier; classes accumulate).
func parseIAL(raw string, _ map[string]string) []ialToken {
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
			// Bare word: an ALD reference (kramdown keeps every IAL ref in ial[:refs]);
			// an unknown one contributes no attributes but is still remembered (e.g.
			// "standalone", "toc", "footnotes").
			j := 0
			for j < len(s) && !isIALSpace(s[j]) {
				j++
			}
			toks = append(toks, ialToken{kind: "ref", val: s[:j]})
			s = s[j:]
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
// order (parser/kramdown.rb#update_attr_with_ial): every referenced ALD is resolved
// FIRST — recursively and in reference order, so its attributes precede the IAL's
// own — then the IAL's literal attributes are applied. Each attribute keeps the
// position of its first appearance (on the element or across the resolution), a
// class accumulates space-separated values, and an id or key takes its last value.
// The element's own bare-word references are recorded in Options["ial_refs"] (used
// by the toc/footnotes/standalone/auto_ids features).
func applyIALToElement(el *Element, raw string, alds map[string]string) {
	toks := parseIAL(raw, nil)
	// Record this element's own (top-level) references only.
	for _, t := range toks {
		if t.kind == "ref" {
			refs, _ := el.Options["ial_refs"].([]string)
			el.Options["ial_refs"] = append(refs, t.val)
		}
	}
	applyIALTokens(el, toks, alds, map[string]bool{})
}

// applyIALTokens folds the tokens of one IAL (or a referenced ALD) into el,
// resolving referenced ALDs before the token list's own attributes. active guards
// against a cyclic ALD reference (kramdown has no such guard and would recurse
// forever); acyclic references — the only kind in practice — apply exactly as the
// gem does.
func applyIALTokens(el *Element, toks []ialToken, alds map[string]string, active map[string]bool) {
	for _, t := range toks {
		if t.kind != "ref" {
			continue
		}
		if def, ok := alds[t.val]; ok && !active[t.val] {
			active[t.val] = true
			applyIALTokens(el, parseIAL(def, nil), alds, active)
			delete(active, t.val)
		}
	}
	for _, t := range toks {
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
