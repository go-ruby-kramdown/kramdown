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
		// A script/style body captured by handle_raw_html_tag is emitted verbatim.
		if v, _ := e.Options["verbatim"].(bool); v {
			return e.Value
		}
		return escapeHTMLText(e.Value)
	}
}

// htmlInner converts the raw-content children of e (kramdown's inner): each child is
// dispatched one indentation level deeper with e as its parent. Used for the raw
// content model, whose children are text / nested HTML nodes.
func (c *htmlConverter) htmlInner(e *Element, indent int) string {
	var b strings.Builder
	ci := indent + 1
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

// convertHTMLElement ports convert_html_element's block-category path across the three
// content models. The body is rendered per model: :block reparses to nested Markdown
// blocks (indented, on their own lines between the tags), :span span-parses its stored
// raw text inline, and :raw serialises its text / nested-HTML children verbatim.
// Indentation and the trailing newline are suppressed when the parent is itself raw
// HTML, exactly as kramdown suppresses them for @stack.last being a raw element.
func (c *htmlConverter) convertHTMLElement(e *Element, indent int, parent *Element) string {
	cm, _ := e.Options["content_model"].(string)
	var res string
	switch cm {
	case cmBlock:
		var sb strings.Builder
		c.convertChildren(e.Children, &sb, indent+1)
		res = sb.String()
	case cmSpan:
		raw, _ := e.Options["raw"].(string)
		res = c.renderRaw(raw, indent+1)
	default:
		res = c.htmlInner(e, indent)
	}
	attrs := htmlAttributes(e.Attrs)
	isClosed, _ := e.Options["is_closed"].(bool)
	raw := parentIsRawHTML(parent)
	var b strings.Builder
	if !raw {
		b.WriteString(ind(indent))
	}
	b.WriteString("<" + e.Value + attrs)
	switch {
	case isClosed && cm == cmRaw:
		b.WriteString(" />")
	case res != "" && cm != cmBlock:
		b.WriteString(">" + res + "</" + e.Value + ">")
	case res != "":
		b.WriteString(">\n" + strings.TrimSuffix(res, "\n") + "\n" + ind(indent) + "</" + e.Value + ">")
	default:
		// An empty body: a void element in the raw model already matched the self-closed
		// case above, so every element reaching here is a non-void element with no
		// content, rendered as an empty tag pair.
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
