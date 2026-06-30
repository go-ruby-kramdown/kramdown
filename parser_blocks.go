// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// harvestDefinitions removes link, abbreviation, ALD and footnote definitions from
// the line stream in a pre-pass (kramdown collects these globally before block
// parsing) and returns the remaining lines.
func (p *parser) harvestDefinitions(lines []string) []string {
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		// Footnote definition: "[^id]: ...". Checked before the link-reference
		// definition because "[^x]" also satisfies the looser "[id]" link pattern.
		if m := reFootnoteDef.FindStringSubmatch(line); m != nil {
			id := m[1]
			var body []string
			first := strings.TrimLeft(m[2], " \t")
			if first != "" {
				body = append(body, first)
			}
			i++
			// Continuation lines: blank lines or 4-space-indented lines belong to the
			// note.
			for i < len(lines) {
				l := lines[i]
				if strings.TrimSpace(l) == "" {
					body = append(body, "")
					i++
					continue
				}
				if strings.HasPrefix(l, "    ") || strings.HasPrefix(l, "\t") {
					body = append(body, stripIndent(expandTabs(l), 4))
					i++
					continue
				}
				break
			}
			// Trim trailing blanks.
			for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
				body = body[:len(body)-1]
			}
			fn := newEl(ElFootnoteDef)
			p.parseBlocks(body, fn)
			// An IAL right after attaches to the definition (ignored for HTML here).
			if i < len(lines) {
				if reBlockIAL.MatchString(strings.TrimRight(lines[i], " \t")) {
					i++
				}
			}
			p.footDefs[id] = fn
			continue
		}
		// Link/image reference definition: "[id]: url "title"" (title may continue
		// on the next line).
		if m := reLinkDef.FindStringSubmatch(line); m != nil {
			id := normalizeRef(m[1])
			url := m[2]
			title := m[3]
			if title == "" && i+1 < len(lines) {
				if tm := reLinkDefTitle.FindStringSubmatch(lines[i+1]); tm != nil {
					title = tm[1]
					i++
				}
			}
			p.linkDefs[id] = linkDef{url: stripURLAngles(url), title: unquoteTitle(title)}
			i++
			continue
		}
		// Abbreviation definition: "*[text]: title".
		if m := reAbbrevDef.FindStringSubmatch(line); m != nil {
			text := m[1]
			title := strings.TrimSpace(m[2])
			def := abbrevDef{title: title}
			// An IAL directly under the definition augments it.
			if i+1 < len(lines) {
				if am := reBlockIAL.FindStringSubmatch(strings.TrimRight(lines[i+1], " \t")); am != nil {
					def.attr = am[1]
					i++
				}
			}
			p.abbrevs[text] = def
			i++
			continue
		}
		out = append(out, line)
		i++
	}
	return out
}

var (
	reLinkDef      = regexp.MustCompile(`^ {0,3}\[([^\]]+)\]:\s+(\S+)(?:\s+["'(](.*)["')])?\s*$`)
	reLinkDefTitle = regexp.MustCompile(`^\s+["'(](.*)["')]\s*$`)
	reAbbrevDef    = regexp.MustCompile(`^\*\[([^\]]+)\]:(.*)$`)
	reFootnoteDef  = regexp.MustCompile(`^ {0,3}\[\^([^\]]+)\]:(.*)$`)
)

// normalizeRef lowercases and collapses internal whitespace of a reference id the
// way kramdown matches link references case-insensitively.
func normalizeRef(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
}

// stripURLAngles removes surrounding <...> from a reference URL.
func stripURLAngles(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "<") && strings.HasSuffix(u, ">") {
		return u[1 : len(u)-1]
	}
	return u
}

// unquoteTitle strips matching surrounding quotes/parens from a captured title.
func unquoteTitle(t string) string {
	if t == "" {
		return ""
	}
	return t
}

// parseList parses an ordered or unordered list starting at lines[start].
func (p *parser) parseList(lines []string, start int, parent *Element) int {
	first := lines[start]
	ordered := reOLItem.MatchString(first)
	listType := ElUL
	if ordered {
		listType = ElOL
	}
	list := newEl(listType)
	i := start
	loose := false
	type itemData struct {
		lines     []string
		looseItem bool // blank separator seen inside the item
	}
	var items []itemData
	for i < len(lines) {
		// Skip blank lines between items (they only affect looseness, already
		// recorded on the preceding item) so a blank-separated sibling continues
		// the same list rather than starting a new one.
		if strings.TrimSpace(lines[i]) == "" {
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			nextIsItem := j < len(lines) && ((ordered && reOLItem.MatchString(lines[j])) ||
				(!ordered && reULItem.MatchString(lines[j])))
			if nextIsItem {
				if len(items) > 0 {
					items[len(items)-1].looseItem = true
					loose = true
				}
				i = j
				continue
			}
			break
		}
		line := lines[i]
		var m []string
		if ordered {
			m = reOLItem.FindStringSubmatch(line)
		} else {
			m = reULItem.FindStringSubmatch(line)
		}
		if m == nil {
			break
		}
		marker := m[2]
		gap := m[3]
		content := m[4]
		// kramdown caps the content indent at marker+1 space when the gap is large
		// (a "big space" starts a code block); for the common case the indent is
		// marker width plus the following spaces.
		indent := len(m[1]) + len(marker) + len(gap)
		it := itemData{lines: []string{content}}
		i++
		freshPara := false // the next indented line starts a blank-separated paragraph
		// Gather continuation / nested / paragraph lines for this item.
		for i < len(lines) {
			l := lines[i]
			if strings.TrimSpace(l) == "" {
				// Look past the blank run.
				j := i
				for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
					j++
				}
				if j >= len(lines) {
					break
				}
				nxt := lines[j]
				indented := strings.HasPrefix(nxt, strings.Repeat(" ", indent))
				nextIsItem := (ordered && reOLItem.MatchString(nxt)) || (!ordered && reULItem.MatchString(nxt))
				if indented {
					// Blank then indented content: a paragraph continuation (loose item).
					it.looseItem = true
					loose = true
					it.lines = append(it.lines, "")
					i++
					freshPara = true
					continue
				}
				if nextIsItem {
					// Blank then a sibling marker: the list stays but this item is loose.
					it.looseItem = true
					loose = true
				}
				break
			}
			// A sibling marker at this level ends the item.
			if (ordered && reOLItem.MatchString(l)) || (!ordered && reULItem.MatchString(l)) {
				lm := reULItem.FindStringSubmatch(l)
				if lm == nil {
					lm = reOLItem.FindStringSubmatch(l)
				}
				if len(lm[1]) < indent {
					break
				}
			}
			if strings.HasPrefix(l, strings.Repeat(" ", indent)) {
				if freshPara {
					// A blank-separated paragraph: kramdown strips its full leading
					// whitespace, not just the item indent.
					it.lines = append(it.lines, strings.TrimLeft(l, " "))
				} else {
					it.lines = append(it.lines, l[indent:])
				}
				i++
				continue
			}
			freshPara = false
			// Lazy continuation: an unindented, non-block, non-marker line.
			if !p.startsNewBlock(lines, i) && !reULItem.MatchString(l) && !reOLItem.MatchString(l) {
				it.lines = append(it.lines, strings.TrimRight(l, " \t"))
				i++
				continue
			}
			break
		}
		// Trim trailing blank lines in the item.
		for len(it.lines) > 0 && strings.TrimSpace(it.lines[len(it.lines)-1]) == "" {
			it.lines = it.lines[:len(it.lines)-1]
		}
		items = append(items, it)
	}
	savedInItem := p.inItem
	p.inItem = true
	for _, it := range items {
		li := newEl(ElLI)
		p.parseBlocks(it.lines, li)
		// An item with a trailing/internal blank renders its lone paragraph wrapped
		// in <p> (the "loose" form).
		if it.looseItem {
			li.Options["force_loose"] = true
		}
		list.addChild(li)
	}
	p.inItem = savedInItem
	list.Options["tight"] = !loose
	parent.addChild(list)
	return i - start
}

// tryDefinitionList recognises a definition list: a term line (or several)
// followed by a ": definition" line. Returns nil if lines[start] is not one.
func (p *parser) tryDefinitionList(lines []string, start int) (*Element, int) {
	// A definition list is one or more non-blank term lines immediately followed by
	// a ": definition" marker. Scan the leading run of term lines (each not itself a
	// marker, not blank, not the start of another block) for a following marker.
	if strings.TrimSpace(lines[start]) == "" || reDefMarker.MatchString(lines[start]) {
		return nil, 0
	}
	k := start
	for k < len(lines) && strings.TrimSpace(lines[k]) != "" && !reDefMarker.MatchString(lines[k]) {
		// A line that begins another block type cannot be a definition term.
		if p.startsNewBlock(lines, k) || reULItem.MatchString(lines[k]) || reOLItem.MatchString(lines[k]) {
			return nil, 0
		}
		k++
	}
	// A single blank line may separate the term(s) from the ": definition" marker
	// (kramdown's "loose" definition list). Skip such a blank run before the marker.
	for k < len(lines) && strings.TrimSpace(lines[k]) == "" {
		k++
	}
	if k >= len(lines) || !reDefMarker.MatchString(lines[k]) {
		return nil, 0
	}
	dl := newEl(ElDL)
	i := start
	pendingLoose := false // a blank line separated the next definition from its term
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			// A blank line continues the definition list only if the next non-blank
			// block is another definition: either a ": def" marker, or a term line
			// immediately followed by a ": def" marker.
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			cont := false
			if j < len(lines) {
				if reDefMarker.MatchString(lines[j]) {
					cont = true
				} else if j+1 < len(lines) && reDefMarker.MatchString(lines[j+1]) &&
					!reDefMarker.MatchString(lines[j]) && strings.TrimSpace(lines[j]) != "" {
					cont = true
				}
			}
			if cont {
				// A definition that follows a blank line renders as a loose <dd> (its
				// content wrapped in <p>).
				pendingLoose = true
				i = j
				continue
			}
			break
		}
		if dm := reDefMarker.FindStringSubmatch(line); dm != nil {
			indent := len(dm[1]) + 1 + len(dm[3])
			var body []string
			body = append(body, dm[4])
			i++
			for i < len(lines) {
				l := lines[i]
				if strings.TrimSpace(l) == "" {
					j := i
					for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
						j++
					}
					if j < len(lines) && strings.HasPrefix(lines[j], strings.Repeat(" ", indent)) {
						body = append(body, "")
						i++
						continue
					}
					break
				}
				if reDefMarker.MatchString(l) {
					break
				}
				if strings.HasPrefix(l, strings.Repeat(" ", indent)) {
					body = append(body, l[indent:])
					i++
					continue
				}
				if !p.startsNewBlock(lines, i) && !reDefMarker.MatchString(l) {
					body = append(body, strings.TrimRight(l, " \t"))
					i++
					continue
				}
				break
			}
			for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
				body = body[:len(body)-1]
			}
			dd := newEl(ElDD)
			p.parseBlocks(body, dd)
			if pendingLoose {
				dd.Options["force_loose"] = true
				pendingLoose = false
			}
			dl.addChild(dd)
			continue
		}
		// Otherwise it is a term line (possibly multiple consecutive terms).
		dt := newEl(ElDT)
		dt.Options["raw"] = strings.TrimRight(line, " \t")
		dl.addChild(dt)
		i++
	}
	return dl, i - start
}
