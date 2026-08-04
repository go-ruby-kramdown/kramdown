// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"
)

// This file ports kramdown's Kramdown::Parser::Kramdown#parse_span_html: the
// span-level HTML front-end that turns inline raw HTML into native :html_element /
// :xml_comment nodes (category span). It is the keystone that graduates the
// span/05_html corpus, the code/em pipes of the table cases and the inline comment /
// CDATA cases: a span start tag becomes an :html_element whose body is parsed either
// as span-level Markdown (its native content model, when parse_span_html is on) or,
// for raw elements, as raw text that still recognises nested span HTML; a stray close
// tag and a block-level tag used in span context are emitted as escaped text.

var (
	// reCDATASpan captures the content of an inline "<![CDATA[…]]>" section.
	reCDATASpan = regexp.MustCompile(`(?s)^<!\[CDATA\[(.*?)\]\]>`)
	// reNewlineRun collapses newline runs in an attribute value to a single space,
	// mirroring parse_span_html's attrs.each_value { value.gsub!(/\n+/, ' ') }.
	reNewlineRun = regexp.MustCompile(`\n+`)
)

// trySpanHTML ports parse_span_html at the current "<": it recognises an HTML
// comment, a CDATA section, a stray close tag, or a start tag, appending the result
// to the span tree (an element via flush + *sp.dst, or escaped text via lit) and
// advancing sp.pos. It returns true when it consumed the construct, or false to leave
// the "<" as a literal character for the caller. It is the only span parser active in
// a raw-content-model body.
func (sp *spanParser) trySpanHTML(lit *strings.Builder, flush func()) bool {
	s := sp.src[sp.pos:]

	// A comment becomes a span-category :xml_comment, rendered verbatim inline.
	if m, ok := matchAnchored(reHTMLComment, s); ok {
		flush()
		el := newEl(ElXMLComment)
		el.Value = m
		el.Options["category"] = cmSpan
		*sp.dst = append(*sp.dst, el)
		sp.pos += len(m)
		return true
	}

	// A CDATA section contributes its (later HTML-escaped) content as text.
	if m := reCDATASpan.FindStringSubmatch(s); m != nil {
		lit.WriteString(m[1])
		sp.pos += len(m[0])
		return true
	}

	// A close tag reaching here has no matching open (a matching one is consumed by
	// the body's stop_re): kramdown warns and emits it as escaped text.
	if m := reHTMLTagCloseAnchor.FindStringSubmatch(s); m != nil {
		sp.p.warn("Found invalidly used HTML closing tag for '" + m[1] + "'")
		lit.WriteString(m[0])
		sp.pos += len(m[0])
		return true
	}

	// A start tag becomes a span-category :html_element.
	name, attrsStr, selfClose, n, ok := matchStartTag(s)
	if !ok {
		return false // not a span-HTML construct: the "<" is literal
	}
	tagName := name
	if htmlElementNames[strings.ToLower(name)] {
		tagName = strings.ToLower(name)
	}
	// A block-level element used in span context is invalid: emit it as escaped text.
	if htmlBlockElements[tagName] {
		sp.p.warn("Found block HTML tag '" + tagName + "' in span-level text")
		lit.WriteString(s[:n])
		sp.pos += n
		return true
	}

	attrs := parseHTMLAttributes(attrsStr, htmlElementNames[tagName])
	for i := range attrs {
		if attrs[i].Val != "" {
			attrs[i].Val = reNewlineRun.ReplaceAllString(attrs[i].Val, " ")
		}
	}

	el := newEl(ElHTMLElement)
	el.Value = tagName
	el.Attrs = attrs

	// do_parsing decides whether the body is span-level Markdown (:span) or raw text
	// recognising only nested span HTML (:raw): a raw content model or a raw enclosing
	// element forces raw; otherwise the parse_span_html option governs. A markdown="…"
	// attribute overrides this (block being rejected with a warning).
	doParsing := contentModelFor(tagName) != cmRaw && !sp.rawMode && sp.p.opts.ParseSpanHTML
	if mv, hadMarkdown := el.deleteAttr("markdown"); hadMarkdown {
		switch mv {
		case "block":
			sp.p.warn("Cannot use block-level parsing in span-level HTML tag - using default mode")
		case "span":
			doParsing = true
		case "1":
			doParsing = contentModelFor(tagName) != cmRaw
		case "0":
			doParsing = false
		}
	}

	el.Options["category"] = cmSpan
	if doParsing {
		el.Options["content_model"] = cmSpan
	} else {
		el.Options["content_model"] = cmRaw
	}
	el.Options["is_closed"] = selfClose

	flush()
	*sp.dst = append(*sp.dst, el)
	sp.pos += n

	if !selfClose && !htmlElementsWithoutBody[tagName] {
		stopRE := spanHTMLCloseRE(tagName)
		if sp.parseHTMLBody(el, stopRE, !doParsing) {
			if loc := stopRE.FindStringIndex(sp.src[sp.pos:]); loc != nil {
				sp.pos += loc[1]
			}
		} else {
			sp.p.warn("Found no end tag for '" + tagName + "' - auto-closing it")
			sp.pos = len(sp.src)
		}
	}
	return true
}

// parseHTMLBody parses el's span-HTML body into el.Children with stopRE as the
// terminating close tag and raw selecting the raw content model (only nested span
// HTML recognised). It returns whether the close tag was found (false = auto-closed
// at end of input). The enclosing stop / raw context is saved and restored so nesting
// composes correctly.
func (sp *spanParser) parseHTMLBody(el *Element, stopRE *regexp.Regexp, raw bool) bool {
	savedDst, savedStop, savedRaw := sp.dst, sp.htmlStopRE, sp.rawMode
	sp.dst = &el.Children
	sp.htmlStopRE = stopRE
	sp.rawMode = raw
	found := sp.parseInto(nil)
	sp.dst, sp.htmlStopRE, sp.rawMode = savedDst, savedStop, savedRaw
	return found
}

// spanHTMLCloseRE builds the anchored close-tag regexp for a span-HTML element:
// "</name\s*>", case-insensitive when name is a known HTML element (kramdown applies
// /i via HTML_ELEMENT[name]) and case-sensitive for an arbitrary XML name.
func spanHTMLCloseRE(name string) *regexp.Regexp {
	pat := `^</` + regexp.QuoteMeta(name) + `\s*>`
	if htmlElementNames[strings.ToLower(name)] {
		pat = `(?i)` + pat
	}
	return regexp.MustCompile(pat)
}

// matchAnchored returns the leading match of an anchored regexp at the start of s and
// whether it matched there.
func matchAnchored(re *regexp.Regexp, s string) (string, bool) {
	loc := re.FindStringIndex(s)
	if loc == nil || loc[0] != 0 {
		return "", false
	}
	return s[:loc[1]], true
}
