// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"fmt"
	"strconv"
	"strings"
)

// appendFootnotes appends the collected footnote definitions as kramdown's
// <div class="footnotes"> … </div> section, with a backlink for each reference.
func (c *htmlConverter) appendFootnotes(body string) string {
	if len(c.footOrder) == 0 {
		return body
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(body, "\n"))
	b.WriteString("\n\n")
	b.WriteString(`<div class="footnotes" role="doc-endnotes">` + "\n")
	if c.doc.Opts.FootnoteNr != 1 {
		b.WriteString(`  <ol start="` + strconv.Itoa(c.doc.Opts.FootnoteNr) + `">` + "\n")
	} else {
		b.WriteString("  <ol>\n")
	}
	// Index loop, not range: rendering a footnote body may reference further
	// footnotes (a footnote inside a footnote), which renderFootnoteRef appends to
	// c.footOrder; those must be emitted too, in first-reference order, exactly as
	// kramdown's footnote-collection pass does.
	for i := 0; i < len(c.footOrder); i++ {
		id := c.footOrder[i]
		def := c.footDefs[id]
		b.WriteString(`    <li id="fn:` + c.doc.Opts.FootnotePrefix + id + `">` + "\n")
		c.renderFootnoteBody(def, id, &b)
		b.WriteString("    </li>\n")
	}
	b.WriteString("  </ol>\n")
	b.WriteString("</div>\n")
	return b.String()
}

// renderFootnoteBody renders a footnote's blocks at the footnote indent (3 levels),
// then places the back-link(s) exactly as kramdown's footnote_content does: into
// the last paragraph or header reached by following the note's final-child chain
// (only when that final block is itself a paragraph unless footnote_backlink_inline
// is set), or, when no such element exists, in a fresh trailing paragraph.
func (c *htmlConverter) renderFootnoteBody(def *Element, id string, b *strings.Builder) {
	blocks := contentBlocks(def.Children)
	var target *Element
	if c.doc.Opts.FootnoteBacklinkInline {
		target = lastParagraphOrHeader(blocks)
	} else if n := len(blocks); n > 0 && blocks[n-1].Type == ElP {
		target = blocks[n-1]
	}
	// insert_space: a back-link appended to existing content is separated from it by
	// a non-breaking space; a back-link in its own new paragraph is not.
	backlink := c.backlinks(id, target != nil)

	var inner strings.Builder
	c.convertChildren(def.Children, &inner, 3)
	body := inner.String()

	switch {
	case backlink == "":
		b.WriteString(body)
	case target != nil:
		b.WriteString(injectFootnoteBacklink(body, backlink))
	default:
		// No inline target: the back-link lives in its own trailing paragraph.
		b.WriteString(body)
		b.WriteString(ind(3) + "<p>" + backlink + "</p>\n")
	}
}

// lastParagraphOrHeader returns the paragraph or header reached by repeatedly
// descending into the final non-blank child of blocks (kramdown's while-loop over
// the last child), or nil when that chain dead-ends before a paragraph/header.
func lastParagraphOrHeader(blocks []*Element) *Element {
	cur := blocks
	for {
		last := lastContentChild(cur)
		if last == nil {
			return nil
		}
		if last.Type == ElP || last.Type == ElHeader {
			return last
		}
		next := footnoteDescendChildren(last)
		if next == nil {
			return nil
		}
		cur = next
	}
}

// lastContentChild returns the final non-blank child of els, or nil when there is
// none.
func lastContentChild(els []*Element) *Element {
	for i := len(els) - 1; i >= 0; i-- {
		if els[i].Type != ElBlank {
			return els[i]
		}
	}
	return nil
}

// footnoteDescendChildren returns the block children to descend into for a
// container element (whose own last child may still be a paragraph/header), or nil
// for a leaf that holds no descendable block children.
func footnoteDescendChildren(el *Element) []*Element {
	switch el.Type {
	case ElBlockquote, ElUL, ElOL, ElLI, ElDL, ElDT, ElDD,
		ElTable, ElThead, ElTbody, ElTfoot, ElTr, ElTd:
		return el.Children
	}
	return nil
}

// footnoteBacklinkTags are the close tags a back-link may be injected before, in
// kramdown's "last paragraph or header" sense.
var footnoteBacklinkTags = []string{
	"</p>", "</h1>", "</h2>", "</h3>", "</h4>", "</h5>", "</h6>",
}

// injectFootnoteBacklink inserts backlink immediately before the last paragraph or
// header close tag in html — the rendered position of the final-child-chain target
// element, which is textually the last such element in the note.
func injectFootnoteBacklink(html, backlink string) string {
	idx := -1
	for _, tag := range footnoteBacklinkTags {
		if p := strings.LastIndex(html, tag); p > idx {
			idx = p
		}
	}
	if idx < 0 {
		return html + backlink
	}
	return html[:idx] + backlink + html[idx:]
}

// backlinks builds the "↩" reverse-footnote links, one per reference, with a
// superscript index for repeats, each separated from the preceding content by a
// non-breaking space. insertSpace controls whether the first link, too, carries the
// leading NBSP (true when it is appended to existing content).
func (c *htmlConverter) backlinks(id string, insertSpace bool) string {
	// An empty footnote_backlink suppresses the reverse-footnote links entirely.
	if c.doc.Opts.FootnoteBacklink == "" {
		return ""
	}
	text := escapeHTMLText(c.doc.Opts.FootnoteBacklink)
	n := c.footRefs[id]
	name := c.doc.Opts.FootnotePrefix + id
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fnref := "fnref:" + name
		if i > 1 {
			fnref = "fnref:" + name + ":" + strconv.Itoa(i-1)
		}
		if i > 1 || insertSpace {
			b.WriteString(" ") // kramdown separates back-links with a real NBSP
		}
		if i == 1 {
			fmt.Fprintf(&b, `<a href="#%s" class="reversefootnote" role="doc-backlink">%s</a>`, fnref, text)
		} else {
			fmt.Fprintf(&b, `<a href="#%s" class="reversefootnote" role="doc-backlink">%s<sup>%d</sup></a>`, fnref, text, i)
		}
	}
	return b.String()
}
