// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "strings"

// This file ports the HTML converter's raw-HTML output path: convert_html_element,
// convert_xml_comment / convert_xml_pi and the surrounding indentation rules, which
// serialise the native tree kramdown's Parser::Html produces. The serialisation must
// match kramdown byte-for-byte (its whitespace/indentation choices differ from a
// generic HTML re-serialiser).

// convertHTMLNode dispatches one node of a raw-HTML subtree, returning its serialised
// HTML. parent is the element whose inner content is being rendered (nil at the
// document root), used to decide indentation exactly as kramdown's @stack.last does.
func (c *htmlConverter) convertHTMLNode(e *Element, indent int, parent *Element) string {
	switch e.Type {
	case ElXMLComment, ElXMLPI:
		return c.convertXMLComment(e, indent, parent)
	case ElHTMLElement:
		return c.convertHTMLElement(e, indent, parent)
	default: // ElText
		return escapeHTMLText(e.Value)
	}
}

// htmlInner converts the children of e (kramdown's inner): the indent is increased by
// the fixed two-space step and each child is dispatched with e as its parent.
func (c *htmlConverter) htmlInner(e *Element, indent int) string {
	var b strings.Builder
	ci := indent + 2
	for _, ch := range e.Children {
		b.WriteString(c.convertHTMLNode(ch, ci, e))
	}
	return b.String()
}

// parentIsRawHTML reports whether parent is a raw-content-model HTML element, in which
// case kramdown suppresses the indentation and trailing newline of a block child.
func parentIsRawHTML(parent *Element) bool {
	return parent != nil && parent.Type == ElHTMLElement &&
		parent.Options["content_model"] == cmRaw
}

// convertHTMLElement ports convert_html_element's block, raw-content-model path: a
// void element serialises self-closed, an element with body renders it inline between
// the tags, and an empty non-void element renders as an empty tag pair. Indentation
// and the trailing newline are suppressed when the parent is itself raw HTML. (The
// block/span content-model shapes and the :span category arrive with those content
// models in a later cluster.)
func (c *htmlConverter) convertHTMLElement(e *Element, indent int, parent *Element) string {
	res := c.htmlInner(e, indent)
	attrs := htmlAttributes(e.Attrs)
	var b strings.Builder
	raw := parentIsRawHTML(parent)
	if !raw {
		b.WriteString(ind(indent))
	}
	b.WriteString("<" + e.Value + attrs)
	isClosed, _ := e.Options["is_closed"].(bool)
	switch {
	case isClosed:
		b.WriteString(" />")
	case res != "":
		b.WriteString(">" + res + "</" + e.Value + ">")
	default:
		b.WriteString("></" + e.Value + ">")
	}
	if !raw {
		b.WriteString("\n")
	}
	return b.String()
}

// convertXMLComment ports convert_xml_comment / convert_xml_pi: a block-category
// comment/PI whose parent is not raw HTML is emitted on its own indented line;
// otherwise it is emitted verbatim inline.
func (c *htmlConverter) convertXMLComment(e *Element, indent int, parent *Element) string {
	if e.Options["category"] == cmBlock && !parentIsRawHTML(parent) {
		return ind(indent) + e.Value + "\n"
	}
	return e.Value
}

// htmlAttributes ports Utils::Html#html_attributes: attributes render in order as
// ` name="value"`, an id whose value is blank is dropped, and every value is escaped
// with the :attribute rules.
func htmlAttributes(attrs []Attr) string {
	var b strings.Builder
	for _, a := range attrs {
		if a.Name == "id" && strings.TrimSpace(a.Val) == "" {
			continue
		}
		b.WriteString(" " + a.Name + `="` + escapeHTMLAttr(a.Val) + `"`)
	}
	return b.String()
}
