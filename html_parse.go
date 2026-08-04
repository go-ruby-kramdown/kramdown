// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// This file ports kramdown's Kramdown::Parser::Html front-end: the constant element
// classifications and parse_raw_html, which turns embedded raw HTML into a native
// element tree (:html_element / :xml_comment / :xml_pi / text nodes). The markdown
// parser drives it through parseBlockHTML. In this cluster only the RAW content model
// (kramdown's default, i.e. no :parse_block_html) is realised; block/span content
// models and markdown="…" are layered on in later clusters.

// contentModel enumerates kramdown's HTML content models for a parsed element.
const (
	cmRaw   = "raw"
	cmBlock = "block"
	cmSpan  = "span"
)

// contentModelFor returns the native content model kramdown assigns to an HTML
// element name (HTML_CONTENT_MODEL): :block, :span or (for raw elements and every
// unknown name) :raw.
func contentModelFor(name string) string {
	switch {
	case htmlContentModelBlock[name]:
		return cmBlock
	case htmlContentModelSpan[name]:
		return cmSpan
	default:
		return cmRaw
	}
}

// setFromWords turns a whitespace-separated word list into a set.
func setFromWords(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

var (
	htmlContentModelBlock = setFromWords(`address applet article aside blockquote body
		dd details div dl fieldset figure figcaption footer form header hgroup iframe li
		main map menu nav noscript object section summary td`)
	htmlContentModelSpan = setFromWords(`a abbr acronym b bdo big button cite caption del
		dfn dt em h1 h2 h3 h4 h5 h6 i ins label legend optgroup p q rb rbc rp rt rtc ruby
		select small span strong sub sup th tt`)
	htmlContentModelRaw = setFromWords(`script style math option textarea pre code kbd samp var`)

	htmlSpanElements = setFromWords(`a abbr acronym b big bdo br button cite code del dfn em
		i img input ins kbd label mark option q rb rbc rp rt rtc ruby samp select small span
		strong sub sup time tt u var`)
	htmlBlockElements = setFromWords(`address article aside applet body blockquote caption col
		colgroup dd div dl dt fieldset figcaption footer form h1 h2 h3 h4 h5 h6 header hgroup
		hr html head iframe legend menu li main map nav ol optgroup p pre section summary table
		tbody td th thead tfoot tr ul`)
	htmlElementsWithoutBody = setFromWords(`area base br col command embed hr img input keygen
		link meta param source track wbr`)
)

// htmlElementNames is the union that kramdown's HTML_ELEMENT hash marks true — every
// name whose casing is normalised to lowercase and whose attribute names are
// lower-cased when parsing.
var htmlElementNames = func() map[string]bool {
	m := map[string]bool{}
	for _, s := range []map[string]bool{htmlSpanElements, htmlBlockElements, htmlElementsWithoutBody,
		htmlContentModelBlock, htmlContentModelSpan, htmlContentModelRaw} {
		for k := range s {
			m[k] = true
		}
	}
	return m
}()

var (
	reHTMLComment     = regexp.MustCompile(`(?s)^<!--.*?-->`)
	reHTMLInstruction = regexp.MustCompile(`(?s)^<\?.*?\?>`)
	reHTMLCDATA       = regexp.MustCompile(`(?s)^<!\[CDATA\[(.*?)\]\]>`)
)

// htmlBlockStartsAt reports whether kramdown's parse_block_html would consume block
// HTML at the start of src (a fresh, root position): a leading comment, or a start tag
// (matching HTML_TAG_RE) whose name is not a span-only element.
func htmlBlockStartsAt(src string) bool {
	if reHTMLComment.MatchString(src) {
		return true
	}
	rest := stripOptSpace(src)
	name, _, _, _, ok := matchStartTag(rest)
	if !ok {
		return false
	}
	return !htmlSpanElements[strings.ToLower(name)]
}

// stripOptSpace removes up to three leading spaces (kramdown's OPT_SPACE).
func stripOptSpace(s string) string {
	n := 0
	for n < 3 && n < len(s) && s[n] == ' ' {
		n++
	}
	return s[n:]
}

// blockHTMLStart reports whether a block-HTML construct begins at lines[i]. It is the
// paragraph-interruption / block-dispatch predicate: exactly the cases parseBlockHTML
// will consume.
func (p *parser) blockHTMLStart(lines []string, i int) bool {
	return htmlBlockStartsAt(strings.Join(lines[i:], "\n") + "\n")
}

// parseBlockHTML parses one top-level block-HTML construct beginning at lines[start],
// appending the resulting native element(s) to parent, and returns the number of
// source lines consumed and whether it consumed anything (false leaves the caller to
// treat the line as a paragraph).
func (p *parser) parseBlockHTML(lines []string, start int, parent *Element) (int, bool) {
	src := strings.Join(lines[start:], "\n") + "\n"
	sc := &htmlScanner{s: src}
	tmp := newEl(ElRoot)
	hp := &htmlParser{p: p, sc: sc, root: parent}
	if !hp.blockHTML(tmp) {
		return 0, false
	}
	parent.Children = append(parent.Children, tmp.Children...)
	nl := strings.Count(src[:sc.pos], "\n")
	// The line-based block driver splices whole lines. When the construct ends mid-line
	// — trailing content after a close tag on the same line, e.g. "</div> test" or
	// "…</p><p>" under a block/span content model — re-inject the remainder of that line
	// as lines[start+nl] and report only the fully consumed lines, so the caller resumes
	// on the leftover text as its own block.
	if !sc.eos() && src[sc.pos-1] != '\n' {
		// src always ends in the newline appended above, so a mid-line stop always has
		// a following newline that bounds the remainder.
		end := strings.IndexByte(src[sc.pos:], '\n')
		lines[start+nl] = src[sc.pos : sc.pos+end]
	}
	return nl, true
}

// htmlParser threads the scanner and the markdown parser (for warnings/options)
// through the recursive raw-HTML descent.
type htmlParser struct {
	p    *parser
	sc   *htmlScanner
	root *Element
}

// blockHTML reproduces parse_block_html for a fresh position: it handles a leading
// comment or a non-span start tag, appending to tree. It returns whether it consumed a
// construct.
func (hp *htmlParser) blockHTML(tree *Element) bool {
	sc := hp.sc
	if m, ok := sc.scanRE(reHTMLComment); ok {
		el := newEl(ElXMLComment)
		el.Value = m
		el.Options["category"] = cmBlock
		tree.addChild(el)
		sc.scanRE(reTrailingWS)
		return true
	}
	// A start tag preceded by up to three spaces, whose name is not a span element.
	lead := 0
	for lead < 3 && sc.pos+lead < len(sc.s) && sc.s[sc.pos+lead] == ' ' {
		lead++
	}
	name, attrs, selfClose, n, ok := matchStartTag(sc.s[sc.pos+lead:])
	if !ok || htmlSpanElements[strings.ToLower(name)] {
		return false
	}
	sc.pos += lead + n
	el := hp.handleStartTag(name, attrs, selfClose, tree)
	hp.handleKramdownHTMLTag(el, tree)
	return true
}

// handleStartTag reproduces handle_html_start_tag: it builds the :html_element,
// appends it to tree, and (for void elements) marks it closed. It returns the created
// element without yet parsing its body. Value normalisation and attribute lower-casing
// follow kramdown's HTML_ELEMENT rule.
func (hp *htmlParser) handleStartTag(name, attrsStr string, selfClose bool, tree *Element) *Element {
	if htmlElementNames[strings.ToLower(name)] {
		name = strings.ToLower(name)
	}
	inHTMLTag := htmlElementNames[name]
	attrs := parseHTMLAttributes(attrsStr, inHTMLTag)
	el := newEl(ElHTMLElement)
	el.Value = name
	el.Attrs = attrs
	el.Options["category"] = cmBlock
	el.Options["selfclose"] = selfClose
	tree.addChild(el)
	return el
}

// handleKramdownHTMLTag reproduces handle_kramdown_html_tag: it computes the
// element's content model, records is_closed, and (unless the element is void)
// parses its body accordingly — as Markdown blocks for :block, span-level Markdown
// for :span, or verbatim raw HTML for :raw. The content model is :raw whenever the
// enclosing element is itself raw, or when :parse_block_html is off; otherwise it is
// the element's native model (HTML_CONTENT_MODEL). The trailing-whitespace scans
// mirror kramdown's so the block boundary is consumed exactly once.
func (hp *htmlParser) handleKramdownHTMLTag(el, tree *Element) {
	closed := el.Options["selfclose"].(bool)
	if !closed && htmlElementsWithoutBody[el.Value] {
		closed = true
	}
	parentRaw := tree.Type == ElHTMLElement && tree.Options["content_model"] == cmRaw
	cm := cmRaw
	if !parentRaw {
		if hp.p.opts.ParseBlockHTML {
			cm = contentModelFor(el.Value)
		} else {
			cm = cmRaw
		}
	}
	// A markdown="…" attribute overrides the content model regardless of the
	// :parse_block_html option (kramdown's HTML_MARKDOWN_ATTR_MAP): "0" forces raw,
	// "1" the element's native model, "span"/"block" that model. The attribute is
	// always removed from the element; an unrecognised value leaves the model as-is.
	if mv, ok := el.deleteAttr("markdown"); ok {
		switch mv {
		case "0":
			cm = cmRaw
		case "1":
			cm = contentModelFor(el.Value)
		case "span":
			cm = cmSpan
		case "block":
			cm = cmBlock
		}
	}
	// NOTE: script/style have their verbatim-body (handle_raw_html_tag) special case,
	// layered on in a later cluster.
	if cm == cmBlock {
		hp.sc.scanRE(reTrailingWS)
	}
	el.Options["content_model"] = cm
	el.Options["is_closed"] = closed
	if !closed {
		switch cm {
		case cmBlock:
			if !hp.parseBlockContentModel(el) {
				hp.p.warn("Found no end tag for '" + el.Value + "' - auto-closing it")
			}
		case cmSpan:
			hp.parseSpanContentModel(el)
		default:
			hp.parseRawHTML(el)
		}
	}
	if !parentRaw {
		hp.sc.scanRE(reTrailingWS)
	}
}

// optSpaceLen returns the length (0..3) of the leading run of spaces at s[i:],
// kramdown's OPT_SPACE.
func optSpaceLen(s string, i int) int {
	n := 0
	for n < 3 && i+n < len(s) && s[i+n] == ' ' {
		n++
	}
	return n
}

// matchCloseTagFor reports whether s begins (after up to three OPT_SPACE spaces)
// with the close tag that terminates el's block content model — kramdown's
// parse_block_html close branch: "</name\s*>" whose lower-cased name equals el's
// value and is not a span-only element. It returns the byte length consumed.
func matchCloseTagFor(s string, el *Element) (int, bool) {
	lead := optSpaceLen(s, 0)
	m := reHTMLTagCloseAnchor.FindStringSubmatch(s[lead:])
	if m == nil {
		return 0, false
	}
	name := m[1]
	if htmlElementNames[strings.ToLower(name)] {
		name = strings.ToLower(name)
	}
	if htmlSpanElements[strings.ToLower(name)] || name != el.Value {
		return 0, false
	}
	return lead + len(m[0]), true
}

// boundaryAt reports whether a fresh block-HTML construct — the matching close tag
// for el, a column-0 comment, or an OPT_SPACE non-span start tag — begins at s[pos:].
// These are exactly the lines at which parse_block_html interrupts the Markdown
// block flow inside el's block content model.
func (hp *htmlParser) boundaryAt(pos int, el *Element) bool {
	s := hp.sc.s[pos:]
	if _, ok := matchCloseTagFor(s, el); ok {
		return true
	}
	if reHTMLComment.MatchString(s) {
		return true
	}
	lead := optSpaceLen(s, 0)
	if name, _, _, _, ok := matchStartTag(s[lead:]); ok && !htmlSpanElements[strings.ToLower(name)] {
		return true
	}
	return false
}

// parseBlockContentModel ports handle_kramdown_html_tag's :block branch (parse_blocks
// over the shared scanner). It threads a stop-tag context — el — through the Markdown
// block flow: runs of non-HTML lines are collected and reparsed as Markdown blocks,
// column-0 comments and OPT_SPACE non-span start tags are handled inline (recursing
// into their own content models), and el's own close tag ends the element. It returns
// true when that close tag is found, false when the input ends first (auto-close).
func (hp *htmlParser) parseBlockContentModel(el *Element) bool {
	sc := hp.sc
	for !sc.eos() {
		if n, ok := matchCloseTagFor(sc.rest(), el); ok {
			sc.pos += n
			return true
		}
		if hp.blockHTMLConstruct(el) {
			continue
		}
		region, adv := hp.collectMarkdownRegion(el)
		sc.pos += adv
		hp.p.parseBlocks(strings.Split(region, "\n"), el)
	}
	return false
}

// blockHTMLConstruct handles one inline block-HTML construct at the scanner's current
// (line-start) position — a column-0 comment or an OPT_SPACE non-span start tag —
// appending it to el and returning true, or returning false without moving when no
// such construct begins here.
func (hp *htmlParser) blockHTMLConstruct(el *Element) bool {
	sc := hp.sc
	if m, ok := sc.scanRE(reHTMLComment); ok {
		c := newEl(ElXMLComment)
		c.Value = m
		c.Options["category"] = cmBlock
		el.addChild(c)
		sc.scanRE(reTrailingWS)
		return true
	}
	lead := optSpaceLen(sc.s, sc.pos)
	name, attrs, selfClose, n, ok := matchStartTag(sc.s[sc.pos+lead:])
	if !ok || htmlSpanElements[strings.ToLower(name)] {
		return false
	}
	sc.pos += lead + n
	child := hp.handleStartTag(name, attrs, selfClose, el)
	hp.handleKramdownHTMLTag(child, el)
	return true
}

// collectMarkdownRegion accumulates whole lines from the scanner's current position
// up to (but not including) the next block-HTML boundary line or the end of input,
// returning the collected text (with the trailing newline stripped) and the number of
// bytes consumed. The first line is always included: the caller only reaches here when
// that line is neither el's close tag nor a block-HTML construct.
func (hp *htmlParser) collectMarkdownRegion(el *Element) (string, int) {
	s := hp.sc.s
	start := hp.sc.pos
	p := start
	for {
		nl := strings.IndexByte(s[p:], '\n')
		if nl < 0 {
			p = len(s)
			break
		}
		p += nl + 1
		if p >= len(s) || hp.boundaryAt(p, el) {
			break
		}
	}
	return strings.TrimSuffix(s[start:p], "\n"), p - start
}

// parseSpanContentModel ports handle_kramdown_html_tag's :span branch: it captures the
// element's body verbatim up to its close tag (or the end of input) and stores it as
// span-level Markdown, which the converter parses when rendering. The close tag match
// is case-insensitive for known HTML elements, mirroring kramdown's stop regexp.
func (hp *htmlParser) parseSpanContentModel(el *Element) {
	sc := hp.sc
	re := hp.spanCloseRE(el.Value)
	rest := sc.rest()
	if loc := re.FindStringIndex(rest); loc != nil {
		el.Options["raw"] = rest[:loc[0]]
		sc.pos += loc[1]
		return
	}
	el.Options["raw"] = rest
	sc.terminate()
	hp.p.warn("Found no end tag for '" + el.Value + "' - auto-closing it")
}

// spanCloseRE builds the close-tag regexp for a span content model element, case
// insensitive for known HTML elements (kramdown applies /i when HTML_ELEMENT[name]).
func (hp *htmlParser) spanCloseRE(name string) *regexp.Regexp {
	pat := `</` + regexp.QuoteMeta(name) + `\s*>`
	if htmlElementNames[strings.ToLower(name)] {
		pat = `(?i)` + pat
	}
	return regexp.MustCompile(pat)
}

var reHTMLTagCloseAnchor = regexp.MustCompile(`^</(` + reUNAME + `)\s*>`)

// parseRawHTML reproduces parse_raw_html: it walks the scanner, storing text runs and
// nested comments / PIs / CDATA / elements as children of el, until el's matching close
// tag or end of input.
func (hp *htmlParser) parseRawHTML(el *Element) {
	sc := hp.sc
	fold := htmlElementNames[el.Value]
	for !sc.eos() {
		text, ok := sc.scanUntilRawStart()
		if !ok {
			addText(el, sc.rest())
			sc.terminate()
			if el.Type == ElHTMLElement {
				hp.p.warn("Found no end tag for '" + el.Value + "' - auto-closing it")
			}
			return
		}
		addText(el, text)
		if m, ok := sc.scanRE(reHTMLComment); ok {
			c := newEl(ElXMLComment)
			c.Value = m
			c.Options["category"] = cmBlock
			el.addChild(c)
			continue
		}
		if m, ok := sc.scanRE(reHTMLInstruction); ok {
			pi := newEl(ElXMLPI)
			pi.Value = m
			pi.Options["category"] = cmBlock
			el.addChild(pi)
			continue
		}
		if m := reHTMLCDATA.FindStringSubmatch(sc.rest()); m != nil {
			sc.pos += len(m[0])
			addText(el, m[1])
			continue
		}
		if name, attrs, selfClose, n, tagOK := matchStartTag(sc.rest()); tagOK {
			sc.pos += n
			child := hp.handleStartTag(name, attrs, selfClose, el)
			hp.handleKramdownHTMLTag(child, el)
			continue
		}
		if cm := reHTMLTagCloseAnchor.FindStringSubmatch(sc.rest()); cm != nil {
			want := el.Value
			got := cm[1]
			if fold {
				got = strings.ToLower(got)
			}
			if el.Value == want && want == got {
				sc.pos += len(cm[0])
				return
			}
			addText(el, cm[0])
			sc.pos += len(cm[0])
			hp.p.warn("Found invalidly used HTML closing tag for '" + cm[1] + "' - ignoring it")
			continue
		}
		addText(el, sc.getch())
	}
}

// addText appends text as a :text child (coalescing with a trailing text child),
// mirroring the base parser's add_text with the default :text type.
func addText(tree *Element, text string) {
	if text == "" {
		return
	}
	if n := len(tree.Children); n > 0 && tree.Children[n-1].Type == ElText {
		tree.Children[n-1].Value += text
		return
	}
	t := newEl(ElText)
	t.Value = text
	tree.addChild(t)
}
