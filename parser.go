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
	// noLinks is set while span-parsing the text of a link, where kramdown forbids
	// a nested <a> (but still allows a nested image), mirroring its parse_link guard.
	noLinks bool
}

// linkDef is a reference-style link/image definition: [id]: url "title".
type linkDef struct {
	url   string
	title string
	ial   string // raw IAL attribute string attached to the definition, if any
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

// expandTabs replaces tabs in a line's leading whitespace with spaces to the next
// 4-column tab stop, so block-level indentation detection works on space counts.
// Only the leading run is expanded: kramdown's adapt_source never expands tabs, so
// a literal tab inside content (e.g. within a link's text) is preserved verbatim.
func expandTabs(line string) string {
	if !strings.Contains(line, "\t") {
		return line
	}
	lead := 0
	for lead < len(line) && (line[lead] == ' ' || line[lead] == '\t') {
		lead++
	}
	if !strings.Contains(line[:lead], "\t") {
		return line // no tab in the leading whitespace: nothing to expand
	}
	var b strings.Builder
	col := 0
	for i := 0; i < lead; i++ {
		if line[i] == '\t' {
			n := 4 - col%4
			for k := 0; k < n; k++ {
				b.WriteByte(' ')
			}
			col += n
		} else {
			b.WriteByte(' ')
			col++
		}
	}
	return b.String() + line[lead:]
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
	// Predefined link definitions (the :link_defs option) seed the table before the
	// source's own definitions, which may override them.
	for id, d := range p.opts.LinkDefs {
		p.linkDefs[normalizeRef(id)] = linkDef{url: d.URL, title: d.Title}
	}
	lines = p.harvestDefinitions(lines)
	root := newEl(ElRoot)
	p.parseBlocks(lines, root)
	// Deferred block-IAL resolution: now that every ALD definition (including any
	// referenced only from earlier in the document) has been collected, apply the
	// block IALs stashed on their target elements.
	p.resolveBlockIALs(root)
	return root
}

// defBoundaryMarker is an internal sentinel line emitted by the definition
// pre-pass in place of a harvested link/abbreviation/footnote definition that is
// not followed by a blank line. It renders nothing but records that the following
// block is NOT after a block boundary — kramdown leaves an ":eob" element with a
// non-nil value (:link_def/…) there, so a table candidate directly beneath such a
// definition is not recognised as a table. The NUL byte cannot occur in source.
const defBoundaryMarker = "\x00kramdown-def-boundary\x00"

// parseBlocks parses a sequence of source lines into block elements appended to
// parent.
func (p *parser) parseBlocks(lines []string, parent *Element) {
	i := 0
	// atBoundary tracks kramdown's after_block_boundary?: true at the start of the
	// stream, right after a blank run, or after an EOB "^" marker; false after any
	// real block or a non-boundary definition marker. Only table recognition
	// consults it (a table must sit at a block boundary).
	atBoundary := true
	for i < len(lines) {
		// Consume blank lines as a single ElBlank separator.
		if strings.TrimSpace(lines[i]) == "" {
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			parent.addChild(newEl(ElBlank))
			i = j
			atBoundary = true
			continue
		}
		// A definition-boundary marker: emit nothing, but the following block is not
		// at a block boundary.
		if lines[i] == defBoundaryMarker {
			atBoundary = false
			i++
			continue
		}
		// Standalone block IAL ({:...}) on its own line attaches to the previous or
		// next block.
		if ial, ok := matchBlockIAL(lines[i]); ok {
			i = p.applyStandaloneIAL(lines, i, ial, parent)
			atBoundary = false
			continue
		}
		// An EOB "^" marker leaves the following block at a boundary (eob with a nil
		// value); every other block clears it.
		isEOB := strings.TrimRight(lines[i], " \t") == "^"
		consumed := p.parseOneBlock(lines, i, parent, atBoundary)
		i += consumed
		atBoundary = isEOB
	}
}

var (
	reATX        = regexp.MustCompile(`^(#{1,6})[\t ]*([^ \t].*)$`)
	reATXNoSpace = regexp.MustCompile(`^(#{1,6})$`)
	reSetext     = regexp.MustCompile(`^(=+|-+)\s*$`)
	reSetextPure = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	// reHR allows tabs as well as spaces between the rule characters (kramdown's
	// HR_START uses [ \t]*), so a tab-separated "-\t-\t-" is a horizontal rule even
	// though content tabs are no longer expanded to spaces.
	reHR         = regexp.MustCompile(`^ {0,3}((\*[ \t]*){3,}|(-[ \t]*){3,}|(_[ \t]*){3,})$`)
	reBlockIAL   = regexp.MustCompile(`^ {0,3}\{:(?:: *)?((?:\\\}|[^}])*)\}\s*$`)
	reIndentCode = regexp.MustCompile(`^( {4}|\t)`)
	// reLazyCodeJoin folds a lazily-continued (0-3 space) code line onto the
	// previous line: the newline becomes a single space.
	reLazyCodeJoin = regexp.MustCompile(`\n( {0,3}\S)`)
	// reCodeIndentStrip removes one leading indent (a tab or four spaces) from each
	// code line.
	reCodeIndentStrip = regexp.MustCompile(`(?m)^(?:\t| {4})`)
	reFence           = regexp.MustCompile("^(~~~+|```+)\\s*([^`]*?)\\s*$")
	reULItem          = regexp.MustCompile(`^( {0,3})([*+-])(\s+)(.*)$`)
	reOLItem          = regexp.MustCompile(`^( {0,3})(\d+)\.(\s+)(.*)$`)
	reBlockquote      = regexp.MustCompile(`^ {0,3}> ?(.*)$`)
	reHeaderID        = regexp.MustCompile(`[ \t]+\{#([A-Za-z][\w:-]*)\}[ \t]*$`)
	reDefMarker       = regexp.MustCompile(`^( {0,3})(:)(\s+)(.*)$`)
	// reBlockMath matches kramdown's BLOCK_MATH_START = /^OPT_SPACE(\\)?\$\$(.*?)\$\$/m
	// at the start of the remaining source: 0-3 space indent, an optional escaping
	// backslash, then the "$$"-delimited LaTeX captured non-greedily and across
	// newlines ((?s)) so a multi-line "$$\begin…\end$$" is one element. The trailing
	// boundary (kramdown's (\s*?\n)? plus before_block_boundary?) is enforced
	// separately in tryBlockMath.
	reBlockMath = regexp.MustCompile(`(?s)^ {0,3}(\\?)\$\$(.*?)\$\$`)
)

// tryBlockMath parses a "$$…$$" math block (possibly spanning several source
// lines) that stands alone at a block boundary, mirroring kramdown's
// parse_block_math. The math is accepted only when the closing "$$" ends its line
// (nothing but horizontal whitespace after it) and the following line is itself a
// block boundary — a blank line, an EOB "^" marker, a block IAL ("{:…}"), or the
// end of input (kramdown's before_block_boundary?). It returns:
//
//   - el != nil, consumed>0: a block :math element spanning consumed lines.
//   - stripped != "", ok=false: a leading-backslash "\$$…$$" that ends its line —
//     kramdown consumes the backslash and reparses the rest as a paragraph
//     (yielding inline math); stripped is lines[i] with the "OPT_SPACE\" removed.
//   - nil, 0, "" : not a block math element; the caller falls to paragraph parsing
//     (where a bare "$$…$$" may still become inline math, or stay literal text).
func (p *parser) tryBlockMath(lines []string, i int) (el *Element, consumed int, stripped string) {
	rest := strings.Join(lines[i:], "\n")
	m := reBlockMath.FindStringSubmatchIndex(rest)
	if m == nil {
		return nil, 0, ""
	}
	backslash := m[2] != m[3] // group 1 ("\\?") matched a backslash
	content := rest[m[4]:m[5]]
	// afterClose is everything past the closing "$$"; the math's own line ends
	// cleanly when only horizontal whitespace precedes a newline or the end of input.
	afterClose := rest[m[1]:]
	trimmed := strings.TrimLeft(afterClose, " \t")
	cleanEnd := trimmed == "" || trimmed[0] == '\n'
	if backslash {
		// A leading backslash is never block math. When the line ends cleanly kramdown
		// consumes the "OPT_SPACE\" and falls through to paragraph parsing; otherwise
		// the backslash is left in place (the whole line becomes literal paragraph text).
		if !cleanEnd {
			return nil, 0, ""
		}
		return nil, 0, reBlockMathLead.ReplaceAllString(lines[i], "")
	}
	if !cleanEnd {
		return nil, 0, "" // text follows the closing "$$": not a block boundary.
	}
	// before_block_boundary?: after the math line's newline, the next line must be a
	// blank line, an EOB "^" marker, a block IAL, or the end of input.
	if trimmed != "" { // trimmed starts with '\n'; inspect the following line.
		next := trimmed[1:]
		if line, _, _ := strings.Cut(next, "\n"); line != "" &&
			strings.TrimSpace(line) != "" && strings.TrimRight(line, " \t") != "^" {
			if _, ok := matchBlockIAL(line); !ok {
				return nil, 0, ""
			}
		}
	}
	el = newEl(ElMath)
	el.Value = strings.TrimSpace(content)
	consumed = strings.Count(rest[:m[1]], "\n") + 1
	return el, consumed, ""
}

// reBlockMathLead strips the "OPT_SPACE\" (0-3 leading spaces then one backslash)
// that kramdown's parse_block_math consumes from a leading-backslash "\$$…$$" line
// before reparsing the remainder as a paragraph.
var reBlockMathLead = regexp.MustCompile(`^ {0,3}\\`)

// parseOneBlock dispatches on lines[i] to the correct block parser and returns the
// number of input lines consumed (always >= 1). atBoundary reports whether the
// block sits at a block boundary (kramdown's after_block_boundary?), which a table
// candidate requires.
func (p *parser) parseOneBlock(lines []string, i int, parent *Element, atBoundary bool) int {
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
		if p.parseATXHeader(m, parent) {
			return 1
		}
		// The contents collapsed to nothing after stripping closing hashes (e.g.
		// "# #"): kramdown then falls through to treat the line as a paragraph.
		return p.parseParagraph(lines, i, parent)
	}
	if reATXNoSpace.MatchString(strings.TrimRight(line, " ")) {
		// "#" alone is a paragraph in kramdown.
		return p.parseParagraph(lines, i, parent)
	}
	if strings.HasPrefix(line, "{::comment}") || strings.HasPrefix(line, "{::comment ") {
		return p.parseComment(lines, i, parent)
	}
	if n, ok := p.parseBlockExtension(lines, i, parent); ok {
		return n
	}
	if p.blockHTMLStart(lines, i) {
		if n, ok := p.parseBlockHTML(lines, i, parent); ok {
			return n
		}
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
	if math, n, stripped := p.tryBlockMath(lines, i); math != nil {
		parent.addChild(math)
		return n
	} else if stripped != "" {
		// A leading-backslash "\$$…$$" line: kramdown drops the backslash and reparses
		// the remainder (and any lines it continues onto) as a paragraph.
		mod := append([]string(nil), lines...)
		mod[i] = stripped
		return p.parseParagraph(mod, i, parent)
	}
	if atBoundary {
		if tbl, n := p.tryTable(lines, i); tbl != nil {
			parent.addChild(tbl)
			return n
		}
	}
	if dl, n := p.tryDefinitionList(lines, i); dl != nil {
		parent.addChild(dl)
		return n
	}
	return p.parseParagraph(lines, i, parent)
}

// parseATXHeader handles "# Header {#id}" lines. It mirrors the gem: the contents
// are right-stripped, an explicit trailing "{#id}" is extracted first, then a run
// of unescaped trailing "#"s is removed. It reports false (no header emitted) when
// the contents collapse to nothing, so the caller can fall back to a paragraph.
func (p *parser) parseATXHeader(m []string, parent *Element) bool {
	level := len(m[1])
	text := rubyRstrip(m[2])
	// Strip an explicit trailing {#id} (must be preceded by whitespace).
	id := ""
	if idm := reHeaderID.FindStringSubmatch(text); idm != nil {
		id = idm[1]
		text = rubyRstrip(text[:len(text)-len(idm[0])])
	}
	// Strip a trailing run of closing "#"s (kramdown: /(?<!\\)#+\z/ then rstrip).
	text = stripClosingHashes(text)
	if text == "" {
		return false
	}
	h := newEl(ElHeader)
	h.Options["level"] = level
	h.Options["raw_text"] = text
	if id != "" {
		h.Options["explicit_id"] = id
	}
	parent.addChild(h)
	return true
}

// rubyRstrip trims trailing whitespace the way Ruby's String#rstrip does.
func rubyRstrip(s string) string {
	return strings.TrimRight(s, " \t\n\r\f\v")
}

// stripClosingHashes reproduces kramdown's `text.sub!(/(?<!\\)#+\z/, ”) &&
// text.rstrip!`: it removes the maximal trailing run of "#" whose first hash is not
// backslash-escaped, then right-strips. A single escaped hash ("header \#") is
// kept verbatim; the string is returned unchanged when it has no trailing hash.
func stripClosingHashes(s string) string {
	j := len(s)
	for j > 0 && s[j-1] == '#' {
		j--
	}
	if j == len(s) {
		return s // no trailing hashes
	}
	start := j
	if j > 0 && s[j-1] == '\\' {
		// The first "#" of the run is escaped; only the hashes after it are removed.
		start = j + 1
	}
	if start >= len(s) {
		return s // nothing removable (e.g. a lone escaped "\#")
	}
	return rubyRstrip(s[:start])
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
		// A Setext underline promotes the paragraph to a header only when it
		// immediately follows a SINGLE content line at a block boundary — kramdown's
		// SETEXT_HEADER_START matches one content line then the underline. A "="/"-"
		// run after further lines is ordinary paragraph text (so a multi-line
		// paragraph ending in "====" stays a paragraph). A pure run also matches the
		// horizontal-rule pattern, but in this single-line position the underline
		// wins (a real HR needs internal spaces, e.g. "- - -").
		if len(buf) == 1 && reSetextPure.MatchString(line) {
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
		buf = append(buf, line)
		i++
	}
	// Trim trailing whitespace only on the last line (a trailing hard break at the
	// very end of a paragraph is dropped by kramdown).
	if len(buf) > 0 {
		buf[len(buf)-1] = strings.TrimRight(buf[len(buf)-1], " \t")
	}
	// A paragraph that directly follows a native (:html_to_native) :p with no blank
	// line between merges into it: kramdown's parse_paragraph appends the text to the
	// preceding :p as a child rather than opening a new paragraph. This is the mid-line
	// "block element then inline text" continuation (e.g. "<p>…</p> some text"). The
	// leading whitespace of the trailing run is preserved, so the merge keeps buf's
	// first line un-stripped.
	if t := nativeParaMergeTarget(parent); t != nil {
		t.addChild(textEl(strings.Join(buf, "\n")))
		return i - start
	}
	// Strip leading whitespace only from the first line (kramdown left-strips a
	// paragraph's opening line; continuation lines keep their indentation). Keep
	// trailing spaces so the span parser can detect hard line breaks ("  \n").
	if len(buf) > 0 {
		buf[0] = strings.TrimLeft(buf[0], " \t")
	}
	para := newEl(ElP)
	para.Options["raw"] = strings.Join(buf, "\n")
	parent.addChild(para)
	return i - start
}

// nativeParaMergeTarget returns the preceding sibling a following paragraph should
// merge into: a native (:html_to_native) :p whose body is child-based. Only such a
// paragraph can immediately precede another with no intervening blank line (two
// Markdown paragraphs are always blank-separated and coalesced), so this is the sole
// reachable continuation target.
func nativeParaMergeTarget(parent *Element) *Element {
	n := len(parent.Children)
	if n == 0 {
		return nil
	}
	last := parent.Children[n-1]
	if last.Type != ElP {
		return nil
	}
	if hn, _ := last.Options["hnative"].(bool); hn {
		return last
	}
	return nil
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
// interrupts. A list item's content is pre-segmented by parseList (each nested list
// begins its own value segment), so a list marker never needs to interrupt a
// paragraph here.
func (p *parser) startsNewBlock(lines []string, i int) bool {
	// Only a start tag whose name is an HTML *block* element interrupts a running
	// paragraph. A span element, a comment, and a tag that is neither block nor span
	// (script / style / an unknown name) are all absorbed into the paragraph and parsed
	// as span-level — kramdown emits them inside the <p> (e.g. a comment inside a
	// blockquote, or an inline <script> whose raw body follows a paragraph line).
	src := strings.Join(lines[i:], "\n") + "\n"
	name, _, _, _, ok := matchStartTag(stripOptSpace(src))
	return ok && htmlBlockElements[strings.ToLower(name)]
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
		// The block comment closes only on a STANDALONE "{:/comment}" (or the short
		// "{:/}") line; an inline occurrence within a line is part of the content.
		if tl := strings.TrimSpace(line); tl == "{:/comment}" || tl == "{:/}" {
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
	closed := false
	for i < len(lines) {
		if strings.HasPrefix(lines[i], strings.Repeat(closer, len(fence))) &&
			strings.TrimSpace(strings.TrimLeft(lines[i], closer)) == "" {
			i++
			closed = true
			break
		}
		buf = append(buf, lines[i])
		i++
	}
	if !closed {
		// A fence that is never closed is not a code block; kramdown reparses the
		// opening line as ordinary paragraph text.
		return p.parseParagraph(lines, start, parent)
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
	var raw []string
	i := start
	lastContent := false
	for i < len(lines) {
		line := lines[i]
		// A whitespace-only line is a blank separator even when itself indented four
		// spaces: it stays in the block only when another indented line follows (the
		// next group), otherwise it ends the block (and separates it from what comes
		// after).
		if strings.TrimSpace(line) == "" {
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j < len(lines) && reIndentCode.MatchString(lines[j]) && strings.TrimSpace(lines[j]) != "" {
				for ; i < j; i++ {
					raw = append(raw, "")
				}
				lastContent = false
				continue
			}
			break
		}
		if reIndentCode.MatchString(line) {
			raw = append(raw, line)
			i++
			lastContent = true
			continue
		}
		// Lazy continuation: an under-indented (0-3 space) non-blank line right after
		// code content is folded into the block, unless it begins an IAL, EOB or HTML
		// block boundary.
		if lastContent && !p.isLazyCodeBoundary(line) {
			raw = append(raw, line)
			i++
			continue
		}
		break
	}
	// Reproduce kramdown's two rewrites: a newline before an under-indented line
	// becomes a single space (lazy join), then one leading indent is stripped from
	// every remaining line.
	text := strings.Join(raw, "\n") + "\n"
	text = reLazyCodeJoin.ReplaceAllString(text, " $1")
	text = reCodeIndentStrip.ReplaceAllString(text, "")
	cb := newEl(ElCodeblock)
	cb.Value = text
	parent.addChild(cb)
	return i - start
}

// isLazyCodeBoundary reports whether line, appearing where a lazy code-continuation
// could, instead ends the code block (an IAL, an EOB marker, or an HTML block).
func (p *parser) isLazyCodeBoundary(line string) bool {
	if strings.TrimRight(line, " \t") == "^" {
		return true
	}
	if _, ok := matchBlockIAL(line); ok {
		return true
	}
	return isHTMLBlockStart(line)
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
		// A standalone block IAL ends the quote so it attaches to the blockquote
		// itself (via applyStandaloneIAL), not to the quote's last inner paragraph.
		if _, ok := matchBlockIAL(line); ok {
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
	// "{::…}" extensions and "{:/…}" stop tags are not IALs (up to three leading
	// spaces are allowed before the brace, matching kramdown's OPT_SPACE).
	tl := strings.TrimLeft(t, " ")
	if len(t)-len(tl) <= 3 && (strings.HasPrefix(tl, "{::") || strings.HasPrefix(tl, "{:/")) {
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
// IAL attaches to the following block. The raw IAL is stored on the target element
// (Options["block_ial"], accumulating) and resolved after the whole document is
// parsed by resolveBlockIALs, so a forward reference to an ALD defined later in the
// document resolves against the complete ALD table (kramdown defers
// update_attr_with_ial to update_tree). It returns the next index to parse from.
func (p *parser) applyStandaloneIAL(lines []string, i int, ial string, parent *Element) int {
	// ALD definition: "{:name: attrs}". Multiple definitions of the same name
	// accumulate (kramdown's @alds[name] ||= {} then parse_attribute_list merges),
	// so later ids/keys override and classes/refs append.
	if name, attrs, ok := splitALD(ial); ok {
		p.aldDefs[name] = joinIAL(p.aldDefs[name], attrs)
		return i + 1
	}
	// Attach to the previous non-blank block if one exists and is not separated by a
	// blank line. Consecutive trailing IALs all see that block as the last child and
	// so accumulate onto it.
	if n := len(parent.Children); n > 0 && parent.Children[n-1].Type != ElBlank {
		attachBlockIAL(parent.Children[n-1], ial)
		return i + 1
	}
	// Leading position: accumulate this IAL and any immediately-consecutive
	// standalone (non-ALD) block IALs, then attach the run to the block that
	// immediately follows (kramdown's @block_ial, applied by the next new_block_el).
	acc := ial
	k := i + 1
	for k < len(lines) {
		b, ok := matchBlockIAL(lines[k])
		if !ok {
			break
		}
		if _, _, isALD := splitALD(b); isALD {
			break
		}
		acc = joinIAL(acc, b)
		k++
	}
	// A blank line, the end of input, or an ALD (whose own eob consumes the pending
	// block IAL) clears the accumulated IAL — it attaches to nothing. Hand the line
	// back to the block loop.
	if k >= len(lines) || strings.TrimSpace(lines[k]) == "" {
		return k
	}
	if _, ok := matchBlockIAL(lines[k]); ok {
		return k
	}
	before := len(parent.Children)
	// A preceding block IAL makes after_block_boundary? true for the block it
	// decorates (so a table may follow directly).
	consumed := p.parseOneBlock(lines, k, parent, true)
	if len(parent.Children) > before {
		attachBlockIAL(parent.Children[len(parent.Children)-1], acc)
	}
	return k + consumed
}

// attachBlockIAL records raw as the deferred block IAL of el, accumulating onto any
// already recorded (space-joined) so consecutive block IALs merge like kramdown's
// single per-element options[:ial].
func attachBlockIAL(el *Element, raw string) {
	if existing, ok := el.Options["block_ial"].(string); ok {
		el.Options["block_ial"] = joinIAL(existing, raw)
		return
	}
	el.Options["block_ial"] = raw
}

// resolveBlockIALs walks the parsed tree and applies each element's deferred block
// IAL (Options["block_ial"]) now that the ALD table is complete, mirroring
// kramdown's update_tree/update_attr_with_ial post-parse pass.
func (p *parser) resolveBlockIALs(el *Element) {
	if raw, ok := el.Options["block_ial"].(string); ok && raw != "" {
		applyIALToElement(el, raw, p.aldDefs)
	}
	for _, c := range el.Children {
		p.resolveBlockIALs(c)
	}
}
