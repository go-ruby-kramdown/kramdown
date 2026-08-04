// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// tocSentinel is the placeholder a {:toc}-tagged list renders to; convert()
// substitutes the generated table of contents for it once every header has been
// collected, mirroring kramdown's @toc_code mechanism.
const tocSentinel = "\x00kramdown:toc\x00"

// reNoTocClass matches kramdown's in_toc? class test (/\bno_toc\b/): a header whose
// class attribute carries the no_toc word is excluded from the table of contents.
var reNoTocClass = regexp.MustCompile(`\bno_toc\b`)

// inTOC reports whether a header participates in the table of contents: its level is
// among the configured toc_levels and it is not tagged {:.no_toc}. Mirrors
// Converter::Base#in_toc?.
func (c *htmlConverter) inTOC(e *Element, level int) bool {
	if levels := c.doc.Opts.TocLevels; len(levels) > 0 {
		found := false
		for _, l := range levels {
			if l == level {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	cls, _ := e.getAttr("class")
	return !reNoTocClass.MatchString(cls)
}

// tocNode is one entry in the nested table-of-contents tree (a :li in kramdown's
// generated toc tree): its heading level, link target id, rendered link text and any
// deeper entries nested beneath it.
type tocNode struct {
	level    int
	id       string
	raw      string // header source used to render the entry's link text
	text     string
	children []*tocNode
}

// generateTOC renders the collected headers as kramdown's table of contents: an
// id="markdown-toc" list (carrying the tagged list's own tag and attributes) of
// nested links, or the empty string when no header is in scope (matching the gem,
// whose convert_root emits ” for an empty toc tree).
func (c *htmlConverter) generateTOC() string {
	nodes := buildTOCTree(c.tocEntries)
	if len(nodes) == 0 {
		return ""
	}
	// The TOC root carries the tagged list's attributes with id defaulting to
	// "markdown-toc"; that id also prefixes every entry's own id.
	attrs := cloneAttrs(c.tocAttrs)
	idPrefix := "markdown-toc"
	if id, ok := attrValue(attrs, "id"); ok && id != "" {
		idPrefix = id
	} else {
		attrs = setAttrValue(attrs, "id", "markdown-toc")
	}
	for i := range nodes {
		c.renderTOCText(nodes[i])
	}
	var b strings.Builder
	c.serializeTOCList(nodes, 0, attrs, idPrefix, &b)
	return b.String()
}

// buildTOCTree threads the flat header list into the nested tree kramdown builds in
// generate_toc_tree: each header opens a new entry beneath the closest shallower one
// still on the stack, else at the root.
func buildTOCTree(entries []tocHeader) []*tocNode {
	var root, stack []*tocNode
	for _, e := range entries {
		li := &tocNode{level: e.level, id: e.id, raw: e.raw}
		for {
			if len(stack) == 0 {
				root = append(root, li)
				stack = append(stack, li)
				break
			}
			top := stack[len(stack)-1]
			if top.level < li.level {
				top.children = append(top.children, li)
				stack = append(stack, li)
				break
			}
			stack = stack[:len(stack)-1]
		}
	}
	return root
}

// renderTOCText fills in a node's (and its descendants') link text from the header's
// source: kramdown copies the header's children, strips footnote references and
// unwraps links (fix_for_toc_entry), then renders them.
func (c *htmlConverter) renderTOCText(n *tocNode) {
	els := unwrapTOCLinks(stripTOCFootnotes(c.parseSpansRender(n.raw)))
	var sb strings.Builder
	c.renderSpanEls(els, &sb, 0)
	n.text = sb.String()
	for _, ch := range n.children {
		c.renderTOCText(ch)
	}
}

// serializeTOCList renders a nested toc list at the given indent level, mirroring
// kramdown's convert_ul (format_as_indented_block_html) — only the root list (level
// 0) carries attributes.
func (c *htmlConverter) serializeTOCList(nodes []*tocNode, level int, rootAttrs []Attr, idPrefix string, b *strings.Builder) {
	pad := ind(level)
	b.WriteString(pad + "<ul")
	if level == 0 {
		b.WriteString(htmlAttributes(rootAttrs))
	}
	b.WriteString(">\n")
	for _, n := range nodes {
		c.serializeTOCItem(n, level+1, idPrefix, b)
	}
	b.WriteString(pad + "</ul>\n")
}

// serializeTOCItem renders one toc entry, mirroring kramdown's convert_li for a li
// whose first child is a transparent paragraph: the link renders inline, an optional
// nested list follows on the same line, and — when that inner content ends in a
// newline — the closing tag is re-indented.
func (c *htmlConverter) serializeTOCItem(n *tocNode, level int, idPrefix string, b *strings.Builder) {
	pad := ind(level)
	res := `<a href="` + escapeHTMLAttr("#"+n.id) + `" id="` +
		escapeHTMLAttr(idPrefix+"-"+n.id) + `">` + n.text + "</a>"
	if len(n.children) > 0 {
		var sb strings.Builder
		c.serializeTOCList(n.children, level+1, nil, idPrefix, &sb)
		res += sb.String()
	}
	b.WriteString(pad + "<li>" + res)
	if strings.HasSuffix(res, "\n") {
		b.WriteString(pad)
	}
	b.WriteString("</li>\n")
}

// stripTOCFootnotes removes footnote references from a span tree (kramdown's
// remove_footnotes), recursing into children first.
func stripTOCFootnotes(els []*Element) []*Element {
	out := els[:0:0]
	for _, e := range els {
		if e.Type == ElFootnoteRef {
			continue
		}
		e.Children = stripTOCFootnotes(e.Children)
		out = append(out, e)
	}
	return out
}

// unwrapTOCLinks replaces every link with its (already unwrapped) children, hoisting
// their content into the parent (kramdown's unwrap_links), recursing first.
func unwrapTOCLinks(els []*Element) []*Element {
	var out []*Element
	for _, e := range els {
		e.Children = unwrapTOCLinks(e.Children)
		if e.Type == ElA {
			out = append(out, e.Children...)
		} else {
			out = append(out, e)
		}
	}
	return out
}

// cloneAttrs returns a copy of the attribute slice so the TOC root can default its id
// without mutating the tagged list's own attributes.
func cloneAttrs(attrs []Attr) []Attr {
	return append([]Attr(nil), attrs...)
}

// attrValue returns the value of the named attribute and whether it is present.
func attrValue(attrs []Attr, name string) (string, bool) {
	for _, a := range attrs {
		if a.Name == name {
			return a.Val, true
		}
	}
	return "", false
}

// setAttrValue appends the named attribute (the caller only calls it when the name is
// absent, matching kramdown's id ||= default).
func setAttrValue(attrs []Attr, name, val string) []Attr {
	return append(attrs, Attr{Name: name, Val: val})
}
