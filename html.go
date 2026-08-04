// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// htmlConverter renders a parsed Document to HTML, reproducing kramdown's exact
// indentation and footnote bookkeeping.
type htmlConverter struct {
	doc       *Document
	usedIds   map[string]bool
	footOrder []string       // footnote ids in first-reference order
	footNums  map[string]int // id -> assigned number
	footRefs  map[string]int // id -> how many times referenced (for backlinks)
	footDefs  map[string]*Element
}

// newHTMLConverter builds a converter bound to doc.
func newHTMLConverter(doc *Document) *htmlConverter {
	return &htmlConverter{
		doc:      doc,
		usedIds:  map[string]bool{},
		footNums: map[string]int{},
		footRefs: map[string]int{},
		footDefs: map[string]*Element{},
	}
}

// convert renders the whole document, appending the collected footnotes.
func (c *htmlConverter) convert() string {
	var b strings.Builder
	c.convertChildren(c.doc.Root.Children, &b, 0)
	out := b.String()
	out = c.appendFootnotes(out)
	return out
}

// convertChildren renders a sequence of block elements at the given indent level.
// It mirrors kramdown's converter exactly: every rendered block already ends in a
// newline, and each blank-line separator (a single ElBlank node, into which the
// parser collapses any run of blank lines) contributes exactly one "\n" — so a
// leading, trailing, or all-blank document emits the gem's lone "\n".
func (c *htmlConverter) convertChildren(els []*Element, b *strings.Builder, indent int) {
	prevBlank := false
	for _, e := range els {
		if e.Type == ElBlank {
			// Collapse a run of blanks (an EOB "^" between blank lines can split one
			// run into adjacent ElBlank nodes) to the single "\n" kramdown emits.
			if !prevBlank {
				b.WriteByte('\n')
			}
			prevBlank = true
			continue
		}
		prevBlank = false
		c.convertBlock(e, b, indent)
	}
}

// ind returns indent*2 spaces (kramdown indents nested blocks two spaces per
// level).
func ind(n int) string { return strings.Repeat("  ", n) }

// convertBlock renders one block element.
func (c *htmlConverter) convertBlock(e *Element, b *strings.Builder, indent int) {
	pad := ind(indent)
	switch e.Type {
	case ElP:
		raw, _ := e.Options["raw"].(string)
		els := c.doc.parseSpansFor(raw)
		// A paragraph whose sole content is a single image carrying the "standalone"
		// IAL reference renders as an HTML5 <figure>/<figcaption> (kramdown's
		// convert_standalone_image), not a <p>.
		if len(els) == 1 && els[0].Type == ElImg && ialHasRef(els[0], "standalone") {
			c.convertStandaloneImage(e, els[0], b, indent)
			return
		}
		els = c.applyTypography(els)
		var sb strings.Builder
		c.renderSpanEls(els, &sb, indent)
		b.WriteString(pad + "<p" + c.attrStr(e) + ">" + sb.String() + "</p>\n")
	case ElHeader:
		c.convertHeader(e, b, indent)
	case ElHR:
		b.WriteString(pad + "<hr" + c.attrStr(e) + " />\n")
	case ElBlockquote:
		b.WriteString(pad + "<blockquote" + c.attrStr(e) + ">\n")
		c.convertChildren(e.Children, b, indent+1)
		b.WriteString(pad + "</blockquote>\n")
	case ElCodeblock:
		c.convertCodeblock(e, b, indent)
	case ElUL, ElOL:
		c.convertList(e, b, indent)
	case ElDL:
		c.convertDL(e, b, indent)
	case ElTable:
		c.convertTable(e, b, indent)
	case ElHTMLBlock:
		b.WriteString(pad + e.Value + "\n")
	case ElComment:
		b.WriteString(pad + "<!-- " + e.Value + " -->\n")
	case ElRaw:
		// A {::nomarkdown} block emits its content verbatim when its target-format
		// filter is empty or names html; otherwise it renders nothing.
		if types, _ := e.Options["types"].([]string); rawForHTML(types) {
			b.WriteString(e.Value + "\n")
		}
	}
}

// convertStandaloneImage renders a single-image paragraph as an HTML5 figure,
// mirroring kramdown's convert_standalone_image. The figure inherits the
// paragraph's own (block-IAL) attributes; the image's class/id are hoisted to the
// figure only when the figure does not already carry that attribute, and every
// other image attribute stays on the <img>.
func (c *htmlConverter) convertStandaloneImage(p, img *Element, b *strings.Builder, indent int) {
	pad := ind(indent)
	inner := ind(indent + 1)
	figAttr := append([]Attr(nil), p.Attrs...)
	imgAttr := append([]Attr(nil), img.Attrs...)
	// Hoist class then id (kramdown's order) from the image to the figure when the
	// figure lacks that attribute; drop it from the image in that case.
	for _, name := range []string{"class", "id"} {
		if hasAttr(figAttr, name) {
			continue
		}
		if v, rest, ok := takeAttr(imgAttr, name); ok {
			figAttr = append(figAttr, Attr{Name: name, Val: v})
			imgAttr = rest
		}
	}
	alt, _ := attrVal(imgAttr, "alt")
	b.WriteString(pad + "<figure" + attrsStr(figAttr) + ">\n")
	b.WriteString(inner + "<img" + attrsStr(imgAttr) + " />\n")
	b.WriteString(inner + "<figcaption>" + alt + "</figcaption>\n")
	b.WriteString(pad + "</figure>\n")
}

// attrsStr renders an attribute slice as ` name="val"` pairs in order, escaping
// each value like an HTML attribute.
func attrsStr(attrs []Attr) string {
	var b strings.Builder
	for _, a := range attrs {
		b.WriteString(" " + a.Name + `="` + escapeHTMLAttr(a.Val) + `"`)
	}
	return b.String()
}

// hasAttr reports whether attrs contains an attribute named name.
func hasAttr(attrs []Attr, name string) bool {
	_, ok := attrVal(attrs, name)
	return ok
}

// attrVal returns the value of the named attribute and whether it is present.
func attrVal(attrs []Attr, name string) (string, bool) {
	for _, a := range attrs {
		if a.Name == name {
			return a.Val, true
		}
	}
	return "", false
}

// takeAttr returns the value of the named attribute, the slice with that
// attribute removed, and whether it was present (attrs unchanged if absent).
func takeAttr(attrs []Attr, name string) (string, []Attr, bool) {
	for i, a := range attrs {
		if a.Name == name {
			return a.Val, append(attrs[:i:i], attrs[i+1:]...), true
		}
	}
	return "", attrs, false
}

// convertHeader renders an ATX/Setext header with its (explicit or generated) id.
func (c *htmlConverter) convertHeader(e *Element, b *strings.Builder, indent int) {
	level := e.Options["level"].(int)
	raw, _ := e.Options["raw_text"].(string)
	inner := c.renderRaw(raw, indent)
	if id, ok := e.getAttr("id"); ok {
		// An id already set by an attached IAL wins over both the explicit {#id} and
		// the auto-generated id.
		c.usedIds[id] = true
	} else if id, ok := e.Options["explicit_id"].(string); ok {
		e.setAttr("id", id)
		c.usedIds[id] = true
	} else if c.doc.Opts.AutoIds {
		id := c.generateId(raw)
		if id != "" {
			e.setAttr("id", c.doc.Opts.AutoIdPrefix+id)
		}
	}
	tag := "h" + strconv.Itoa(level)
	b.WriteString(ind(indent) + "<" + tag + c.attrStr(e) + ">" + inner + "</" + tag + ">\n")
}

var reIdStrip = regexp.MustCompile(`[^a-zA-Z0-9 -]`)
var reIdLead = regexp.MustCompile(`^[^a-zA-Z]+`)
var reIdSpace = regexp.MustCompile(`\s+`)

// generateId derives a header id from its raw text the way kramdown's auto_ids do,
// de-duplicating with a "-N" suffix.
func (c *htmlConverter) generateId(raw string) string {
	// Render to plain text (markup stripped), then slug.
	plain := plainText(c.doc.parseSpansFor(raw))
	s := reIdStrip.ReplaceAllString(plain, "")
	s = strings.TrimSpace(s)
	s = reIdLead.ReplaceAllString(s, "")
	s = strings.ToLower(s)
	s = reIdSpace.ReplaceAllString(s, "-")
	if s == "" {
		s = "section"
	}
	base := s
	n := 1
	for c.usedIds[s] {
		s = base + "-" + strconv.Itoa(n)
		n++
	}
	c.usedIds[s] = true
	return s
}

// convertCodeblock renders a code block. When the Rouge highlighter is enabled and
// resolves a lexer for the block's language, it emits the gem's nested
// highlighter-rouge markup; otherwise it degrades to the plain <pre><code> form,
// carrying a language-<x> class on the <code> element.
func (c *htmlConverter) convertCodeblock(e *Element, b *strings.Builder, indent int) {
	pad := ind(indent)
	// The fenced info string wins; only when it is absent is a language-<x> class
	// consumed off the element (matching kramdown's extract_code_language).
	lang, _ := e.Options["lang"].(string)
	classExtracted := false
	if lang == "" {
		if l := peekLangClass(e); l != "" {
			lang, classExtracted = l, true
		}
	}
	shOpts := c.doc.Opts.SyntaxHighlighterOpts
	if c.rougeEnabled() && !shOpts.BlockDisable {
		hlLang := lang
		if hlLang == "" {
			hlLang = shOpts.DefaultLang
		}
		if inner, langClass, ok := c.rougeHighlight(e.Value, hlLang, shOpts.GuessLang); ok {
			b.WriteString(pad + `<div class="` + langClass + `highlighter-rouge"><div class="highlight"><pre class="highlight"><code>`)
			b.WriteString(inner)
			b.WriteString("</code></pre>\n</div></div>\n")
			return
		}
	}
	preAttr := c.codeblockPreAttr(e, classExtracted, lang)
	codeAttr := ""
	if lang != "" {
		codeAttr = ` class="language-` + escapeHTMLAttr(lang) + `"`
	}
	b.WriteString(pad + "<pre" + preAttr + "><code" + codeAttr + ">")
	b.WriteString(escapeHTMLText(e.Value))
	b.WriteString("</code></pre>\n")
}

// codeblockPreAttr renders a code block's <pre> attributes. When the language was
// consumed from the class attribute (fenced info string absent), that language-<x>
// token is removed from the emitted class — it moves to the <code> element — and an
// emptied class attribute is dropped entirely.
func (c *htmlConverter) codeblockPreAttr(e *Element, stripLang bool, lang string) string {
	if !stripLang {
		return c.attrStr(e)
	}
	var b strings.Builder
	for _, a := range e.Attrs {
		if a.Name == "class" {
			cls := stripLangToken(a.Val, lang)
			if cls == "" {
				continue
			}
			b.WriteString(` class="` + escapeHTMLAttr(cls) + `"`)
			continue
		}
		b.WriteString(" " + a.Name + `="` + escapeHTMLAttr(a.Val) + `"`)
	}
	return b.String()
}

// convertList renders a <ul>/<ol>, eliding the <p> wrapper of a tight item's lone
// paragraph.
func (c *htmlConverter) convertList(e *Element, b *strings.Builder, indent int) {
	pad := ind(indent)
	tag := "ul"
	if e.Type == ElOL {
		tag = "ol"
	}
	b.WriteString(pad + "<" + tag + c.attrStr(e) + ">\n")
	for _, li := range e.Children {
		c.convertLI(li, b, indent+1)
	}
	b.WriteString(pad + "</" + tag + ">\n")
}

// convertLI renders a list item. kramdown marks an item's first paragraph
// "transparent" (rendered inline, without a <p> wrapper) when the item is tight;
// finalizeListItems has already applied that per-item decision.
func (c *htmlConverter) convertLI(li *Element, b *strings.Builder, indent int) {
	pad := ind(indent)
	blocks := contentBlocks(li.Children)
	if len(blocks) == 0 {
		b.WriteString(pad + "<li" + c.attrStr(li) + "></li>\n")
		return
	}
	transparent, _ := li.Options["first_transparent"].(bool)
	if transparent {
		// Lone transparent paragraph: fully inline. With trailing blocks (e.g. a
		// nested list) the first paragraph renders inline then a newline, the rest as
		// blocks — kramdown's "<li>text\n  <ul>…" layout.
		if len(blocks) == 1 {
			b.WriteString(pad + "<li" + c.attrStr(li) + ">")
			b.WriteString(c.renderSpans(blocks[0], indent))
			b.WriteString("</li>\n")
			return
		}
		b.WriteString(pad + "<li" + c.attrStr(li) + ">")
		b.WriteString(c.renderSpans(li.Children[0], indent))
		b.WriteString("\n")
		c.convertChildren(li.Children[1:], b, indent+1)
		b.WriteString(pad + "</li>\n")
		return
	}
	b.WriteString(pad + "<li" + c.attrStr(li) + ">\n")
	c.convertChildren(li.Children, b, indent+1)
	b.WriteString(pad + "</li>\n")
}

// convertDL renders a definition list.
func (c *htmlConverter) convertDL(e *Element, b *strings.Builder, indent int) {
	pad := ind(indent)
	b.WriteString(pad + "<dl" + c.attrStr(e) + ">\n")
	cpad := ind(indent + 1)
	for _, ch := range e.Children {
		switch ch.Type {
		case ElDT:
			raw, _ := ch.Options["raw"].(string)
			b.WriteString(cpad + "<dt" + c.attrStr(ch) + ">" + c.renderRaw(raw, indent+1) + "</dt>\n")
		case ElDD:
			blocks := contentBlocks(ch.Children)
			forceLoose, _ := ch.Options["force_loose"].(bool)
			if len(blocks) == 1 && blocks[0].Type == ElP && !hasBlankSep(ch.Children) && !forceLoose {
				b.WriteString(cpad + "<dd" + c.attrStr(ch) + ">" + c.renderSpans(blocks[0], indent+1) + "</dd>\n")
			} else if len(blocks) == 0 {
				b.WriteString(cpad + "<dd" + c.attrStr(ch) + "></dd>\n")
			} else {
				b.WriteString(cpad + "<dd" + c.attrStr(ch) + ">\n")
				c.convertChildren(ch.Children, b, indent+2)
				b.WriteString(cpad + "</dd>\n")
			}
		}
	}
	b.WriteString(pad + "</dl>\n")
}

// convertTable renders a table with its thead/tbody sections and per-cell
// alignment styles.
func (c *htmlConverter) convertTable(e *Element, b *strings.Builder, indent int) {
	pad := ind(indent)
	b.WriteString(pad + "<table" + c.attrStr(e) + ">\n")
	for _, sec := range e.Children {
		tag := "tbody"
		cell := "td"
		switch sec.Type {
		case ElThead:
			tag = "thead"
			cell = "th"
		case ElTfoot:
			tag = "tfoot"
		}
		b.WriteString(ind(indent+1) + "<" + tag + ">\n")
		for _, tr := range sec.Children {
			b.WriteString(ind(indent+2) + "<tr>\n")
			for _, td := range tr.Children {
				style := ""
				if al, ok := td.Options["align"].(string); ok && al != "" {
					style = ` style="text-align: ` + al + `"`
				}
				raw, _ := td.Options["raw"].(string)
				inner := c.renderRaw(raw, indent+3)
				if inner == "" {
					// kramdown renders an empty cell as a non-breaking space.
					inner = "\u00a0"
				}
				b.WriteString(ind(indent+3) + "<" + cell + style + ">" + inner + "</" + cell + ">\n")
			}
			b.WriteString(ind(indent+2) + "</tr>\n")
		}
		b.WriteString(ind(indent+1) + "</" + tag + ">\n")
	}
	b.WriteString(pad + "</table>\n")
}

// contentBlocks returns e's non-blank children (the renderable blocks).
func contentBlocks(els []*Element) []*Element {
	var out []*Element
	for _, e := range els {
		if e.Type == ElBlank {
			continue
		}
		out = append(out, e)
	}
	return out
}

// hasBlankSep reports whether a list/dd item contains an internal blank separator
// (which forces the loose, <p>-wrapped form).
func hasBlankSep(els []*Element) bool {
	for i, e := range els {
		if e.Type == ElBlank && i > 0 && i < len(els)-1 {
			return true
		}
	}
	return false
}

// attrStr renders an element's HTML attributes in emission order.
func (c *htmlConverter) attrStr(e *Element) string {
	var b strings.Builder
	for _, a := range e.Attrs {
		b.WriteString(" " + a.Name + `="` + escapeHTMLAttr(a.Val) + `"`)
	}
	return b.String()
}

// renderCodespan renders an inline code span. With the Rouge highlighter enabled
// and a resolvable lexer (from a language-<x> class or a guess) it emits the gem's
// <code class="language-<x> highlighter-rouge">…spans…</code>; otherwise it renders
// the plain <code> element, preserving any class attribute unchanged.
func (c *htmlConverter) renderCodespan(e *Element, b *strings.Builder) {
	shOpts := c.doc.Opts.SyntaxHighlighterOpts
	if c.rougeEnabled() && !shOpts.SpanDisable {
		lang := peekLangClass(e)
		if lang == "" {
			lang = shOpts.DefaultLang
		}
		if inner, langClass, ok := c.rougeHighlight(e.Value, lang, shOpts.GuessLang); ok {
			b.WriteString(`<code class="` + langClass + `highlighter-rouge">` + inner + "</code>")
			return
		}
	}
	b.WriteString("<code" + c.attrStr(e) + ">" + escapeHTMLText(e.Value) + "</code>")
}

// renderSpans parses and renders e's raw text into inline HTML.
func (c *htmlConverter) renderSpans(e *Element, indent int) string {
	raw, _ := e.Options["raw"].(string)
	return c.renderRaw(raw, indent)
}

// renderRaw parses raw inline text and renders it to HTML.
func (c *htmlConverter) renderRaw(raw string, indent int) string {
	els := c.doc.parseSpansFor(raw)
	els = c.applyTypography(els)
	var b strings.Builder
	c.renderSpanEls(els, &b, indent)
	return b.String()
}

// parseSpansFor span-parses raw text using the document's harvested definitions.
func (d *Document) parseSpansFor(raw string) []*Element {
	p := d.spanParserState()
	return p.parseSpans(raw)
}

// spanParserState returns the parser bound to the document's harvested
// definitions so span parsing can resolve links/abbreviations/footnotes. New
// always populates it during the block parse.
func (d *Document) spanParserState() *parser {
	return d.parserState
}

// renderSpanEls renders a slice of span elements to HTML.
func (c *htmlConverter) renderSpanEls(els []*Element, b *strings.Builder, indent int) {
	for _, e := range els {
		c.renderSpan(e, b, indent)
	}
}

// renderSpan renders one span element.
func (c *htmlConverter) renderSpan(e *Element, b *strings.Builder, indent int) {
	switch e.Type {
	case ElText:
		b.WriteString(escapeHTMLText(e.Value))
	case ElEm:
		b.WriteString("<em" + c.attrStr(e) + ">")
		c.renderSpanEls(e.Children, b, indent)
		b.WriteString("</em>")
	case ElStrong:
		b.WriteString("<strong" + c.attrStr(e) + ">")
		c.renderSpanEls(e.Children, b, indent)
		b.WriteString("</strong>")
	case ElCodespan:
		c.renderCodespan(e, b)
	case ElA:
		c.renderLink(e, b, indent)
	case ElImg:
		c.renderImage(e, b)
	case ElBr:
		b.WriteString("<br />\n")
	case ElTypographicSym:
		if sub, ok := c.doc.Opts.TypographicSymbols[e.Value]; ok {
			b.WriteString(escapeHTMLText(sub))
		} else {
			b.WriteString(symChar(e.Value))
		}
	case ElRawHTMLSpan:
		b.WriteString(e.Value)
	case ElAbbr:
		c.renderAbbr(e, b)
	case ElFootnoteRef:
		c.renderFootnoteRef(e, b)
	}
}

// renderLink renders an <a>; autolinks escape their href differently (ampersands).
func (c *htmlConverter) renderLink(e *Element, b *strings.Builder, indent int) {
	var ab strings.Builder
	for _, a := range e.Attrs {
		val := a.Val
		if a.Name == "href" {
			val = escapeHref(val)
			ab.WriteString(" href=\"" + val + "\"")
			continue
		}
		ab.WriteString(" " + a.Name + "=\"" + escapeHTMLAttr(val) + "\"")
	}
	b.WriteString("<a" + ab.String() + ">")
	c.renderSpanEls(e.Children, b, indent)
	b.WriteString("</a>")
}

// renderImage renders an <img /> with src/alt/title in kramdown's order.
func (c *htmlConverter) renderImage(e *Element, b *strings.Builder) {
	src, _ := e.getAttr("src")
	alt, _ := e.getAttr("alt")
	b.WriteString(`<img src="` + escapeHref(src) + `" alt="` + escapeHTMLAttr(alt) + `"`)
	if title, ok := e.getAttr("title"); ok {
		b.WriteString(` title="` + escapeHTMLAttr(title) + `"`)
	}
	// any extra IAL attrs
	for _, a := range e.Attrs {
		if a.Name == "src" || a.Name == "alt" || a.Name == "title" {
			continue
		}
		b.WriteString(" " + a.Name + `="` + escapeHTMLAttr(a.Val) + `"`)
	}
	b.WriteString(" />")
}

// renderAbbr renders an <abbr> with its title (and any IAL class).
func (c *htmlConverter) renderAbbr(e *Element, b *strings.Builder) {
	title, _ := e.Options["title"].(string)
	var ab strings.Builder
	for _, a := range e.Attrs {
		ab.WriteString(" " + a.Name + `="` + escapeHTMLAttr(a.Val) + `"`)
	}
	if title != "" {
		ab.WriteString(` title="` + escapeHTMLAttr(title) + `"`)
	}
	b.WriteString("<abbr" + ab.String() + ">" + escapeHTMLText(e.Value) + "</abbr>")
}

// renderFootnoteRef renders a footnote marker, assigning the footnote its number
// on first reference and tracking repeat references for the back-links.
func (c *htmlConverter) renderFootnoteRef(e *Element, b *strings.Builder) {
	id := e.Options["name"].(string)
	num, ok := c.footNums[id]
	if !ok {
		num = c.doc.Opts.FootnoteNr + len(c.footOrder)
		c.footNums[id] = num
		c.footOrder = append(c.footOrder, id)
		if def, ok := c.doc.spanParserState().footDefs[id]; ok {
			c.footDefs[id] = def
		}
	}
	c.footRefs[id]++
	refIdx := c.footRefs[id]
	name := c.doc.Opts.FootnotePrefix + id
	fnref := "fnref:" + name
	if refIdx > 1 {
		fnref = "fnref:" + name + ":" + strconv.Itoa(refIdx-1)
	}
	linkText := strconv.Itoa(num)
	if c.doc.Opts.FootnoteLinkText != "" {
		linkText = strings.ReplaceAll(c.doc.Opts.FootnoteLinkText, "%s", linkText)
	}
	fmt.Fprintf(b, `<sup id="%s"><a href="#fn:%s" class="footnote" rel="footnote" role="doc-noteref">%s</a></sup>`,
		fnref, name, linkText)
}
