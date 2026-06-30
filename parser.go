// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// parser holds the mutable state of a single block-level parse: the source split
// into lines, the link/abbreviation/ALD definitions harvested in a pre-pass, the
// footnote definitions, and the warnings list.
type parser struct {
	src      string
	opts     Options
	linkDefs map[string]linkDef
	abbrevs  map[string]abbrevDef
	aldDefs  map[string]string // ALD id -> raw attribute string
	footDefs map[string]*Element
	warnings []string
	// inItem is set while parsing the de-indented content of a list/definition
	// item, where (unlike at the top level) a list marker on a continuation line
	// begins a nested list rather than lazily continuing a paragraph.
	inItem bool
}

// linkDef is a reference-style link/image definition: [id]: url "title".
type linkDef struct {
	url   string
	title string
}

// abbrevDef is an abbreviation definition: *[text]: title.
type abbrevDef struct {
	title string
	attr  string // raw IAL string attached to the definition, if any
}

// newParser builds a parser for src under opts.
func newParser(src string, opts Options) *parser {
	return &parser{
		src:      src,
		opts:     opts,
		linkDefs: map[string]linkDef{},
		abbrevs:  map[string]abbrevDef{},
		aldDefs:  map[string]string{},
		footDefs: map[string]*Element{},
	}
}

// warn records a parser warning (kramdown surfaces these on Document#warnings).
func (p *parser) warn(msg string) { p.warnings = append(p.warnings, msg) }

// normalize converts CRLF/CR to LF, expands leading tabs to 4-space stops the way
// kramdown does, and guarantees a trailing newline.
func normalize(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	if !strings.HasSuffix(src, "\n") {
		src += "\n"
	}
	return src
}

// expandTabs replaces tabs with spaces to the next 4-column tab stop, matching
// kramdown's leading-whitespace handling for block detection.
func expandTabs(line string) string {
	if !strings.Contains(line, "\t") {
		return line
	}
	var b strings.Builder
	col := 0
	for _, r := range line {
		if r == '\t' {
			n := 4 - col%4
			for i := 0; i < n; i++ {
				b.WriteByte(' ')
			}
			col += n
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// parse drives the whole parse: a definition pre-pass strips link/abbrev/ALD/
// footnote definitions, then the remaining lines are parsed into block elements.
func (p *parser) parse() *Element {
	src := normalize(p.src)
	lines := strings.Split(src, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i := range lines {
		lines[i] = expandTabs(lines[i])
	}
	lines = p.harvestDefinitions(lines)
	root := newEl(ElRoot)
	p.parseBlocks(lines, root)
	return root
}

// parseBlocks parses a sequence of source lines into block elements appended to
// parent.
func (p *parser) parseBlocks(lines []string, parent *Element) {
	i := 0
	for i < len(lines) {
		// Consume blank lines as a single ElBlank separator.
		if strings.TrimSpace(lines[i]) == "" {
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			parent.addChild(newEl(ElBlank))
			i = j
			continue
		}
		// Standalone block IAL ({:...}) on its own line attaches to the previous or
		// next block.
		if ial, ok := matchBlockIAL(lines[i]); ok {
			i = p.applyStandaloneIAL(lines, i, ial, parent)
			continue
		}
		consumed := p.parseOneBlock(lines, i, parent)
		i += consumed
	}
}

var (
	reATX        = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*$`)
	reATXNoSpace = regexp.MustCompile(`^(#{1,6})$`)
	reSetext     = regexp.MustCompile(`^(=+|-+)\s*$`)
	reSetextPure = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	reHR         = regexp.MustCompile(`^ {0,3}((\* *){3,}|(- *){3,}|(_ *){3,})$`)
	reBlockIAL   = regexp.MustCompile(`^\{:(?:: *)?([^}]*)\}\s*$`)
	reIndentCode = regexp.MustCompile(`^( {4}|\t)`)
	reFence      = regexp.MustCompile("^(~~~+|```+)\\s*([^`]*?)\\s*$")
	reULItem     = regexp.MustCompile(`^( {0,3})([*+-])(\s+)(.*)$`)
	reOLItem     = regexp.MustCompile(`^( {0,3})(\d+)\.(\s+)(.*)$`)
	reBlockquote = regexp.MustCompile(`^ {0,3}> ?(.*)$`)
	reHeaderID   = regexp.MustCompile(`[ \t]+\{#([A-Za-z][\w:-]*)\}[ \t]*$`)
	reDefMarker  = regexp.MustCompile(`^( {0,3})(:)(\s+)(.*)$`)
)

// parseOneBlock dispatches on lines[i] to the correct block parser and returns the
// number of input lines consumed (always >= 1).
func (p *parser) parseOneBlock(lines []string, i int, parent *Element) int {
	line := lines[i]

	if strings.TrimRight(line, " \t") == "^" {
		// An end-of-block marker: it terminates the preceding block and renders
		// nothing itself, leaving the surrounding blocks directly adjacent.
		return 1
	}
	if reHR.MatchString(line) {
		parent.addChild(newEl(ElHR))
		return 1
	}
	if m := reATX.FindStringSubmatch(line); m != nil {
		p.parseATXHeader(m, parent)
		return 1
	}
	if reATXNoSpace.MatchString(strings.TrimRight(line, " ")) {
		// "#" alone is a paragraph in kramdown.
		return p.parseParagraph(lines, i, parent)
	}
	if strings.HasPrefix(line, "{::comment}") || strings.HasPrefix(line, "{::comment ") {
		return p.parseComment(lines, i, parent)
	}
	if isHTMLBlockStart(line) {
		return p.parseHTMLBlock(lines, i, parent)
	}
	if reFence.MatchString(line) {
		return p.parseFencedCode(lines, i, parent)
	}
	if reIndentCode.MatchString(line) {
		return p.parseIndentedCode(lines, i, parent)
	}
	if reBlockquote.MatchString(line) {
		return p.parseBlockquote(lines, i, parent)
	}
	if reULItem.MatchString(line) || reOLItem.MatchString(line) {
		return p.parseList(lines, i, parent)
	}
	if tbl, n := p.tryTable(lines, i); tbl != nil {
		parent.addChild(tbl)
		return n
	}
	if dl, n := p.tryDefinitionList(lines, i); dl != nil {
		parent.addChild(dl)
		return n
	}
	return p.parseParagraph(lines, i, parent)
}

// parseATXHeader handles "# Header {#id}" lines, stripping trailing "#"s and an
// explicit {#id} IAL.
func (p *parser) parseATXHeader(m []string, parent *Element) {
	level := len(m[1])
	text := m[2]
	// Strip an explicit trailing {#id}.
	id := ""
	if idm := reHeaderID.FindStringSubmatch(text); idm != nil {
		id = idm[1]
		text = text[:len(text)-len(idm[0])]
	}
	// Strip a run of trailing closing #'s (optionally space-separated).
	text = stripClosingHashes(text)
	h := newEl(ElHeader)
	h.Options["level"] = level
	h.Options["raw_text"] = text
	if id != "" {
		h.Options["explicit_id"] = id
	}
	parent.addChild(h)
}

// stripClosingHashes removes a trailing " ###" closing-hash run from an ATX header
// (kramdown keeps a "header #" with no leading space before the hashes literal,
// but strips " #" / " ##").
func stripClosingHashes(s string) string {
	t := strings.TrimRight(s, " ")
	// trailing hashes must be preceded by a space to be a closer
	j := len(t)
	for j > 0 && t[j-1] == '#' {
		j--
	}
	if j == len(t) {
		return s // no trailing hashes
	}
	if j == 0 {
		// all hashes: keep as-is (handled elsewhere)
		return strings.TrimRight(s, " ")
	}
	if t[j-1] == ' ' {
		return strings.TrimRight(t[:j], " ")
	}
	// escaped or attached hash like "header #" already has space, "header#" stays
	return strings.TrimRight(s, " ")
}

// parseParagraph collects consecutive non-blank, non-block-starting lines into a
// paragraph, honouring a Setext underline that promotes it to a header.
func (p *parser) parseParagraph(lines []string, start int, parent *Element) int {
	var buf []string
	i := start
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			break
		}
		// A lone "^" end-of-block marker terminates the paragraph (the block loop
		// then consumes the marker itself, emitting nothing).
		if len(buf) > 0 && strings.TrimRight(line, " \t") == "^" {
			break
		}
		// A Setext underline after at least one collected line makes a header. A
		// pure run of "=" or "-" is a Setext underline even though a "-" run also
		// matches the horizontal-rule pattern; in paragraph context the underline
		// wins (a real HR needs internal spaces, e.g. "- - -").
		if len(buf) > 0 && reSetextPure.MatchString(line) {
			p.makeSetextHeader(buf, line, parent)
			return i - start + 1
		}
		// Stop if a new block (other than a lazily-continued paragraph) begins.
		if len(buf) > 0 && p.startsNewBlock(lines, i) {
			break
		}
		if ial, ok := matchBlockIAL(line); ok && len(buf) > 0 {
			// IAL terminates the paragraph; let the block loop attach it.
			_ = ial
			break
		}
		// Strip leading whitespace only from the first line (kramdown left-strips a
		// paragraph's opening line; continuation lines keep their indentation). Keep
		// trailing spaces so the span parser can detect hard line breaks ("  \n").
		if len(buf) == 0 {
			line = strings.TrimLeft(line, " \t")
		}
		buf = append(buf, line)
		i++
	}
	// Trim trailing whitespace only on the last line (a trailing hard break at the
	// very end of a paragraph is dropped by kramdown).
	if len(buf) > 0 {
		buf[len(buf)-1] = strings.TrimRight(buf[len(buf)-1], " \t")
	}
	para := newEl(ElP)
	para.Options["raw"] = strings.Join(buf, "\n")
	parent.addChild(para)
	return i - start
}

// makeSetextHeader builds a header from collected paragraph lines and the
// underline (= → h1, - → h2), honouring a trailing {#id}.
func (p *parser) makeSetextHeader(buf []string, underline string, parent *Element) {
	level := 2
	if strings.HasPrefix(strings.TrimSpace(underline), "=") {
		level = 1
	}
	text := strings.TrimRight(strings.Join(buf, "\n"), " \t")
	id := ""
	if idm := reHeaderID.FindStringSubmatch(text); idm != nil {
		id = idm[1]
		text = strings.TrimRight(text[:len(text)-len(idm[0])], " \t")
	}
	h := newEl(ElHeader)
	h.Options["level"] = level
	h.Options["raw_text"] = text
	if id != "" {
		h.Options["explicit_id"] = id
	}
	parent.addChild(h)
}

// startsNewBlock reports whether lines[i] begins a block that interrupts a running
// paragraph (or lazy continuation) without an intervening blank line. kramdown is
// blank-line delimited: a header, list, fence, quote or table on the next line is
// absorbed into the paragraph, NOT split out — only a block-level HTML element
// interrupts. Inside a list/definition item, a list marker additionally starts a
// nested list.
func (p *parser) startsNewBlock(lines []string, i int) bool {
	line := lines[i]
	switch {
	case isHTMLBlockStart(line):
		return true
	case p.inItem && (reULItem.MatchString(line) || reOLItem.MatchString(line)):
		// Inside a list item, a marker line starts a nested list (top-level
		// paragraphs, by contrast, swallow a marker as lazy continuation).
		return true
	}
	return false
}

// parseComment handles the {::comment}…{:/comment} extension, emitting an
// ElComment whose Value is the enclosed text rendered as an HTML comment.
func (p *parser) parseComment(lines []string, start int, parent *Element) int {
	first := lines[start]
	// Self-closing form: {::comment ... /}
	if strings.HasSuffix(strings.TrimSpace(first), "/}") {
		return 1 // produces nothing
	}
	var buf []string
	rest := strings.TrimPrefix(first, "{::comment}")
	if idx := strings.Index(rest, "{:/comment}"); idx >= 0 {
		buf = append(buf, rest[:idx])
		c := newEl(ElComment)
		c.Value = strings.TrimSpace(strings.Join(buf, "\n"))
		parent.addChild(c)
		return 1
	}
	if strings.TrimSpace(rest) != "" {
		buf = append(buf, rest)
	}
	i := start + 1
	closed := false
	for i < len(lines) {
		line := lines[i]
		if idx := strings.Index(line, "{:/comment}"); idx >= 0 {
			if idx > 0 {
				buf = append(buf, line[:idx])
			}
			closed = true
			i++
			break
		}
		if strings.TrimSpace(line) == "{:/}" {
			closed = true
			i++
			break
		}
		buf = append(buf, line)
		i++
	}
	if !closed {
		// Unterminated: kramdown treats the opener literally as a paragraph.
		return p.parseParagraph(lines, start, parent)
	}
	c := newEl(ElComment)
	c.Value = strings.TrimSpace(strings.Join(buf, "\n"))
	parent.addChild(c)
	return i - start
}

// parseFencedCode handles ``` / ~~~ fenced code blocks with an optional language.
func (p *parser) parseFencedCode(lines []string, start int, parent *Element) int {
	m := reFence.FindStringSubmatch(lines[start])
	fence := m[1]
	lang := strings.TrimSpace(m[2])
	closer := fence[:1]
	var buf []string
	i := start + 1
	for i < len(lines) {
		if strings.HasPrefix(lines[i], strings.Repeat(closer, len(fence))) &&
			strings.TrimSpace(strings.TrimLeft(lines[i], closer)) == "" {
			i++
			break
		}
		buf = append(buf, lines[i])
		i++
	}
	cb := newEl(ElCodeblock)
	cb.Value = strings.Join(buf, "\n")
	if len(buf) > 0 {
		cb.Value += "\n"
	}
	if lang != "" {
		cb.Options["lang"] = lang
	}
	parent.addChild(cb)
	return i - start
}

// parseIndentedCode handles a run of 4-space-indented lines as a literal code
// block, including blank lines that sit between indented lines.
func (p *parser) parseIndentedCode(lines []string, start int, parent *Element) int {
	var buf []string
	i := start
	for i < len(lines) {
		line := lines[i]
		if reIndentCode.MatchString(line) {
			buf = append(buf, stripIndent(line, 4))
			i++
			continue
		}
		if strings.TrimSpace(line) == "" {
			// Lookahead: include the blank only if more indented code follows.
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j < len(lines) && reIndentCode.MatchString(lines[j]) {
				for ; i < j; i++ {
					buf = append(buf, "")
				}
				continue
			}
			break
		}
		break
	}
	// Trim trailing blank lines.
	for len(buf) > 0 && buf[len(buf)-1] == "" {
		buf = buf[:len(buf)-1]
	}
	cb := newEl(ElCodeblock)
	cb.Value = strings.Join(buf, "\n") + "\n"
	parent.addChild(cb)
	return i - start
}

// stripIndent removes up to n leading spaces (a leading tab counts as a full
// removal of the indent) from line.
func stripIndent(line string, n int) string {
	count := 0
	for count < n && count < len(line) && line[count] == ' ' {
		count++
	}
	return line[count:]
}

// parseBlockquote gathers ">"-prefixed (and lazily-continued) lines and parses the
// dequoted content recursively.
func (p *parser) parseBlockquote(lines []string, start int, parent *Element) int {
	var inner []string
	i := start
	for i < len(lines) {
		line := lines[i]
		if m := reBlockquote.FindStringSubmatch(line); m != nil {
			inner = append(inner, m[1])
			i++
			continue
		}
		if strings.TrimSpace(line) == "" {
			break
		}
		// Lazy continuation: a non-blank, non-block line continues the quote.
		if !p.startsNewBlock(lines, i) {
			inner = append(inner, strings.TrimRight(line, " \t"))
			i++
			continue
		}
		break
	}
	bq := newEl(ElBlockquote)
	p.parseBlocks(inner, bq)
	parent.addChild(bq)
	return i - start
}

// matchBlockIAL returns the raw attribute string of a standalone block IAL line
// ("{:...}") and whether it matched. A "{::...}" extension (e.g. {::comment}) is
// not an IAL and is excluded so the dedicated extension parser handles it.
func matchBlockIAL(line string) (string, bool) {
	t := strings.TrimRight(line, " \t")
	if strings.HasPrefix(t, "{::") {
		return "", false
	}
	if m := reBlockIAL.FindStringSubmatch(t); m != nil {
		// Exclude ALD references which start with a letter+colon ("{:id: ...}") — those
		// are recognised separately by splitALD in applyStandaloneIAL.
		body := m[1]
		return body, true
	}
	return "", false
}

// applyStandaloneIAL attaches a standalone {:...} line to the previous block, or
// (if it is an ALD definition "{:id: ...}") records the ALD; a leading-position
// IAL attaches to the following block. It returns the next index to parse from.
func (p *parser) applyStandaloneIAL(lines []string, i int, ial string, parent *Element) int {
	// ALD definition: "{:name: attrs}".
	if name, attrs, ok := splitALD(ial); ok {
		p.aldDefs[name] = attrs
		return i + 1
	}
	// Attach to previous non-blank block if one exists and is not separated by a
	// blank line; otherwise buffer for the next block.
	if n := len(parent.Children); n > 0 && parent.Children[n-1].Type != ElBlank {
		applyIALToElement(parent.Children[n-1], ial, p.aldDefs)
		return i + 1
	}
	// Leading IAL: peek next block and attach after parsing it.
	next := i + 1
	// skip following blank lines
	for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
		next++
	}
	if next >= len(lines) {
		return len(lines)
	}
	before := len(parent.Children)
	consumed := p.parseOneBlock(lines, next, parent)
	if len(parent.Children) > before {
		applyIALToElement(parent.Children[len(parent.Children)-1], ial, p.aldDefs)
	}
	return next + consumed
}
