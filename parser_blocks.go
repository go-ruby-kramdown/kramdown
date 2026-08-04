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
	pendingAbbrevIAL := ""
	pendingLinkIAL := ""
	i := 0
	// atBoundary tracks whether the current line sits at a block boundary (the start
	// of the stream, right after a blank line, or right after a harvested definition
	// — each of which kramdown treats as after_block_boundary?). A link-reference
	// definition is only recognised at a boundary, so "[id]: url" sitting directly
	// under a paragraph line stays part of that paragraph.
	atBoundary := true
	for i < len(lines) {
		line := lines[i]
		// A run of standalone block IALs immediately preceding a link-reference
		// definition attaches to that definition (kramdown applies the pending
		// block IAL to the :eob element the definition creates). Intercept the run
		// here so the IAL lines are consumed rather than emitted as paragraphs.
		if body, ok := matchBlockIAL(line); ok {
			if _, _, isALD := splitALD(body); !isALD {
				if j, attrs, hit := collectLinkDefIAL(lines, i); hit {
					pendingLinkIAL = joinIAL(pendingLinkIAL, attrs)
					i = j
					atBoundary = true
					continue
				}
			}
		}
		// Footnote definition: "[^id]: ...". Checked before the link-reference
		// definition because "[^x]" also satisfies the looser "[id]" link pattern.
		if m := reFootnoteDef.FindStringSubmatch(line); m != nil {
			id := m[1]
			var body []string
			first := strings.TrimLeft(m[2], " \t")
			// A definition whose marker line carries no inline content (the body opens
			// on the following line) keeps a leading blank in kramdown, which the
			// converter renders as a blank line inside the footnote's <li>.
			leadingBlank := first == ""
			if first != "" {
				body = append(body, first)
			}
			i++
			// Continuation lines: a 4-space-indented line belongs to the note, and a
			// blank line only when it is itself followed (before any unindented content)
			// by more indented text — mirroring kramdown's CODEBLOCK_MATCH, which pairs
			// each blank with a following indented run. Trailing blank lines that lead to
			// no further indented content are NOT part of the note: they stay in the
			// stream as the block separation after the definition.
			pendingBlanks := 0
			for i < len(lines) {
				l := lines[i]
				if strings.TrimSpace(l) == "" {
					pendingBlanks++
					i++
					continue
				}
				if strings.HasPrefix(l, "    ") || strings.HasPrefix(l, "\t") {
					for ; pendingBlanks > 0; pendingBlanks-- {
						body = append(body, "")
					}
					body = append(body, stripIndent(expandTabs(l), 4))
					i++
					continue
				}
				break
			}
			// Give back the trailing blank lines not absorbed into the note.
			i -= pendingBlanks
			fn := newEl(ElFootnoteDef)
			fn.Options["leading_blank"] = leadingBlank
			p.parseBlocks(body, fn)
			// An IAL right after attaches to the definition (ignored for HTML here).
			if i < len(lines) {
				if reBlockIAL.MatchString(strings.TrimRight(lines[i], " \t")) {
					i++
				}
			}
			p.footDefs[id] = fn
			// kramdown records a footnote definition as a block-level end-of-block
			// element, so a definition sitting between two blocks keeps them apart
			// (e.g. two single-item lists must not merge into one loose list). Emit the
			// same boundary marker used for link definitions: it renders nothing and,
			// unlike a source "^" marker, does not loosen a list it closes.
			out = append(out, defBoundaryMarker)
			atBoundary = true
			continue
		}
		// Link/image reference definition: "[id]: url "title"" (the URL may contain
		// spaces or be angle-bracketed, and the title may continue on the next line).
		// kramdown only recognises it at a block boundary, so it is skipped when the
		// previous line was ordinary paragraph text.
		if atBoundary {
			if id, url, title, consumed, ok := p.matchLinkDef(lines, i); ok {
				id = normalizeRef(id)
				ial := pendingLinkIAL
				pendingLinkIAL = ""
				i += consumed
				// A run of standalone block IALs immediately after the definition also
				// attaches to it; consume those lines too.
				for i < len(lines) {
					body, ok := matchBlockIAL(lines[i])
					if !ok {
						break
					}
					if _, _, isALD := splitALD(body); isALD {
						break
					}
					ial = joinIAL(ial, body)
					i++
				}
				p.linkDefs[id] = linkDef{url: url, title: title, ial: ial}
				// kramdown leaves an ":eob :link_def" element here; a following block that
				// is not separated by a blank line is therefore NOT at a block boundary (a
				// table directly beneath a link-reference definition stays a paragraph).
				if i < len(lines) && strings.TrimSpace(lines[i]) != "" {
					out = append(out, defBoundaryMarker)
				}
				atBoundary = true
				continue
			}
		}
		// A standalone block IAL immediately before an abbreviation definition
		// augments that definition (kramdown attaches it to the abbreviation).
		if body, ok := matchBlockIAL(line); ok && i+1 < len(lines) &&
			reAbbrevDef.MatchString(lines[i+1]) {
			if _, _, isALD := splitALD(body); !isALD {
				pendingAbbrevIAL = body
				i++
				atBoundary = true
				continue
			}
		}
		// Abbreviation definition: "*[text]: title".
		if m := reAbbrevDef.FindStringSubmatch(line); m != nil {
			text := m[1]
			title := strings.TrimSpace(m[2])
			def := abbrevDef{title: title}
			var attrs []string
			if pendingAbbrevIAL != "" {
				attrs = append(attrs, pendingAbbrevIAL)
				pendingAbbrevIAL = ""
			}
			// An IAL directly under the definition also augments it.
			if i+1 < len(lines) {
				if am := reBlockIAL.FindStringSubmatch(strings.TrimRight(lines[i+1], " \t")); am != nil {
					attrs = append(attrs, am[1])
					i++
				}
			}
			def.attr = strings.Join(attrs, " ")
			p.abbrevs[text] = def
			i++
			atBoundary = true
			continue
		}
		out = append(out, line)
		atBoundary = strings.TrimSpace(line) == ""
		i++
	}
	return out
}

var (
	reLinkDef     = regexp.MustCompile(`^ {0,3}\[([^\]]+)\]:\s+(\S+)(?:\s+["'(](.*)["')])?\s*$`)
	reAbbrevDef   = regexp.MustCompile(`^ {0,3}\*\[([^\]]+)\]:(.*)$`)
	reFootnoteDef = regexp.MustCompile(`^ {0,3}\[\^([^\]]+)\]:(.*)$`)

	// reLinkDefHead splits a candidate link-definition line into its id and the
	// remainder after "]:" (0-3 leading spaces; the id is any run without "]" or a
	// newline). kramdown's LINK_DEFINITION_START limits the indent to three spaces,
	// so a four-space-indented "[id]: url" is an indented code block instead.
	reLinkDefHead = regexp.MustCompile(`^ {0,3}\[([^\]\n]+)\]:[ \t]*(.*)$`)
	// reLinkDefTitleDQ/SQ peel a trailing quoted title off a bare-URL remainder; the
	// URL capture is greedy so the LAST quoted run becomes the title (matching
	// kramdown's non-greedy URL that backtracks to the final title).
	reLinkDefTitleDQ = regexp.MustCompile(`^(.*\S)[ \t]+"(.*)"[ \t]*$`)
	reLinkDefTitleSQ = regexp.MustCompile(`^(.*\S)[ \t]+'(.*)'[ \t]*$`)
	// reLinkDefTitleLineDQ/SQ match a title standing alone (inline after an angle URL
	// or on the definition's next line).
	reLinkDefTitleLineDQ = regexp.MustCompile(`^[ \t]*"(.*)"[ \t]*$`)
	reLinkDefTitleLineSQ = regexp.MustCompile(`^[ \t]*'(.*)'[ \t]*$`)
	// reURLSpaceQuote is kramdown's "space before a quote" guard: a bare URL that
	// contains whitespace immediately before a quote is not a valid definition.
	reURLSpaceQuote = regexp.MustCompile("[ \t][\"']")
)

// matchLinkDef recognises a link-reference definition beginning at lines[i],
// mirroring kramdown's LINK_DEFINITION_START and its parse_link_definition guard.
// It returns the id, URL and title, the number of source lines consumed (1, or 2
// when the title is on the following line), and whether a definition was found. The
// caller invokes it only at a block boundary.
func (p *parser) matchLinkDef(lines []string, i int) (id, url, title string, consumed int, ok bool) {
	m := reLinkDefHead.FindStringSubmatch(lines[i])
	if m == nil {
		return
	}
	id, rest := m[1], m[2]
	// Angle-bracketed URL: "<…>" may contain spaces; an inline title may follow.
	// kramdown tries this alternative first but backtracks to the bare-URL form when
	// the "<…>" is unterminated or trailed by non-title text, so those fall through.
	if strings.HasPrefix(rest, "<") {
		if k := strings.Index(rest, ">"); k >= 0 {
			url = rest[1:k]
			after := strings.TrimLeft(rest[k+1:], " \t")
			if after == "" {
				if t, cok := linkDefTitleNextLine(lines, i); cok {
					return id, url, t, 2, true
				}
				return id, url, "", 1, true
			}
			if t, tok := linkDefInlineTitle(after); tok {
				return id, url, t, 1, true
			}
		}
	}
	// A bare URL must contain at least one non-space character.
	if strings.TrimSpace(rest) == "" {
		return
	}
	// A bare URL may contain spaces; peel off a trailing quoted title if present.
	if u, t, tok := linkDefBareTitle(rest); tok {
		if reURLSpaceQuote.MatchString(u) {
			return "", "", "", 0, false
		}
		return id, strings.TrimRight(u, " \t"), t, 1, true
	}
	if reURLSpaceQuote.MatchString(rest) {
		return "", "", "", 0, false // an unbalanced/misplaced title quote invalidates it
	}
	url = strings.TrimRight(rest, " \t")
	if t, cok := linkDefTitleNextLine(lines, i); cok {
		return id, url, t, 2, true
	}
	return id, url, "", 1, true
}

// linkDefInlineTitle returns the title text of a lone quoted string (an inline
// title following an angle URL), and whether one was present.
func linkDefInlineTitle(s string) (string, bool) {
	if m := reLinkDefTitleLineDQ.FindStringSubmatch(s); m != nil {
		return m[1], true
	}
	if m := reLinkDefTitleLineSQ.FindStringSubmatch(s); m != nil {
		return m[1], true
	}
	return "", false
}

// linkDefBareTitle splits a bare-URL remainder into its URL and a trailing quoted
// title, reporting whether such a title was found.
func linkDefBareTitle(s string) (url, title string, ok bool) {
	if m := reLinkDefTitleDQ.FindStringSubmatch(s); m != nil {
		return m[1], m[2], true
	}
	if m := reLinkDefTitleSQ.FindStringSubmatch(s); m != nil {
		return m[1], m[2], true
	}
	return "", "", false
}

// linkDefTitleNextLine returns a title standing alone on the line after a
// definition (kramdown allows the title on the following line), and whether the
// immediately-following non-blank line is such a title.
func linkDefTitleNextLine(lines []string, i int) (string, bool) {
	if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) == "" {
		return "", false
	}
	return linkDefInlineTitle(lines[i+1])
}

// collectLinkDefIAL scans a run of consecutive standalone (non-ALD) block IALs
// starting at index i and reports whether the line immediately after the run is a
// link-reference definition. When it is, it returns the index of that definition
// line, the run's attributes joined into one IAL body, and true; otherwise it
// returns false so the caller leaves the IAL lines to normal block handling.
func collectLinkDefIAL(lines []string, i int) (int, string, bool) {
	attrs := ""
	j := i
	for j < len(lines) {
		body, ok := matchBlockIAL(lines[j])
		if !ok {
			break
		}
		if _, _, isALD := splitALD(body); isALD {
			break
		}
		attrs = joinIAL(attrs, body)
		j++
	}
	if j == i || j >= len(lines) || !reLinkDef.MatchString(lines[j]) {
		return 0, "", false
	}
	return j, attrs, true
}

// joinIAL concatenates two raw IAL attribute bodies, separating them with a space
// when both are non-empty so their attributes accumulate onto one element.
func joinIAL(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + " " + b
	}
}

// normalizeRef lowercases and collapses internal whitespace of a reference id the
// way kramdown matches link references case-insensitively.
func normalizeRef(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
}

// parseList parses an ordered or unordered list starting at lines[start],
// faithfully following kramdown's parser/kramdown/list.rb. Each item's content is
// accumulated into one or more "value segments" (kramdown's item.value array): a
// new segment begins at the line where a nested list first appears, and every
// segment is parsed as an independent block stream. Content lines carry an
// indentation that is stripped before re-parsing; a lazily-continued nested-list
// marker is re-indented so the recursive parse recognises it.
func (p *parser) parseList(lines []string, start int, parent *Element) int {
	ordered := reOLItem.MatchString(lines[start])
	listType := ElUL
	if ordered {
		listType = ElOL
	}
	list := newEl(listType)

	type itemData struct {
		segments []string // concatenated value segments
		ial      string   // a leading "{:…}" IAL applied to the <li>
		hasIAL   bool
	}
	var items []itemData
	cur := -1

	indentation := 0
	nestedListFound := false
	lastIsBlank := false
	eobFound := false

	i := start
	for i < len(lines) {
		line := lines[i]
		// A horizontal rule after a blank line terminates the list.
		if lastIsBlank && reHR.MatchString(line) {
			break
		}
		// An end-of-block marker terminates the list and is itself consumed.
		if strings.TrimRight(line, " \t") == "^" {
			eobFound = true
			i++
			break
		}
		// A marker whose indent is below the item-content indent starts a sibling
		// item (the first item accepts any 0-3 space indent). reULItem/reOLItem cap
		// their leading-space capture at three, matching kramdown's fetch_pattern.
		if g := listMarker(line, ordered); g != nil && (len(items) == 0 || len(g[1]) <= siblingMax(indentation)) {
			content := g[3] + g[4]
			firstContent, ind := parseFirstListLine(markerLength(g, ordered), content)
			var it itemData
			if body, rest, ok := stripLeadingItemIAL(firstContent); ok {
				it.ial, it.hasIAL, firstContent = body, true, rest
			}
			if firstContent == "" {
				it.segments = []string{""}
			} else {
				it.segments = []string{firstContent + "\n"}
			}
			items = append(items, it)
			cur = len(items) - 1
			indentation = ind
			nestedListFound = listStartMatch(firstContent)
			lastIsBlank = false
			i++
			continue
		}
		// A sufficiently-indented content line, or (when the previous line was not
		// blank) a lazily-continued line, extends the current item.
		if matchesContentRe(line, indentation) || (!lastIsBlank && matchesLazyRe(line, indentation)) {
			result, indentFound := stripListIndent(line, indentation)
			switch {
			case !nestedListFound && indentFound && listStartMatch(result):
				// The first nested list begins: start a new value segment.
				items[cur].segments = append(items[cur].segments, "")
				nestedListFound = true
			case nestedListFound && !indentFound && listStartMatch(result):
				// A lazily-continued nested-list marker: re-indent it so the recursive
				// parse of this segment recognises the marker at the nested level.
				result = strings.Repeat(" ", indentation+4) + result
			}
			n := len(items[cur].segments) - 1
			items[cur].segments[n] += result + "\n"
			lastIsBlank = false
			i++
			continue
		}
		// A blank line: kept in the item's value (its looseness decision reads it) and
		// treated as the start of a nested list for subsequent lines.
		if strings.TrimSpace(line) == "" {
			nestedListFound = true
			lastIsBlank = true
			n := len(items[cur].segments) - 1
			items[cur].segments[n] += "\n"
			i++
			continue
		}
		break
	}

	for idx := range items {
		it := items[idx]
		li := newEl(ElLI)
		if it.hasIAL {
			applyIALToElement(li, it.ial, p.aldDefs)
		}
		for _, seg := range it.segments {
			p.parseBlocks(splitSegment(seg), li)
		}
		list.addChild(li)
	}
	parent.addChild(list)
	// Under the GFM dialect, rewrite "[ ] "/"[x] " item markers into task-list
	// checkboxes (kramdown-parser-gfm's parse_list override).
	if p.opts.Input == "GFM" {
		applyGFMTaskList(list)
	}
	// kramdown re-emits the last item's trailing blank as a separator after the
	// list, unless an EOB marker closed it.
	if finalizeListItems(list, eobFound) && !eobFound {
		parent.addChild(newEl(ElBlank))
	}
	return i - start
}

// splitSegment splits a value-segment string into re-parseable lines. Each stored
// line ended in a newline, so a single trailing empty element is dropped.
func splitSegment(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}

// listMarker returns the list-marker submatch for line at the current list type,
// or nil if the line does not begin with a marker of that type.
func listMarker(line string, ordered bool) []string {
	if ordered {
		return reOLItem.FindStringSubmatch(line)
	}
	return reULItem.FindStringSubmatch(line)
}

// markerLength returns the length of the marker plus its leading spaces (kramdown's
// @src[1].length): the leading spaces and bullet for a UL, or the digits and dot
// for an OL.
func markerLength(g []string, ordered bool) int {
	if ordered {
		return len(g[1]) + len(g[2]) + 1
	}
	return len(g[1]) + len(g[2])
}

// siblingMax is the greatest leading-space count a marker may have and still start
// a sibling item at the given content indentation (kramdown's fetch_pattern range).
func siblingMax(indentation int) int {
	m := indentation - 1
	if m > 3 {
		m = 3
	}
	if m < 0 {
		m = 0
	}
	return m
}

// listStartMatch reports whether s begins with a list marker (kramdown's
// LIST_START), used to detect a nested list.
func listStartMatch(s string) bool {
	return reULItem.MatchString(s) || reOLItem.MatchString(s)
}

// parseFirstListLine reproduces kramdown's parse_first_list_line: it computes the
// content indentation (marker width plus the leading whitespace after it, with tabs
// expanded to the next 4-column stop) and returns the left-stripped first-line
// content. An item whose first line is only an IAL (or empty) uses a fixed indent
// of four.
func parseFirstListLine(markerLen int, content string) (string, int) {
	indentation := markerLen
	if isItemIALCheck(content) {
		indentation = 4
	} else {
		content = expandLeadingTabs(content, markerLen)
		sp := 0
		for sp < len(content) && content[sp] == ' ' {
			sp++
		}
		indentation += sp
	}
	return strings.TrimLeft(content, " \t"), indentation
}

// isItemIALCheck reports whether content is only a leading item IAL followed by
// whitespace (kramdown's LIST_ITEM_IAL_CHECK).
func isItemIALCheck(content string) bool {
	rest := content
	if _, r, ok := stripLeadingItemIAL(content); ok {
		rest = r
	}
	return strings.TrimSpace(rest) == ""
}

// expandLeadingTabs replaces a leading run of "spaces then tabs" with spaces,
// aligning each tab to the next 4-column stop relative to base (kramdown's tab
// expansion in parse_first_list_line).
func expandLeadingTabs(content string, base int) string {
	for {
		sp := 0
		for sp < len(content) && content[sp] == ' ' {
			sp++
		}
		if sp >= len(content) || content[sp] != '\t' {
			break
		}
		tabs := 0
		for sp+tabs < len(content) && content[sp+tabs] == '\t' {
			tabs++
		}
		temp := sp + base
		add := 4 - (temp % 4) + (tabs-1)*4
		content = content[:sp] + strings.Repeat(" ", add) + content[sp+tabs:]
	}
	return content
}

// matchesContentRe reports whether line is indented enough to belong to an item at
// the given content indentation (kramdown's content_re), counting a leading tab or
// four spaces as one indent unit.
func matchesContentRe(line string, indentation int) bool {
	q, r := indentation/4, indentation%4
	if p := consumeIndentUnits(line, q); p >= 0 {
		if p+r <= len(line) && allSpaces(line[p:p+r]) && hasNonSpace(line[p+r:]) {
			return true
		}
	}
	if p := consumeIndentUnits(line, q+1); p >= 0 && hasNonSpace(line[p:]) {
		return true
	}
	return false
}

// consumeIndentUnits consumes k indent units (each a tab or four spaces) from the
// start of line, returning the byte offset past them or -1 if fewer are present.
func consumeIndentUnits(line string, k int) int {
	pos := 0
	for u := 0; u < k; u++ {
		switch {
		case pos < len(line) && line[pos] == '\t':
			pos++
		case pos+4 <= len(line) && line[pos:pos+4] == "    ":
			pos += 4
		default:
			return -1
		}
	}
	return pos
}

// allSpaces reports whether s consists entirely of ASCII spaces.
func allSpaces(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			return false
		}
	}
	return true
}

// hasNonSpace reports whether s contains a non-whitespace character.
func hasNonSpace(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return true
		}
	}
	return false
}

// matchesLazyRe reports whether line may lazily continue an item (kramdown's
// lazy_re): a non-blank line that does not begin (within the item indent, capped at
// three spaces) with a block IAL or an HTML lazy-boundary tag.
func matchesLazyRe(line string, indentation int) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}
	max := indentation
	if max > 3 {
		max = 3
	}
	ls := 0
	for ls < len(line) && line[ls] == ' ' {
		ls++
	}
	if ls <= max {
		rest := line[ls:]
		if _, ok := matchBlockIAL(rest); ok {
			return false
		}
		if isHTMLBlockStart(rest) {
			return false
		}
	}
	return true
}

// stripListIndent removes the item's content indentation from line (kramdown's
// indent_re), first expanding a leading run of tabs to four spaces each. It reports
// whether the full indentation was present.
func stripListIndent(line string, indentation int) (string, bool) {
	tabs := 0
	for tabs < len(line) && line[tabs] == '\t' {
		tabs++
	}
	if tabs > 0 {
		line = strings.Repeat(" ", tabs*4) + line[tabs:]
	}
	sp := 0
	for sp < len(line) && line[sp] == ' ' {
		sp++
	}
	if sp >= indentation {
		return line[indentation:], true
	}
	return line, false
}

// finalizeListItems reproduces kramdown's per-item looseness decision
// (parser/kramdown/list.rb): the first paragraph of an item is rendered
// "transparent" (inline, without a <p> wrapper) unless the item is loose. It also
// pops a trailing :blank child off each item. eobFound reports whether the list was
// terminated by an EOB ("^") marker, which suppresses the last-item exemption. It
// returns whether the last item carried a trailing blank (kramdown's re-emitted
// separator after the list).
func finalizeListItems(list *Element, eobFound bool) bool {
	items := list.Children
	lastHadBlank := false
	for idx, li := range items {
		isLast := idx == len(items)-1
		ch := li.Children
		if len(ch) > 0 {
			// Condition A: first child is a paragraph and it is not immediately
			// followed by a blank separator — except the very last item may keep a
			// single trailing blank (unless an EOB marker closed the list).
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
		}
		// Pop a trailing blank child; on the last item it becomes the separator the
		// caller re-emits after the list.
		popped := false
		if n := len(li.Children); n > 0 && li.Children[n-1].Type == ElBlank {
			li.Children = li.Children[:n-1]
			popped = true
		}
		if isLast {
			lastHadBlank = popped
		}
	}
	return lastHadBlank
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

// defMarkerIALOnly reports whether the text following a definition marker's
// leading whitespace is empty or a single "{:…}" IAL with nothing else, in which
// case kramdown fixes the definition's continuation indentation at four columns.
func defMarkerIALOnly(after string) bool {
	if strings.TrimSpace(after) == "" {
		return true
	}
	if _, rest, ok := stripLeadingItemIAL(after); ok {
		return strings.TrimSpace(rest) == ""
	}
	return false
}

// defGroupFollows reports whether lines[start] begins a fresh term/definition
// group: one or more consecutive term lines (each a plain line, not itself a
// marker, blank or the start of another block) immediately followed by a
// ": definition" marker.
func (p *parser) defGroupFollows(lines []string, start int) bool {
	k := start
	for k < len(lines) && strings.TrimSpace(lines[k]) != "" && !reDefMarker.MatchString(lines[k]) {
		if p.startsNewBlock(lines, k) || reULItem.MatchString(lines[k]) || reOLItem.MatchString(lines[k]) {
			return false
		}
		k++
	}
	return k > start && k < len(lines) && reDefMarker.MatchString(lines[k])
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
				} else if p.defGroupFollows(lines, j) {
					// The blank precedes one or more fresh term lines whose ": def" marker
					// follows immediately: a new, tight term/definition group.
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
			if defMarkerIALOnly(dm[4]) {
				// A definition marker whose content is empty or a lone "{:…}" IAL fixes
				// the continuation indentation at four columns, matching kramdown's
				// parse_first_list_line (LIST_ITEM_IAL_CHECK sets indentation = 4).
				indent = 4
			}
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
				if strings.HasPrefix(l, strings.Repeat(" ", indent)) {
					// An indented line belongs to this definition, even one that itself
					// looks like a ": …" marker (a nested definition list): the indent is
					// stripped and the remainder parsed recursively.
					body = append(body, l[indent:])
					i++
					continue
				}
				if reDefMarker.MatchString(l) {
					break // an un-indented marker starts a sibling definition
				}
				if !p.startsNewBlock(lines, i) {
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
			// When the marker's own line carries no text (it was empty or only an IAL),
			// kramdown consumes that line's newline together with the IAL, so the
			// definition body starts at the following (indented) line rather than a
			// spurious leading blank.
			if len(body) > 0 && strings.TrimSpace(body[0]) == "" {
				body = body[1:]
			}
			p.parseBlocks(body, dd)
			if pendingLoose {
				dd.Options["force_loose"] = true
				pendingLoose = false
			}
			dl.addChild(dd)
			continue
		}
		// A line that begins another block (e.g. a block-level HTML element) is not a
		// term: it terminates the definition list, which the block loop then resumes
		// on. kramdown's term scan only accepts non-block lines, so a stray block line
		// after a definition ends the <dl> rather than becoming a dangling term.
		if p.startsNewBlock(lines, i) {
			break
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
