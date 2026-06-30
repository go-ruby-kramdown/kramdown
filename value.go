// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package kramdown is a pure-Go (CGO-free) reimplementation of Ruby's kramdown
// Markdown-to-HTML converter — the parser and HTML renderer that back
// Kramdown::Document.new(src, options).to_html. It parses the kramdown dialect (a
// superset of Markdown: ATX/Setext headers with inline-attribute lists,
// blockquotes, fenced and indented code, ordered/unordered/definition lists,
// tables with alignment, footnotes, abbreviations, smart-quote typography, block
// and span IALs/ALDs, the {::comment} extension, …) into an element tree and
// renders the gem's HTML byte-for-byte on the common feature set — with no Ruby
// runtime.
//
// The value model is deliberately small: a source string in, an HTML string out,
// plus an options hash. The intermediate element tree ([Element]) mirrors
// kramdown's own AST (a type, a value, attributes and children) so a host (such
// as go-embedded-ruby) can bind Kramdown::Document / Kramdown::Element directly
// onto it.
package kramdown

// ElementType enumerates the kinds of node in the kramdown element tree. The set
// mirrors the subset of Kramdown::Element types this converter produces.
type ElementType int

const (
	// ElRoot is the document root; its Children are the top-level blocks.
	ElRoot ElementType = iota
	// ElBlank is a run of one or more blank lines between blocks.
	ElBlank
	// ElP is a paragraph; its Children are span elements.
	ElP
	// ElHeader is an ATX or Setext header; Value is unused, Options["level"] is the
	// level (1..6) and Options["raw_text"] the source used for auto-ids.
	ElHeader
	// ElBlockquote is a blockquote; Children are nested blocks.
	ElBlockquote
	// ElCodeblock is a fenced or indented code block; Value holds the literal text.
	ElCodeblock
	// ElHR is a horizontal rule.
	ElHR
	// ElUL / ElOL are unordered / ordered lists; Children are ElLI.
	ElUL
	// ElOL is an ordered list.
	ElOL
	// ElLI is a list item; Children are nested blocks (or a single bare paragraph
	// whose <p> wrapper is elided when the item is "tight").
	ElLI
	// ElDL is a definition list; Children are ElDT / ElDD.
	ElDL
	// ElDT is a definition term.
	ElDT
	// ElDD is a definition description.
	ElDD
	// ElTable is a table; Children are ElThead / ElTbody.
	ElTable
	// ElThead / ElTbody / ElTr / ElTd structure a table.
	ElThead
	// ElTbody is a table body.
	ElTbody
	// ElTr is a table row.
	ElTr
	// ElTd is a table cell (a <td> or, in a thead, a <th>).
	ElTd
	// ElHTMLBlock is a passthrough block of raw HTML; Value holds it verbatim.
	ElHTMLBlock
	// ElComment is a {::comment} extension block; Value holds the comment text.
	ElComment
	// ElFootnoteDef collects a footnote definition's blocks (never rendered inline).
	ElFootnoteDef

	// --- span elements ---

	// ElText is literal text; Value holds it.
	ElText
	// ElEm / ElStrong are emphasis / strong emphasis.
	ElEm
	// ElStrong is strong emphasis.
	ElStrong
	// ElCodespan is an inline code span; Value holds the literal text.
	ElCodespan
	// ElA is a hyperlink; Options["href"]/["title"] carry the destination.
	ElA
	// ElImg is an image; Options["src"]/["alt"]/["title"] carry the attributes.
	ElImg
	// ElBr is a hard line break.
	ElBr
	// ElTypographicSym carries a smart-typography substitution; Value is the entity
	// name (e.g. "ldquo", "mdash").
	ElTypographicSym
	// ElFootnoteRef is a footnote reference; Options["name"] is the id.
	ElFootnoteRef
	// ElAbbr is an expanded abbreviation; Value is the matched text and
	// Options["title"]/["class"] carry the definition.
	ElAbbr
	// ElRawHTMLSpan is raw inline HTML passed through verbatim in Value.
	ElRawHTMLSpan
)

// Attr is one HTML attribute (name/value), kept ordered as kramdown emits them.
type Attr struct {
	Name string
	Val  string
}

// Element is a node in the kramdown element tree. Type selects the node kind,
// Value carries literal text for leaf nodes, Children holds nested elements, Attrs
// holds rendered HTML attributes in emission order, and Options carries
// parser-internal metadata (header level, list tightness, table alignments, …).
type Element struct {
	Type     ElementType
	Value    string
	Children []*Element
	Attrs    []Attr
	Options  map[string]any
}

// newEl builds an Element of the given type with an initialised Options map.
func newEl(t ElementType) *Element {
	return &Element{Type: t, Options: map[string]any{}}
}

// setAttr sets attribute name to val, replacing any existing one of that name
// (preserving its position) or appending it in encounter order otherwise.
func (e *Element) setAttr(name, val string) {
	for i := range e.Attrs {
		if e.Attrs[i].Name == name {
			e.Attrs[i].Val = val
			return
		}
	}
	e.Attrs = append(e.Attrs, Attr{Name: name, Val: val})
}

// getAttr returns the value of attribute name and whether it is present.
func (e *Element) getAttr(name string) (string, bool) {
	for i := range e.Attrs {
		if e.Attrs[i].Name == name {
			return e.Attrs[i].Val, true
		}
	}
	return "", false
}

// addChild appends c to e's children.
func (e *Element) addChild(c *Element) { e.Children = append(e.Children, c) }
