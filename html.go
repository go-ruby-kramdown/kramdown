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

// convertChildren renders a sequence of block elements at the given indent level,
// emitting a blank line between two rendered blocks exactly where the source had a
// blank-line separator (an ElBlank node).
func (c *htmlConverter) convertChildren(els []*Element, b *strings.Builder, indent int) {
	rendered := false
	pendingBlank := false
	for _, e := range els {
		if e.Type == ElBlank {
			if rendered {
				pendingBlank = true
			}
			continue
		}
		if rendered && pendingBlank {
			b.WriteByte('\n')
		}
		pendingBlank = false
		rendered = true
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
		b.WriteString(pad + "<p" + c.attrStr(e) + ">")
		b.WriteString(c.renderSpans(e, indent))
		b.WriteString("</p>\n")
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
	}
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

// convertCodeblock renders a code block, attaching a language class when set.
func (c *htmlConverter) convertCodeblock(e *Element, b *strings.Builder, indent int) {
	pad := ind(indent)
	preAttr := c.attrStr(e)
	codeAttr := ""
	if lang, ok := e.Options["lang"].(string); ok && lang != "" {
		codeAttr = ` class="language-` + escapeHTMLAttr(lang) + `"`
	}
	b.WriteString(pad + "<pre" + preAttr + "><code" + codeAttr + ">")
	b.WriteString(escapeHTMLText(e.Value))
	b.WriteString("</code></pre>\n")
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
	// Looseness is a list-wide property in kramdown: if any item is separated by a
	// blank line, every item renders in the loose (<p>-wrapped) form.
	tight, _ := e.Options["tight"].(bool)
	for _, li := range e.Children {
		if !tight {
			li.Options["force_loose"] = true
		}
		c.convertLI(li, b, indent+1)
	}
	b.WriteString(pad + "</" + tag + ">\n")
}

// convertLI renders a list item, choosing the tight (inline) or loose (block) form
// based on its content.
func (c *htmlConverter) convertLI(li *Element, b *strings.Builder, indent int) {
	pad := ind(indent)
	blocks := contentBlocks(li.Children)
	forceLoose, _ := li.Options["force_loose"].(bool)
	tight := !hasBlankSep(li.Children) && !forceLoose
	// Tight item: a single paragraph, no internal blank separators, not forced
	// loose -> inline text without a <p> wrapper.
	if len(blocks) == 1 && blocks[0].Type == ElP && tight {
		b.WriteString(pad + "<li" + c.attrStr(li) + ">")
		b.WriteString(c.renderSpans(blocks[0], indent))
		b.WriteString("</li>\n")
		return
	}
	if len(blocks) == 0 {
		b.WriteString(pad + "<li" + c.attrStr(li) + "></li>\n")
		return
	}
	// Tight item whose first block is a paragraph followed by further blocks (e.g. a
	// nested list): the leading paragraph renders inline (no <p>), the rest as
	// blocks. Match kramdown's "<li>text\n  <ul>…" layout.
	if tight && blocks[0].Type == ElP {
		b.WriteString(pad + "<li" + c.attrStr(li) + ">")
		b.WriteString(c.renderSpans(blocks[0], indent))
		b.WriteString("\n")
		c.convertChildren(blocks[1:], b, indent+1)
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
		if sec.Type == ElThead {
			tag = "thead"
			cell = "th"
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
				b.WriteString(ind(indent+3) + "<" + cell + style + ">" + c.renderRaw(raw, indent+3) + "</" + cell + ">\n")
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
		b.WriteString("<code" + c.attrStr(e) + ">" + escapeHTMLText(e.Value) + "</code>")
	case ElA:
		c.renderLink(e, b, indent)
	case ElImg:
		c.renderImage(e, b)
	case ElBr:
		b.WriteString("<br />\n")
	case ElTypographicSym:
		b.WriteString(symChar(e.Value))
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
	fnref := "fnref:" + id
	if refIdx > 1 {
		fnref = "fnref:" + id + ":" + strconv.Itoa(refIdx-1)
	}
	fmt.Fprintf(b, `<sup id="%s"><a href="#fn:%s" class="footnote" rel="footnote" role="doc-noteref">%d</a></sup>`,
		fnref, id, num)
}
