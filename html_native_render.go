// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"strconv"
	"strings"
)

// This file serialises the native tree produced by the :html_to_native transform
// (html_native.go). It is deliberately separate from the markdown-native block
// converter: kramdown's ElementConverter builds elements whose bodies are their
// converted *children*, whereas this port's markdown-native :p / :header / :td render
// from a lazily re-parsed raw string. The renderer here follows kramdown's
// Converter::Html child-based path (inner + format_as_* helpers) so the converted
// subtree comes out byte-for-byte. Every converted node carries Options["hnative"];
// convertBlock and renderSpan hand such nodes here.

// convertNativeBlock renders one block-level node of an html_to_native subtree at the
// given indent level (2 spaces per level), mirroring the matching convert_* method of
// kramdown's HTML converter.
func (c *htmlConverter) convertNativeBlock(e *Element, indent int) string {
	switch e.Type {
	case ElP:
		if t, _ := e.Options["transparent"].(bool); t {
			return c.nativeInner(e, indent)
		}
		return formatAsBlockHTML("p", e.Attrs, c.nativeInner(e, indent), indent)
	case ElHeader:
		return c.convertNativeHeader(e, indent)
	case ElBlockquote:
		return formatAsIndentedBlockHTML("blockquote", e.Attrs, c.nativeInner(e, indent), indent)
	case ElUL:
		return formatAsIndentedBlockHTML("ul", e.Attrs, c.nativeInner(e, indent), indent)
	case ElOL:
		return formatAsIndentedBlockHTML("ol", e.Attrs, c.nativeInner(e, indent), indent)
	case ElDL:
		return formatAsIndentedBlockHTML("dl", e.Attrs, c.nativeInner(e, indent), indent)
	case ElLI:
		return c.convertNativeLI("li", e, indent)
	case ElDD:
		return c.convertNativeLI("dd", e, indent)
	case ElDT:
		return formatAsBlockHTML("dt", e.Attrs, c.nativeInner(e, indent), indent)
	case ElHR:
		return ind(indent) + "<hr" + htmlAttributes(e.Attrs) + " />\n"
	case ElCodeblock:
		// A <pre> converted by convert_pre; its value already carries the chomp+"\n"
		// normalisation, so the shared code-block renderer emits it byte-for-byte
		// (degrading to plain <pre><code> when no language resolves a highlighter).
		var b strings.Builder
		c.convertCodeblock(e, &b, indent)
		return b.String()
	case ElTable:
		return formatAsIndentedBlockHTML("table", e.Attrs, c.nativeInner(e, indent), indent)
	case ElThead:
		return formatAsIndentedBlockHTML("thead", e.Attrs, c.nativeInner(e, indent), indent)
	case ElTbody:
		return formatAsIndentedBlockHTML("tbody", e.Attrs, c.nativeInner(e, indent), indent)
	case ElTfoot:
		return formatAsIndentedBlockHTML("tfoot", e.Attrs, c.nativeInner(e, indent), indent)
	case ElTr:
		return formatAsIndentedBlockHTML("tr", e.Attrs, c.nativeInner(e, indent), indent)
	case ElTd:
		return c.convertNativeTd(e, indent)
	case ElHTMLElement:
		// A raw content-model element (e.g. a non-simple <table> kept raw) serialises
		// through the existing verbatim raw-HTML path, which reproduces its preserved
		// whitespace and nested raw children exactly. A block content-model element (a
		// <div> whose comments/paragraphs were converted) renders child-based.
		if e.Options["content_model"] == cmRaw {
			return c.convertHTMLNode(e, indent, nil)
		}
		return c.convertNativeHTMLElement(e, indent)
	case ElXMLComment, ElXMLPI:
		// Only a block-category comment/PI reaches here (a span one is serialised inline
		// by renderSpan); it takes its own indented line.
		return ind(indent) + e.Value + "\n"
	default:
		return ""
	}
}

// nativeInner ports Converter::Html#inner for the converted tree: each child is
// rendered one indent level deeper, a block child through convertNativeBlock and a
// span child through the shared renderSpan.
func (c *htmlConverter) nativeInner(e *Element, indent int) string {
	var b strings.Builder
	for _, ch := range e.Children {
		if nativeIsBlock(ch) {
			b.WriteString(c.convertNativeBlock(ch, indent+1))
		} else {
			c.renderSpan(ch, &b, indent+1)
		}
	}
	return b.String()
}

// convertNativeHeader ports convert_header for a converted <h1>..<h6>: the id comes
// from an explicit attribute or (with auto_ids) the header's extracted raw text, and
// the body from its converted children.
func (c *htmlConverter) convertNativeHeader(e *Element, indent int) string {
	level := e.Options["level"].(int)
	raw, _ := e.Options["raw_text"].(string)
	if id, ok := e.getAttr("id"); ok {
		c.usedIds[id] = true
	} else if c.doc.Opts.AutoIds {
		if id := c.generateId(raw); id != "" {
			e.setAttr("id", c.doc.Opts.AutoIdPrefix+id)
		}
	}
	tag := "h" + strconv.Itoa(level)
	return formatAsBlockHTML(tag, e.Attrs, c.nativeInner(e, indent), indent)
}

// convertNativeLI ports convert_li / convert_dd: an item whose sole content is a
// transparent paragraph (a wrap_text_children group) renders inline, otherwise its
// block children render on their own indented lines.
func (c *htmlConverter) convertNativeLI(tag string, e *Element, indent int) string {
	res := c.nativeInner(e, indent)
	var b strings.Builder
	b.WriteString(ind(indent) + "<" + tag + htmlAttributes(e.Attrs) + ">")
	firstTransparent := false
	if len(e.Children) > 0 && e.Children[0].Type == ElP {
		firstTransparent, _ = e.Children[0].Options["transparent"].(bool)
	}
	if len(e.Children) == 0 || firstTransparent {
		b.WriteString(res)
		if strings.HasSuffix(res, "\n") {
			b.WriteString(ind(indent))
		}
	} else {
		b.WriteString("\n" + res + ind(indent))
	}
	b.WriteString("</" + tag + ">\n")
	return b.String()
}

// convertNativeTd ports convert_td for a converted simple-table cell: it renders as
// <th> inside a thead section (recorded in celltag) else <td>, applies the column
// alignment as a text-align style (appended after any existing style), and emits a
// non-breaking space for an empty cell.
func (c *htmlConverter) convertNativeTd(e *Element, indent int) string {
	res := c.nativeInner(e, indent)
	// celltag is always recorded by annotateCells for every converted cell.
	tag, _ := e.Options["celltag"].(string)
	attrs := e.Attrs
	if align, _ := e.Options["cellalign"].(string); align != "" && align != "default" {
		attrs = append([]Attr(nil), e.Attrs...)
		found := false
		for i := range attrs {
			if attrs[i].Name == "style" {
				attrs[i].Val += "; text-align: " + align
				found = true
				break
			}
		}
		if !found {
			attrs = append(attrs, Attr{Name: "style", Val: "text-align: " + align})
		}
	}
	if res == "" {
		res = " "
	}
	return formatAsBlockHTML(tag, attrs, res, indent)
}

// convertNativeHTMLElement ports convert_html_element's block-category, non-raw path
// for a node that kept its :html_element form in the converted tree (raw elements are
// routed to the verbatim renderer by the caller). A :block content model puts its body
// on indented lines; a :span content model (a block-category element such as <caption>
// whose native model is span) renders its body inline; an empty body closes with a bare
// tag pair.
func (c *htmlConverter) convertNativeHTMLElement(e *Element, indent int) string {
	res := c.nativeInner(e, indent)
	cm, _ := e.Options["content_model"].(string)
	var b strings.Builder
	b.WriteString(ind(indent) + "<" + e.Value + htmlAttributes(e.Attrs))
	switch {
	case res == "":
		b.WriteString("></" + e.Value + ">")
	case cm != cmBlock:
		b.WriteString(">" + res + "</" + e.Value + ">")
	default:
		b.WriteString(">\n" + strings.TrimSuffix(res, "\n") + "\n" + ind(indent) + "</" + e.Value + ">")
	}
	b.WriteString("\n")
	return b.String()
}

// formatAsBlockHTML ports format_as_block_html: a start/end tag pair with an inline
// body on one indented line.
func formatAsBlockHTML(name string, attrs []Attr, body string, indent int) string {
	return ind(indent) + "<" + name + htmlAttributes(attrs) + ">" + body + "</" + name + ">\n"
}

// formatAsIndentedBlockHTML ports format_as_indented_block_html: a start tag, the body
// on its own lines, then an indented end tag.
func formatAsIndentedBlockHTML(name string, attrs []Attr, body string, indent int) string {
	return ind(indent) + "<" + name + htmlAttributes(attrs) + ">\n" + body + ind(indent) + "</" + name + ">\n"
}
