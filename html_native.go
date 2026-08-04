// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
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
	case "code", "pre":
		nc.convertCode(el)
	default:
		// convert_script (math tags) and convert_textarea are deferred: script/textarea
		// need the raw-body handling of cluster 5 to be reachable cleanly. Their elements
		// fall through to process_html_element and stay :html_element.
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

// convertTable ports convert_table: a <table> that is not a "simple" table (a cell
// holds a block element, rows have differing cell counts, or mixed/justify alignments)
// keeps its raw :html_element form and serialises verbatim. A simple table is mapped
// onto native :table/:thead/:tbody/:tfoot/:tr/:td elements, its per-column alignment
// extracted from the cell styles, and a bare sequence of rows wrapped in a :tbody.
func (nc nativeConverter) convertTable(el *Element) {
	if !nc.isSimpleTable(el) {
		nc.processHTMLElement(el, false, false)
		return
	}
	nc.removeTextChildren(el)
	nc.processChildren(el, true, false)
	nc.setBasics(el, ElTable, nil)
	alignment := nc.calcAlignment(el)
	el.Options["alignment"] = alignment
	// Drop any :html_element children the row/section walk left behind (a simple table
	// carries only native rows/sections).
	var kept []*Element
	for _, c := range el.Children {
		if c.Type != ElHTMLElement {
			kept = append(kept, c)
		}
	}
	el.Children = kept
	// A bare run of rows becomes a single implicit tbody.
	if len(el.Children) > 0 && el.Children[0].Type == ElTr {
		tbody := newEl(ElTbody)
		tbody.Options["hnative"] = true
		tbody.Children = el.Children
		el.Children = []*Element{tbody}
	}
	// Precompute, per cell, the tag kramdown's convert_td emits (a <th> inside a thead,
	// else <td>) and the column alignment, so the renderer needs no ancestor stack.
	nc.annotateCells(el, alignment)
}

// annotateCells walks the converted table recording on each :td the rendered cell tag
// ("th" for a cell in a thead section, otherwise "td") and the column alignment from
// the table's alignment array, mirroring convert_td's stack lookups.
func (nc nativeConverter) annotateCells(el *Element, alignment []string) {
	for _, sec := range el.Children {
		tag := "td"
		if sec.Type == ElThead {
			tag = "th"
		}
		for _, tr := range sec.Children {
			// A simple table's rows all hold the same number of cells (is_simple_table?'s
			// check_nr_cells) and the alignment array is one row's cell count, so every
			// cell index is in range.
			for j, td := range tr.Children {
				td.Options["celltag"] = tag
				td.Options["cellalign"] = alignment[j]
			}
		}
	}
}

// reSliceAlign matches the text-align declaration convert_table slices out of a cell's
// style (with an optional leading "; "), capturing the alignment keyword.
var reSliceAlign = regexp.MustCompile(`(?:;\s*)?text-align:\s+(center|left|right)`)

// calcAlignment ports convert_table's calc_alignment lambda: for every row it records
// the per-column alignment (the last row wins — a simple table's rows all agree) and
// strips the text-align declaration out of each cell's style, deleting an emptied
// style attribute.
func (nc nativeConverter) calcAlignment(el *Element) []string {
	var alignment []string
	var walk func(c *Element)
	walk = func(c *Element) {
		if c.Type == ElTr {
			alignment = nil
			for _, td := range c.Children {
				al := "default"
				if style, ok := td.getAttr("style"); ok {
					if loc := reSliceAlign.FindStringSubmatchIndex(style); loc != nil {
						al = style[loc[2]:loc[3]]
						rest := style[:loc[0]] + style[loc[1]:]
						if strings.TrimSpace(rest) == "" {
							td.deleteAttr("style")
						} else {
							td.setAttr("style", rest)
						}
					}
				}
				alignment = append(alignment, al)
			}
			return
		}
		for _, cc := range c.Children {
			walk(cc)
		}
	}
	walk(el)
	return alignment
}

// reCheckAlign matches a cell's text-align for the simple-table alignment check,
// including the justify/inherit keywords that disqualify a table.
var reCheckAlign = regexp.MustCompile(`text-align:\s+(center|left|right|justify|inherit)`)

// isSimpleTable ports is_simple_table?: a table is "simple" when every cell holds only
// phrasing content, every row has the same non-zero cell count, all rows share the same
// alignment (with no justify/inherit), and its rows are a bare <td> sequence or a
// thead/tbody/tfoot layout containing a tbody.
func (nc nativeConverter) isSimpleTable(el *Element) bool {
	if !nc.checkCells(el) {
		return false
	}
	nr := nc.checkNrCells(el)
	if nr <= 0 {
		return false
	}
	if !nc.checkAlignment(el) {
		return false
	}
	return nc.checkRows(el, "td") || nc.checkSectioned(el)
}

// checkCells ports is_simple_table?'s check_cells: every <th>/<td> must contain only
// phrasing content.
func (nc nativeConverter) checkCells(c *Element) bool {
	if c.Value == "th" || c.Value == "td" {
		return nc.onlyPhrasing(c)
	}
	for _, cc := range c.Children {
		if !nc.checkCells(cc) {
			return false
		}
	}
	return true
}

// onlyPhrasing ports the only_phrasing_content lambda: every descendant is a text node
// or a non-block HTML element.
func (nc nativeConverter) onlyPhrasing(c *Element) bool {
	for _, cc := range c.Children {
		if cc.Type != ElText && htmlBlockElements[cc.Value] {
			return false
		}
		if !nc.onlyPhrasing(cc) {
			return false
		}
	}
	return true
}

// checkNrCells ports check_nr_cells: it returns the common per-row cell count, 0 when
// no row has cells, or -1 when rows disagree.
func (nc nativeConverter) checkNrCells(el *Element) int {
	nr := 0
	var walk func(t *Element)
	walk = func(t *Element) {
		if t.Value == "tr" {
			count := 0
			for _, cc := range t.Children {
				if cc.Value == "th" || cc.Value == "td" {
					count++
				}
			}
			if count != nr {
				if nr == 0 {
					nr = count
				} else {
					nr = -1
				}
			}
			return
		}
		for _, cc := range t.Children {
			walk(cc)
		}
	}
	walk(el)
	return nr
}

// checkAlignment ports check_alignment: all rows must share the same per-column
// alignment and no cell may request justify/inherit.
func (nc nativeConverter) checkAlignment(el *Element) bool {
	var alignment []string
	have := false
	var walk func(t *Element) bool
	walk = func(t *Element) bool {
		if t.Value == "tr" {
			var cur []string
			for _, cell := range t.Children {
				if cell.Value != "th" && cell.Value != "td" {
					continue
				}
				style, _ := cell.getAttr("style")
				m := reCheckAlign.FindStringSubmatch(style)
				if m != nil && (m[1] == "justify" || m[1] == "inherit") {
					return false
				}
				if m == nil {
					cur = append(cur, "default")
				} else {
					cur = append(cur, m[1])
				}
			}
			if !have {
				alignment, have = cur, true
			}
			return stringSliceEqual(alignment, cur)
		}
		for _, cc := range t.Children {
			if !walk(cc) {
				return false
			}
		}
		return true
	}
	return walk(el)
}

// checkRows ports check_rows: every child of t is a row (or text) whose children are
// all cells of the given tag (or text).
func (nc nativeConverter) checkRows(t *Element, cellTag string) bool {
	for _, r := range t.Children {
		if r.Value != "tr" && r.Type != ElText {
			return false
		}
		for _, cc := range r.Children {
			if cc.Value != cellTag && cc.Type != ElText {
				return false
			}
		}
	}
	return true
}

// checkSectioned ports the sectioned branch of is_simple_table?: the table's children
// are thead (of <th> rows) / tbody / tfoot (of <td> rows) sections, with at least one
// tbody present.
func (nc nativeConverter) checkSectioned(el *Element) bool {
	for _, t := range el.Children {
		ok := t.Type == ElText ||
			(t.Value == "thead" && nc.checkRows(t, "th")) ||
			((t.Value == "tfoot" || t.Value == "tbody") && nc.checkRows(t, "td"))
		if !ok {
			return false
		}
	}
	for _, t := range el.Children {
		if t.Value == "tbody" {
			return true
		}
	}
	return false
}

// stringSliceEqual reports whether two string slices have identical elements.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// convertCode ports convert_code / convert_pre (aliases): it flattens the element's
// descendant text, re-decoding every HTML entity to the character it names. Because
// that flattened text always collapses to a single run (kramdown's process_text /
// inject never fails for well-formed content), the element is retyped to :codespan
// (for <code>) or :codeblock (for <pre>) carrying the decoded literal — the renderer
// re-escapes it. A <pre> whose sole child is a <code class="language-x"> hoists that
// language token onto the <pre>, mirroring the gem so a fenced language survives.
func (nc nativeConverter) convertCode(el *Element) {
	var b strings.Builder
	nc.extractText(el, &b)
	decoded := decodeCodeText(b.String())
	if el.Value == "code" {
		nc.setBasics(el, ElCodespan, nil)
		el.Value = decoded
		el.Children = nil
		return
	}
	nc.setBasics(el, ElCodeblock, nil)
	if len(el.Children) == 1 && el.Children[0].Value == "code" {
		if cls, ok := attrVal(el.Children[0].Attrs, "class"); ok {
			if lang := scanLanguageToken(cls); lang != "" {
				existing, _ := attrVal(el.Attrs, "class")
				el.setAttr("class", strings.TrimRight(lang+" "+existing, " "))
			}
		}
	}
	// convert_codeblock chomps the trailing record separator then re-appends "\n"; the
	// port's codeblock renderer writes the value verbatim, so normalise it here.
	el.Value = chompRecordSeparator(decoded) + "\n"
	el.Children = nil
}

// reLanguageToken mirrors convert_code's language-class scan (/\blanguage-\S+/): the
// whole non-whitespace token, used to hoist a fenced language onto a <pre>.
var reLanguageToken = regexp.MustCompile(`\blanguage-\S+`)

// scanLanguageToken returns the first language-<...> token of a class attribute, or
// "" when absent.
func scanLanguageToken(cls string) string {
	return reLanguageToken.FindString(cls)
}

// decodeCodeText re-decodes the HTML entities of a flattened code run to their
// characters, mirroring convert_code's inject: a recognised named entity or a
// numeric/hex reference becomes its character; an unrecognised named entity degrades
// to a literal "&" with its remainder rescanned (kramdown's amp fallback), so the
// text round-trips through the renderer's re-escaping.
func decodeCodeText(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		loc := reHTMLEntity.FindStringSubmatchIndex(s)
		if loc == nil {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:loc[0]])
		name, dec, hex := "", "", ""
		if loc[2] >= 0 {
			name = s[loc[2]:loc[3]]
		}
		if loc[4] >= 0 {
			dec = s[loc[4]:loc[5]]
		}
		if loc[6] >= 0 {
			hex = s[loc[6]:loc[7]]
		}
		if ch, ok := decodeEntity(name, dec, hex); ok {
			b.WriteString(ch)
			s = s[loc[1]:]
		} else {
			b.WriteByte('&')
			s = s[loc[0]+1:]
		}
	}
	return b.String()
}

// decodeEntity resolves one HTML entity (exactly one of name / dec / hex is set) to
// its character, returning false for an unresolvable reference (an unknown named
// entity or an out-of-range code point) so the caller can apply the amp fallback.
func decodeEntity(name, dec, hex string) (string, bool) {
	switch {
	case dec != "":
		return codePointChar(dec, 10)
	case hex != "":
		return codePointChar(hex, 16)
	default:
		return namedCodeEntity(name)
	}
}

// codePointChar parses a numeric code point in the given base and returns its UTF-8
// character, or false when it is malformed or outside the Unicode range.
func codePointChar(digits string, base int) (string, bool) {
	n, err := strconv.ParseInt(digits, base, 32)
	if err != nil || n < 0 || n > unicode.MaxRune {
		return "", false
	}
	return string(rune(n)), true
}

// namedCodeEntity maps a named HTML entity to its character for code decoding. It
// covers the XML built-ins plus the smart-quote / typographic set this port models
// (via symParts); any other name is left unresolved for the amp fallback.
func namedCodeEntity(name string) (string, bool) {
	switch name {
	case "lt":
		return "<", true
	case "gt":
		return ">", true
	case "amp":
		return "&", true
	case "quot":
		return "\"", true
	case "apos":
		return "'", true
	case "nbsp":
		return " ", true
	}
	if parts, ok := symParts[name]; ok {
		var b strings.Builder
		for _, p := range parts {
			b.WriteRune(rune(p.cp))
		}
		return b.String(), true
	}
	return "", false
}

// chompRecordSeparator removes a single trailing record separator (Ruby's String#chomp
// with the default separator: "\r\n", "\n" or "\r").
func chompRecordSeparator(s string) string {
	switch {
	case strings.HasSuffix(s, "\r\n"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "\n"), strings.HasSuffix(s, "\r"):
		return s[:len(s)-1]
	}
	return s
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
