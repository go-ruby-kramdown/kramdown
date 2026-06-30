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
	b.WriteString("  <ol>\n")
	for _, id := range c.footOrder {
		def := c.footDefs[id]
		b.WriteString(`    <li id="fn:` + id + `">` + "\n")
		c.renderFootnoteBody(def, id, &b)
		b.WriteString("    </li>\n")
	}
	b.WriteString("  </ol>\n")
	b.WriteString("</div>\n")
	return b.String()
}

// renderFootnoteBody renders a footnote's blocks at the footnote indent (3 levels),
// appending the back-link(s) into the final paragraph (or a new paragraph when the
// last block is not a paragraph).
func (c *htmlConverter) renderFootnoteBody(def *Element, id string, b *strings.Builder) {
	backlink := c.backlinks(id)
	blocks := contentBlocks(def.Children)
	var inner strings.Builder
	// kramdown emits a leading blank line inside the <li> when the first block is
	// not a simple paragraph (a code block, a quote, or an empty note).
	if len(blocks) == 0 || blocks[0].Type != ElP {
		inner.WriteString("\n")
	}
	lastIsP := len(blocks) > 0 && blocks[len(blocks)-1].Type == ElP
	for i, blk := range blocks {
		if i == len(blocks)-1 && lastIsP {
			// Append the backlink (with its leading NBSP) inside the last paragraph.
			inner.WriteString(ind(3) + "<p" + c.attrStr(blk) + ">")
			inner.WriteString(c.renderSpans(blk, 3))
			inner.WriteString(backlink)
			inner.WriteString("</p>\n")
			continue
		}
		c.convertBlock(blk, &inner, 3)
		if i < len(blocks)-1 {
			inner.WriteString("\n")
		}
	}
	if !lastIsP {
		// Backlink in its own trailing paragraph (no leading NBSP).
		inner.WriteString(ind(3) + "<p>" + strings.TrimPrefix(backlink, " ") + "</p>\n")
	}
	b.WriteString(inner.String())
}

// backlinks builds the "↩" reverse-footnote links, one per reference, with a
// superscript index for repeats, each separated by a non-breaking space.
func (c *htmlConverter) backlinks(id string) string {
	n := c.footRefs[id]
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fnref := "fnref:" + id
		if i > 1 {
			fnref = "fnref:" + id + ":" + strconv.Itoa(i-1)
		}
		b.WriteString("\u00a0") // kramdown separates back-links with a real NBSP
		if i == 1 {
			fmt.Fprintf(&b, `<a href="#%s" class="reversefootnote" role="doc-backlink">&#8617;</a>`, fnref)
		} else {
			fmt.Fprintf(&b, `<a href="#%s" class="reversefootnote" role="doc-backlink">&#8617;<sup>%d</sup></a>`, fnref, i)
		}
	}
	return b.String()
}
