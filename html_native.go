// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// This file ports kramdown's Kramdown::Parser::Html::ElementConverter, the
// :html_to_native transform that maps a parsed raw-HTML element tree onto native
// kramdown elements where possible: <b>/<strong> -> :strong, <i>/<em> -> :em,
// <h1>.. -> :header, <code>/<pre> -> :codespan/:codeblock, a simple <table> ->
// :table, and the list/paragraph/blockquote containers, converting entities in text
// and normalising whitespace. Elements that cannot be represented natively keep their
// :html_element form. The converted subtree is serialised by the dedicated,
// child-based renderer in html_native_render.go (kept apart from the markdown-native
// converter, whose block elements render from lazily re-parsed raw strings).

var (
	nativeRemoveTextChildren = setFromWords(`html head hgroup ol ul dl table colgroup
		tbody thead tfoot tr select optgroup`)
	nativeWrapTextChildren = setFromWords(`body section nav article aside header footer
		address div li dd blockquote figure figcaption fieldset form`)
	nativeRemoveWhitespaceChildren = setFromWords(`body section nav article aside header
		footer address div li dd blockquote figure figcaption td th fieldset form`)
	nativeStripWhitespace = setFromWords(`address article aside blockquote body caption dd
		div dl dt fieldset figcaption form footer header h1 h2 h3 h4 h5 h6 legend li nav p
		section td th`)
	// br and img are omitted from kramdown's SIMPLE_ELEMENTS here on purpose: their
	// native forms (ElBr renders a trailing newline for markdown hard breaks; ElImg is
	// a separate span node) would diverge from convert_br / convert_img's bare
	// "<br />" / "<img … />". Left as :html_element they serialise byte-identically
	// (void element, no trailing newline) — so the output matches without the native
	// conversion's side effects (e.g. a <br> inside a table cell).
	nativeSimpleElements = setFromWords(`em strong blockquote hr p thead tbody tfoot
		tr td th ul ol dl li dt dd`)
)

// nativeElementType maps a SIMPLE_ELEMENTS tag name to the native element type
// set_basics assigns it. th shares :td (kramdown renders a thead cell as <th>).
var nativeElementType = map[string]ElementType{
	"em": ElEm, "strong": ElStrong, "blockquote": ElBlockquote, "hr": ElHR,
	"p": ElP, "thead": ElThead, "tbody": ElTbody, "tfoot": ElTfoot,
	"tr": ElTr, "td": ElTd, "th": ElTd, "ul": ElUL, "ol": ElOL, "dl": ElDL, "li": ElLI,
	"dt": ElDT, "dd": ElDD,
}

// reHTMLEntity mirrors kramdown's HTML_ENTITY_RE: a named, decimal or hex entity.
var reHTMLEntity = regexp.MustCompile(`&([\w:][\w.:-]*);|&#(\d+);|&#x([0-9a-fA-F]+);`)

var reWSRun = regexp.MustCompile(`\s+`)

// nativeConverter carries the transform state (kramdown's ElementConverter holds
// @root; this port needs nothing more, but a receiver keeps the method set tidy).
type nativeConverter struct{}

// convertHTMLToNative runs the ElementConverter over el and its subtree in place,
// mirroring ElementConverter.convert(root, el). el is a top-level parsed HTML node
// (an :html_element / :xml_comment); parent is nil at that top call.
func convertHTMLToNative(el *Element) {
	(nativeConverter{}).process(el, true, false, nil)
}

// process ports ElementConverter#process: it converts el and its children. A comment
// / PI gets its category from the parent; an :html_element is dispatched to a
// convert_<name> method, or mapped through the simple-element / process_html_element
// path with the trailing strip / whitespace-removal / text-wrapping steps.
func (nc nativeConverter) process(el *Element, doConversion, preserveText bool, parent *Element) {
	switch el.Type {
	case ElXMLComment, ElXMLPI:
		ptype := nativeParentTagName(parent)
		el.Options = map[string]any{}
		if contentModelFor(ptype) == cmSpan {
			el.Options["category"] = cmSpan
		} else {
			el.Options["category"] = cmBlock
		}
		return
	case ElHTMLElement:
		// fall through to the conversion body below
	default:
		return
	}

	if doConversion && nc.dispatchConvert(el) {
		return
	}
	tag := el.Value
	if doConversion && nativeRemoveTextChildren[tag] {
		nc.removeTextChildren(el)
	}
	if doConversion && nativeSimpleElements[tag] {
		nc.setBasics(el, nativeElementType[tag], nil)
		nc.processChildren(el, doConversion, preserveText)
	} else {
		nc.processHTMLElement(el, doConversion, preserveText)
	}
	if doConversion {
		if nativeStripWhitespace[tag] {
			nc.stripWhitespace(el)
		}
		if nativeRemoveWhitespaceChildren[tag] {
			nc.removeWhitespaceChildren(el)
		}
		if nativeWrapTextChildren[tag] {
			nc.wrapTextChildren(el)
		}
	}
}

// dispatchConvert invokes the convert_<name> handler for el's tag, returning whether
// one existed (and ran). It mirrors method_defined?("convert_#{value}").
func (nc nativeConverter) dispatchConvert(el *Element) bool {
	switch el.Value {
	case "a":
		nc.convertA(el)
	case "em", "i", "strong", "b":
		nc.convertEm(el)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		nc.convertH(el)
	case "table":
		nc.convertTable(el)
	default:
		// convert_script (math tags), convert_textarea and convert_code/convert_pre are
		// deferred: script/textarea need the raw-body handling of cluster 5 to be
		// reachable cleanly, and codeblock rendering pulls in the highlighter. Their
		// elements fall through to process_html_element and stay :html_element.
		return false
	}
	return true
}

// processChildren ports process_children: each text child becomes the whitespace /
// entity-processed element run process_text yields; each element child is processed
// recursively with el as its parent.
func (nc nativeConverter) processChildren(el *Element, doConversion, preserveText bool) {
	var out []*Element
	for _, c := range el.Children {
		if c.Type == ElText {
			out = append(out, nc.processText(c.Value, preserveText || !doConversion)...)
		} else {
			nc.process(c, doConversion, preserveText, el)
			out = append(out, c)
		}
	}
	el.Children = out
}

// processText ports process_text: it compresses whitespace (unless preserve) and
// splits the run on HTML entities into :text, smart-quote / typographic-symbol
// (ElTypographicSym) and plain-entity elements. A plain entity is kept as a text node
// preserving its "&name;" form (the port's entity-output convention), which renders
// unchanged and reduces correctly inside convert_code.
func (nc nativeConverter) processText(raw string, preserve bool) []*Element {
	if !preserve {
		raw = reWSRun.ReplaceAllString(raw, " ")
	}
	var out []*Element
	for len(raw) > 0 {
		loc := reHTMLEntity.FindStringSubmatchIndex(raw)
		if loc == nil {
			out = append(out, textEl(raw))
			break
		}
		if loc[0] > 0 {
			out = append(out, textEl(raw[:loc[0]]))
		}
		name := ""
		if loc[2] >= 0 {
			name = raw[loc[2]:loc[3]]
		}
		switch {
		case isSmartQuoteEntity(name):
			out = append(out, typoSym(name))
		case isTypographicEntity(name):
			out = append(out, typoSym(name))
		default:
			out = append(out, textEl(raw[loc[0]:loc[1]]))
		}
		raw = raw[loc[1]:]
	}
	return out
}

func isSmartQuoteEntity(n string) bool {
	switch n {
	case "lsquo", "rsquo", "ldquo", "rdquo":
		return true
	}
	return false
}

func isTypographicEntity(n string) bool {
	switch n {
	case "mdash", "ndash", "hellip", "laquo", "raquo":
		return true
	}
	return false
}

// processHTMLElement ports process_html_element: it records the element's native
// category (span/block) and content model, then processes its children. When
// do_conversion is false the content model is forced to raw.
func (nc nativeConverter) processHTMLElement(el *Element, doConversion, preserveText bool) {
	cat := cmBlock
	if htmlSpanElements[el.Value] {
		cat = cmSpan
	}
	cm := cmRaw
	if doConversion {
		cm = contentModelFor(el.Value)
	}
	el.Options = map[string]any{"category": cat, "content_model": cm, "hnative": true}
	nc.processChildren(el, doConversion, preserveText)
}

// setBasics ports set_basics: it retypes el to a native element, replaces its options
// with opts (plus the hnative marker so the dedicated renderer claims it) and clears
// its value.
func (nc nativeConverter) setBasics(el *Element, t ElementType, opts map[string]any) {
	el.Type = t
	if opts == nil {
		opts = map[string]any{}
	}
	opts["hnative"] = true
	el.Options = opts
	el.Value = ""
}

// removeTextChildren ports remove_text_children: it drops every direct text child.
func (nc nativeConverter) removeTextChildren(el *Element) {
	var out []*Element
	for _, c := range el.Children {
		if c.Type != ElText {
			out = append(out, c)
		}
	}
	el.Children = out
}

// wrapTextChildren ports wrap_text_children: consecutive non-block (or text) children
// are grouped into a transparent :p, block children stay as-is.
func (nc nativeConverter) wrapTextChildren(el *Element) {
	var out []*Element
	lastIsP := false
	for _, c := range el.Children {
		if !nativeIsBlock(c) || c.Type == ElText {
			if !lastIsP {
				p := newEl(ElP)
				p.Options = map[string]any{"transparent": true, "hnative": true}
				out = append(out, p)
				lastIsP = true
			}
			last := out[len(out)-1]
			last.Children = append(last.Children, c)
		} else {
			out = append(out, c)
			lastIsP = false
		}
	}
	el.Children = out
}

// stripWhitespace ports strip_whitespace: left-strip the first and right-strip the
// last child when they are text nodes.
func (nc nativeConverter) stripWhitespace(el *Element) {
	if len(el.Children) == 0 {
		return
	}
	if f := el.Children[0]; f.Type == ElText {
		f.Value = strings.TrimLeft(f.Value, " \t\r\n\f\v")
	}
	if l := el.Children[len(el.Children)-1]; l.Type == ElText {
		l.Value = strings.TrimRight(l.Value, " \t\r\n\f\v")
	}
}

// removeWhitespaceChildren ports remove_whitespace_children: a whitespace-only text
// node is dropped when it is first, last, or sits between two block siblings.
func (nc nativeConverter) removeWhitespaceChildren(el *Element) {
	var out []*Element
	n := len(el.Children)
	for i, c := range el.Children {
		if c.Type == ElText && strings.TrimSpace(c.Value) == "" &&
			(i == 0 || i == n-1 || (nativeIsBlock(el.Children[i-1]) && nativeIsBlock(el.Children[i+1]))) {
			continue
		}
		out = append(out, c)
	}
	el.Children = out
}

// extractText ports extract_text: it concatenates every descendant text value.
func (nc nativeConverter) extractText(el *Element, b *strings.Builder) {
	if el.Type == ElText {
		b.WriteString(el.Value)
	}
	for _, c := range el.Children {
		nc.extractText(c, b)
	}
}

// convertA ports convert_a: an anchor with an href becomes a native :a; otherwise it
// stays a (raw) html element.
func (nc nativeConverter) convertA(el *Element) {
	if _, ok := attrVal(el.Attrs, "href"); ok {
		nc.setBasics(el, ElA, nil)
		nc.processChildren(el, true, false)
	} else {
		nc.processHTMLElement(el, false, false)
	}
}

var emphasisTypeMap = map[string]ElementType{"em": ElEm, "i": ElEm, "strong": ElStrong, "b": ElStrong}

// convertEm ports convert_em (and its b/strong/i aliases): the element converts to
// :em/:strong only when its text has no leading or trailing whitespace; otherwise it
// stays a raw html element (kramdown keeps "<em> x</em>" verbatim).
func (nc nativeConverter) convertEm(el *Element) {
	var b strings.Builder
	nc.extractText(el, &b)
	text := b.String()
	if hasLeadingOrTrailingSpace(text) {
		nc.processHTMLElement(el, false, false)
		return
	}
	nc.setBasics(el, emphasisTypeMap[el.Value], nil)
	nc.processChildren(el, true, false)
}

// convertH ports convert_h1 (and h2..h6): the element becomes a native :header whose
// level is the tag digit and whose raw_text (for auto-ids) is its extracted text.
func (nc nativeConverter) convertH(el *Element) {
	level := int(el.Value[1] - '0')
	var b strings.Builder
	nc.extractText(el, &b)
	nc.setBasics(el, ElHeader, map[string]any{"level": level, "raw_text": b.String()})
	nc.processChildren(el, true, false)
}

// convertTable ports the non-simple branch of convert_table: a <table> that is not a
// "simple" table (a cell holds a block element, rows have differing cell counts, or
// mixed alignments) keeps its raw :html_element form with its content parsed raw, so
// it serialises verbatim. The simple-table native mapping (:table/:thead/:tbody with
// alignment) is deferred to a later PR; every simple table therefore also renders
// verbatim here.
func (nc nativeConverter) convertTable(el *Element) {
	nc.processHTMLElement(el, false, false)
}

// nativeParentTagName returns the tag name used to classify a comment/PI's category
// from its parent (kramdown's ptype): div at the top level, the tag name for an
// element that stayed :html_element, or the converted native element's tag name (a
// :header maps to "h1"). The category then decides whether a comment inside a
// span-content container such as a <p>/<h1> serialises inline rather than as an
// indented block line.
func nativeParentTagName(parent *Element) string {
	if parent == nil {
		return "div"
	}
	if parent.Type == ElHTMLElement {
		return parent.Value
	}
	return nativeTagNames[parent.Type]
}

// nativeTagNames maps a converted native element type back to the tag name kramdown
// uses for its content-model lookup (:header -> "h1"); an absent type yields "".
var nativeTagNames = map[ElementType]string{
	ElHeader: "h1", ElP: "p", ElEm: "em", ElStrong: "strong", ElBlockquote: "blockquote",
	ElUL: "ul", ElOL: "ol", ElLI: "li", ElDL: "dl", ElDT: "dt", ElDD: "dd", ElA: "a",
}

// nativeIsBlock reports whether c is a block-level element in the converted tree
// (kramdown's Element#block?), used by the whitespace / wrap helpers.
func nativeIsBlock(c *Element) bool {
	switch c.Type {
	case ElP, ElHeader, ElBlockquote, ElCodeblock, ElUL, ElOL, ElLI, ElDL, ElDT, ElDD,
		ElTable, ElThead, ElTbody, ElTfoot, ElTr, ElTd, ElHR:
		return true
	case ElHTMLElement, ElXMLComment, ElXMLPI:
		return c.Options["category"] == cmBlock
	}
	return false
}

// hasLeadingOrTrailingSpace reports whether s starts or ends with an ASCII whitespace
// character (kramdown's /\A\s/ || /\s\z/).
func hasLeadingOrTrailingSpace(s string) bool {
	if s == "" {
		return false
	}
	return isHTMLSpace(s[0]) || isHTMLSpace(s[len(s)-1])
}

func isHTMLSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}

// textEl builds a plain :text element.
func textEl(s string) *Element {
	t := newEl(ElText)
	t.Value = s
	return t
}
