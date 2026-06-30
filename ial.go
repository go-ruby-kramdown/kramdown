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
			if def, ok := alds[word]; ok {
				toks = append(toks, parseIAL(def, alds)...)
			}
			// else ignored (kramdown drops unknown bare words)
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

// applyIALToElement applies a raw IAL string to el's HTML attributes, merging
// classes and overriding ids/keys, matching kramdown's emission order: keys (in
// order), then a single class attribute (space-joined, later-first), then id.
func applyIALToElement(el *Element, raw string, alds map[string]string) {
	toks := parseIAL(raw, alds)
	var classes []string
	id := ""
	var keys []ialToken
	for _, t := range toks {
		switch t.kind {
		case "class":
			classes = append(classes, t.val)
		case "id":
			id = t.val
		case "key":
			// replace existing key of same name
			replaced := false
			for i := range keys {
				if keys[i].name == t.name {
					keys[i].val = t.val
					replaced = true
				}
			}
			if !replaced {
				keys = append(keys, t)
			}
		}
	}
	// kramdown merges new classes after existing class attribute value.
	if len(classes) > 0 {
		existing, _ := el.getAttr("class")
		all := classes
		if existing != "" {
			all = append([]string{existing}, classes...)
		}
		el.setAttr("class", strings.Join(all, " "))
	}
	for _, k := range keys {
		el.setAttr(k.name, k.val)
	}
	if id != "" {
		el.setAttr("id", id)
	}
}

// htmlBlockTags is the set of block-level HTML tags whose opening line triggers a
// raw HTML passthrough block.
var htmlBlockTags = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"details": true, "dialog": true, "dd": true, "div": true, "dl": true,
	"dt": true, "fieldset": true, "figcaption": true, "figure": true,
	"footer": true, "form": true, "h1": true, "h2": true, "h3": true,
	"h4": true, "h5": true, "h6": true, "header": true, "hgroup": true,
	"hr": true, "li": true, "main": true, "nav": true, "ol": true, "p": true,
	"pre": true, "section": true, "table": true, "ul": true, "script": true,
	"style": true, "iframe": true, "noscript": true,
}

// reHTMLOpen matches a leading "<tag" or "</tag" or "<!--".
var reHTMLOpen = regexp.MustCompile(`^ {0,3}</?([a-zA-Z][a-zA-Z0-9]*)`)

// isHTMLBlockStart reports whether line opens a raw HTML block.
func isHTMLBlockStart(line string) bool {
	if strings.HasPrefix(strings.TrimLeft(line, " "), "<!--") {
		return true
	}
	m := reHTMLOpen.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	return htmlBlockTags[strings.ToLower(m[1])]
}

// parseHTMLBlock consumes a raw HTML block verbatim until a blank line (kramdown's
// default html block handling for our supported subset) and stores it on an
// ElHTMLBlock element.
func (p *parser) parseHTMLBlock(lines []string, start int, parent *Element) int {
	var buf []string
	i := start
	// HTML comment block: gather until "-->".
	if strings.HasPrefix(strings.TrimLeft(lines[start], " "), "<!--") {
		for i < len(lines) {
			buf = append(buf, lines[i])
			if strings.Contains(lines[i], "-->") {
				i++
				break
			}
			i++
		}
	} else {
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			buf = append(buf, lines[i])
			i++
		}
	}
	hb := newEl(ElHTMLBlock)
	hb.Value = strings.Join(buf, "\n")
	parent.addChild(hb)
	return i - start
}
