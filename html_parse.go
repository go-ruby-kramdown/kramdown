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
)

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
	// The line-based block driver can only splice whole lines. When the construct ends
	// mid-line (trailing content after a close tag on the same line — a block/span
	// content-model scenario handled in a later cluster), bail out so the caller keeps
	// the line as a paragraph rather than desynchronising the line cursor.
	if !(sc.eos() || src[sc.pos-1] == '\n') {
		return 0, false
	}
	parent.Children = append(parent.Children, tmp.Children...)
	return strings.Count(src[:sc.pos], "\n"), true
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

// handleKramdownHTMLTag reproduces handle_kramdown_html_tag for the raw content model
// (the default when :parse_block_html is off): it records is_closed, and unless the
// element is void, parses its body as raw HTML. The trailing-whitespace scan after the
// element mirrors kramdown's, so the block boundary is consumed exactly once.
func (hp *htmlParser) handleKramdownHTMLTag(el, tree *Element) {
	closed := el.Options["selfclose"].(bool)
	if !closed && htmlElementsWithoutBody[el.Value] {
		closed = true
	}
	el.Options["content_model"] = cmRaw
	el.Options["is_closed"] = closed
	// NOTE: script/style have their verbatim-body (handle_raw_html_tag) special case,
	// which only matters once the block content model lands, added in a later cluster.
	if !closed {
		hp.parseRawHTML(el)
	}
	if !(tree.Type == ElHTMLElement && tree.Options["content_model"] == cmRaw) {
		hp.sc.scanRE(reTrailingWS)
	}
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
