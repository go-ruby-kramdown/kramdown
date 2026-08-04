// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"fmt"
	"strconv"
	"strings"
)

// footnoteContent renders the collected footnote definitions as kramdown's
// <div class="footnotes"> … </div> section (with a backlink for each reference), or
// the empty string when there are no footnotes. The caller either appends it to the
// body or, for a {:footnotes} placement, substitutes it for the sentinel — kramdown
// adds no separator of its own, so any blank line before the block comes from the
// document body itself (footnote_content in the gem).
func (c *htmlConverter) footnoteContent() string {
	if len(c.footOrder) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="footnotes" role="doc-endnotes">` + "\n")
	if c.doc.Opts.FootnoteNr != 1 {
		b.WriteString(`  <ol start="` + strconv.Itoa(c.doc.Opts.FootnoteNr) + `">` + "\n")
	} else {
		b.WriteString("  <ol>\n")
	}
	// Index-based: rendering a footnote body may reference further footnotes
	// (a footnote defined inside another footnote's definition), appending them to
	// footOrder, and those must be emitted too. Re-read len each iteration.
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
// appending the back-link(s) into the target paragraph/header (or a new paragraph
// when there is no such target). kramdown injects the back-link into the last
// paragraph when the note ends in one, or — with footnote_backlink_inline — into
// the deepest last paragraph or header of the note's content.
func (c *htmlConverter) renderFootnoteBody(def *Element, id string, b *strings.Builder) {
	backlink := c.backlinks(id)
	blocks := contentBlocks(def.Children)
	var inner strings.Builder
	// kramdown keeps a leading blank line inside the <li> when the definition's
	// body opened on the line after the marker.
	if lb, _ := def.Options["leading_blank"].(bool); lb {
		inner.WriteString("\n")
	}
	// The back-link target follows kramdown's footnote_content: the note's last
	// block when it is a paragraph, or — with footnote_backlink_inline — the deepest
	// last paragraph/header reached by descending into the final block.
	var target *Element
	if n := len(blocks); n > 0 && (blocks[n-1].Type == ElP || c.doc.Opts.FootnoteBacklinkInline) {
		target = descendLastPHeader(blocks)
	}
	// Render every block at the footnote indent, blank-line-separated.
	var bodyB strings.Builder
	for i, blk := range blocks {
		c.convertBlock(blk, &bodyB, 3)
		if i < len(blocks)-1 {
			bodyB.WriteString("\n")
		}
	}
	switch {
	case backlink == "":
		inner.WriteString(bodyB.String())
	case target != nil:
		// Inject the back-link(s), with their leading NBSP, just before the target
		// element's closing tag (the last </p>/<hN> in the rendered body — the target
		// is the last paragraph/header in document order).
		inner.WriteString(insertBacklinkBeforeClose(bodyB.String(), backlink))
	default:
		// No paragraph/header target: the back-link goes in its own trailing
		// paragraph, without the leading NBSP separator (which takes the entity_output
		// form, so strip that exact rendering).
		sep := entityOut("nbsp", 0x00A0, c.doc.Opts.EntityOutput)
		inner.WriteString(bodyB.String())
		inner.WriteString(ind(3) + "<p>" + strings.TrimPrefix(backlink, sep) + "</p>\n")
	}
	b.WriteString(inner.String())
}

// descendLastPHeader returns the deepest last paragraph or header reachable by
// repeatedly descending into the last content block, mirroring kramdown's descent
// in footnote_content. It returns nil when the descent bottoms out at a block with
// no paragraph/header (e.g. a code block or table).
func descendLastPHeader(blocks []*Element) *Element {
	children := blocks
	for {
		cb := contentBlocks(children)
		if len(cb) == 0 {
			return nil
		}
		last := cb[len(cb)-1]
		if last.Type == ElP || last.Type == ElHeader {
			return last
		}
		children = last.Children
	}
}

// insertBacklinkBeforeClose inserts the back-link markup immediately before the
// closing tag of the last paragraph or header in s (the descent target's element).
func insertBacklinkBeforeClose(s, backlink string) string {
	at := strings.LastIndex(s, "</p>")
	for _, h := range []string{"</h1>", "</h2>", "</h3>", "</h4>", "</h5>", "</h6>"} {
		if i := strings.LastIndex(s, h); i > at {
			at = i
		}
	}
	return s[:at] + backlink + s[at:]
}

// backlinks builds the "↩" reverse-footnote links, one per reference, with a
// superscript index for repeats, each separated by a non-breaking space.
func (c *htmlConverter) backlinks(id string) string {
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
		// kramdown separates back-links with a non-breaking space, rendered per the
		// entity_output mode (the U+00A0 character by default).
		b.WriteString(entityOut("nbsp", 0x00A0, c.doc.Opts.EntityOutput))
		if i == 1 {
			fmt.Fprintf(&b, `<a href="#%s" class="reversefootnote" role="doc-backlink">%s</a>`, fnref, text)
		} else {
			fmt.Fprintf(&b, `<a href="#%s" class="reversefootnote" role="doc-backlink">%s<sup>%d</sup></a>`, fnref, text, i)
		}
	}
	return b.String()
}
