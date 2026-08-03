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
		lines         []string
		looseItem     bool // blank separator seen inside the item
		trailingBlank bool // a blank line separates this item from the next sibling
	}
	var items []itemData
	eobFound := false
	for i < len(lines) {
		// A standalone EOB "^" marker terminates the list (kramdown's EOB_MARKER); the
		// marker itself is left for the surrounding block loop to consume (it renders
		// nothing), so a following list starts fresh.
		if strings.TrimRight(lines[i], " \t") == "^" {
			eobFound = true
			break
		}
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
			// A horizontal rule (kramdown breaks on last_is_blank && HR_START, even
			// though "* * *" also matches a UL marker) or a lone EOB "^" ends the list,
			// as does any non-continuation line. In every case the blank line kramdown
			// consumed belongs to the (now last) item's value, so it carries a trailing
			// blank that its looseness decision reads.
			if j >= len(lines) || reHR.MatchString(lines[j]) ||
				strings.TrimRight(lines[j], " \t") == "^" || !nextIsItem {
				if j < len(lines) && strings.TrimRight(lines[j], " \t") == "^" {
					// kramdown consumes the blank into the item then breaks on EOB, and
					// does NOT re-emit it after the list — skip past it to the marker.
					eobFound = true
					i = j
				}
				if len(items) > 0 {
					items[len(items)-1].trailingBlank = true
				}
				break
			}
			if len(items) > 0 {
				items[len(items)-1].looseItem = true
				items[len(items)-1].trailingBlank = true
				loose = true
			}
			i = j
			continue
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
			if strings.TrimRight(l, " \t") == "^" {
				// EOB marker: end the item (and the list); the outer loop stops too.
				break
			}
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
					// Blank then a sibling marker: the list stays but this item is loose
					// and carries a trailing blank child (kramdown keeps the blank line in
					// the item's value, which drives its looseness decision).
					it.looseItem = true
					it.trailingBlank = true
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
		// Trim trailing blank lines in the item; a trailing blank child is (re)added
		// below once the final trailingBlank state is known (a later loop iteration can
		// mark this item once it sees the blank that separates it from what follows).
		for len(it.lines) > 0 && strings.TrimSpace(it.lines[len(it.lines)-1]) == "" {
			it.lines = it.lines[:len(it.lines)-1]
		}
		items = append(items, it)
	}
	savedInItem := p.inItem
	p.inItem = true
	for _, it := range items {
		li := newEl(ElLI)
		lns := it.lines
		// A leading "{:…}" IAL on the item's first line applies to the <li>.
		if len(lns) > 0 {
			if body, rest, ok := stripLeadingItemIAL(lns[0]); ok {
				applyIALToElement(li, body, p.aldDefs)
				lns = append([]string{rest}, lns[1:]...)
			}
		}
		// A blank line separated this item from what follows: kramdown keeps that
		// blank in the item's value as a trailing :blank child, which the looseness
		// rule reads.
		if it.trailingBlank {
			lns = append(lns, "")
		}
		p.parseBlocks(lns, li)
		list.addChild(li)
	}
	p.inItem = savedInItem
	finalizeListItems(list, eobFound)
	list.Options["tight"] = !loose
	parent.addChild(list)
	return i - start
}

// finalizeListItems reproduces kramdown's per-item looseness decision
// (parser/kramdown/list.rb): the first paragraph of an item is rendered
// "transparent" (inline, without a <p> wrapper) unless the item is loose. It also
// pops a trailing :blank child off each item. eobFound reports whether the list was
// terminated by an EOB ("^") marker, which suppresses the last-item exemption.
func finalizeListItems(list *Element, eobFound bool) {
	items := list.Children
	for idx, li := range items {
		ch := li.Children
		if len(ch) == 0 {
			continue
		}
		isLast := idx == len(items)-1
		// Condition A: first child is a paragraph and it is not immediately followed
		// by a blank separator — except the very last item may keep a single trailing
		// blank (unless an EOB marker closed the list).
		aCond := ch[0].Type == ElP &&
			(len(ch) < 2 || ch[1].Type != ElBlank ||
				(isLast && len(ch) == 2 && !eobFound))
		// Condition B: a non-last item always qualifies; a lone item qualifies; the
		// last item qualifies only if some earlier item is not a non-transparent
		// paragraph (i.e. the list is not uniformly loose).
		bCond := !isLast || len(items) == 1 || anyEarlierQualifies(items[:idx])
		if aCond && bCond {
			li.Options["first_transparent"] = true
		}
		// Pop a trailing blank child (it becomes a separator after the list, which the
		// surrounding block stream already emits).
		if n := len(li.Children); n > 0 && li.Children[n-1].Type == ElBlank {
			li.Children = li.Children[:n-1]
		}
	}
}

// anyEarlierQualifies is kramdown's list.children[0..-2].any? predicate: an item
// qualifies if it is empty, its first child is not a paragraph, or its first
// paragraph is transparent.
func anyEarlierQualifies(earlier []*Element) bool {
	for _, li := range earlier {
		if len(li.Children) == 0 || li.Children[0].Type != ElP {
			return true
		}
		if t, _ := li.Options["first_transparent"].(bool); t {
			return true
		}
	}
	return false
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
	// At most ONE blank line may separate the term(s) from the ": definition"
	// marker (kramdown's "loose" definition list). Two or more blanks break the
	// association, so the ": …" line is a plain paragraph, not a definition.
	blanks := 0
	for k < len(lines) && strings.TrimSpace(lines[k]) == "" {
		k++
		blanks++
	}
	if blanks > 1 || k >= len(lines) || !reDefMarker.MatchString(lines[k]) {
		return nil, 0
	}
	dl := newEl(ElDL)
	i := start
	pendingLoose := false // a blank line separated the next definition from its term
	for i < len(lines) {
		line := lines[i]
		if strings.TrimRight(line, " \t") == "^" {
			// An end-of-block marker terminates the definition list; a following
			// term/definition starts a fresh <dl>.
			break
		}
		if _, ok := matchBlockIAL(line); ok {
			// A standalone block IAL ends the list; the surrounding block loop attaches
			// it to the <dl> just parsed.
			break
		}
		if strings.TrimSpace(line) == "" {
			// A blank line continues the definition list only if the next non-blank
			// block is another definition: either a ": def" marker, or a term line
			// immediately followed by a ": def" marker.
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			cont := false
			markerAfterBlank := false
			if j < len(lines) {
				if reDefMarker.MatchString(lines[j]) {
					cont = true
					// The blank separates a definition from its own term, so this
					// definition is loose (its content is wrapped in <p>).
					markerAfterBlank = true
				} else if j+1 < len(lines) && reDefMarker.MatchString(lines[j+1]) &&
					!reDefMarker.MatchString(lines[j]) && strings.TrimSpace(lines[j]) != "" {
					// The blank precedes a fresh term whose marker follows immediately:
					// a new, tight term/definition group.
					cont = true
				}
			}
			if cont {
				if markerAfterBlank {
					pendingLoose = true
				}
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
				if strings.TrimRight(l, " \t") == "^" {
					break // EOB terminates this definition (and the list)
				}
				if _, ok := matchBlockIAL(l); ok && !strings.HasPrefix(l, strings.Repeat(" ", indent)) {
					break // a standalone (unindented) block IAL ends the definition
				}
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
			// A leading "{:…}" IAL on the definition applies to the <dd>.
			if len(body) > 0 {
				if ialBody, rest, ok := stripLeadingItemIAL(body[0]); ok {
					applyIALToElement(dd, ialBody, p.aldDefs)
					body[0] = rest
				}
			}
			p.parseBlocks(body, dd)
			if pendingLoose {
				dd.Options["force_loose"] = true
				pendingLoose = false
			}
			dl.addChild(dd)
			continue
		}
		// Otherwise it is a term line (possibly multiple consecutive terms). A leading
		// "{:…}" IAL on the term applies to the <dt>.
		dt := newEl(ElDT)
		term := strings.TrimRight(line, " \t")
		if body, rest, ok := stripLeadingItemIAL(term); ok {
			applyIALToElement(dt, body, p.aldDefs)
			term = rest
		}
		dt.Options["raw"] = term
		dl.addChild(dt)
		i++
	}
	return dl, i - start
}
