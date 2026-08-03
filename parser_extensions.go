// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strconv"
	"strings"
)

// This file ports the block-level generic extensions of
// parser/kramdown/extensions.rb: {::nomarkdown}…{:/nomarkdown} (raw passthrough,
// optionally filtered by target format) and {::options key="val" /} (inline option
// setting). An extension whose name is unknown, or a known one with no stop tag, is
// reverted to literal paragraph text exactly as the gem does.

var (
	// reExtStart matches a block extension start tag on its own line: "{::name
	// attrs /}" (self-closing) or "{::name attrs}".
	reExtStart = regexp.MustCompile(`^ {0,3}\{::(\w+)(?:[ \t]((?:\\\}|[^}])*?))?(/)?\}[ \t]*$`)
	// reExtStopName matches a stop tag "{:/name}" or "{:/}" on its own line.
	reExtStopName = regexp.MustCompile(`^ {0,3}\{:/(\w*)\}[ \t]*$`)
	// reExtAttrPair matches a key="val" / key='val' attribute pair (Go's RE2 has no
	// backreferences, so the two quote styles are separate alternatives).
	reExtAttrPair = regexp.MustCompile(`(\w+)=(?:"([^"]*)"|'([^']*)')`)
)

// parseBlockExtension handles a "{::…}" block-extension start tag at lines[start].
// It returns the number of consumed lines and whether it handled the block; a false
// return leaves the line to the ordinary block parsers (so an unknown or
// unterminated extension degrades to literal paragraph text).
func (p *parser) parseBlockExtension(lines []string, start int, parent *Element) (int, bool) {
	m := reExtStart.FindStringSubmatch(lines[start])
	if m == nil {
		return 0, false
	}
	name, attrRaw, selfClose := m[1], m[2], m[3] == "/"
	switch name {
	case "nomarkdown", "options":
	default:
		// comment is handled by parseComment; any other name is unknown and falls
		// through to become literal paragraph text.
		return 0, false
	}
	// Locate the body and stop tag (a self-closing tag has neither).
	body, stopIdx := "", start
	if !selfClose {
		stop := -1
		for k := start + 1; k < len(lines); k++ {
			if sm := reExtStopName.FindStringSubmatch(lines[k]); sm != nil && (sm[1] == "" || sm[1] == name) {
				stop = k
				break
			}
		}
		if stop < 0 {
			return 0, false // no stop tag: literal paragraph
		}
		body = strings.Join(lines[start+1:stop], "\n")
		stopIdx = stop
	}
	consumed := stopIdx - start + 1
	switch name {
	case "nomarkdown":
		if !selfClose {
			raw := newEl(ElRaw)
			raw.Value = body
			raw.Options["types"] = extAttrType(attrRaw)
			parent.addChild(raw)
		}
	case "options":
		p.applyInlineOptions(attrRaw)
		// Emit nothing; the following blank line supplies any separator.
	}
	return consumed, true
}

// rawForHTML reports whether a {::nomarkdown} block with the given target-format
// filter should render for the HTML converter: an empty filter matches all
// formats, otherwise it must name "html".
func rawForHTML(types []string) bool {
	if len(types) == 0 {
		return true
	}
	for _, t := range types {
		if t == "html" {
			return true
		}
	}
	return false
}

// extAttrType returns the space-split "type" attribute of an extension tag ([] when
// absent), naming the target formats a {::nomarkdown} block is restricted to.
func extAttrType(attrRaw string) []string {
	for _, m := range reExtAttrPair.FindAllStringSubmatch(attrRaw, -1) {
		if m[1] == "type" {
			return strings.Fields(attrPairValue(m))
		}
	}
	return nil
}

// attrPairValue returns the captured value of a reExtAttrPair match, taking
// whichever of the double- or single-quoted alternatives matched.
func attrPairValue(m []string) string {
	if m[2] != "" {
		return m[2]
	}
	return m[3]
}

// applyInlineOptions folds a {::options} tag's recognised key="val" pairs into the
// parser's options (mirroring kramdown's handle_extension 'options'). Unknown keys
// and values that fail to parse are ignored.
func (p *parser) applyInlineOptions(attrRaw string) {
	for _, m := range reExtAttrPair.FindAllStringSubmatch(attrRaw, -1) {
		key, val := m[1], attrPairValue(m)
		switch key {
		case "footnote_nr":
			if n, err := strconv.Atoi(val); err == nil {
				p.opts.FootnoteNr = n
			}
		case "auto_ids":
			p.opts.AutoIds = val == "true"
		case "auto_id_prefix":
			p.opts.AutoIdPrefix = val
		case "hard_wrap":
			p.opts.HardWrap = val == "true"
		case "footnote_prefix":
			p.opts.FootnotePrefix = val
		case "syntax_highlighter":
			p.opts.SyntaxHighlighter = normalizeHighlighter(val)
		case "syntax_highlighter_opts":
			applyHighlighterOpts(&p.opts.SyntaxHighlighterOpts, val)
		}
	}
}

// reSHDefaultLang / reSHGuessLang read the two syntax_highlighter_opts keys this
// port honours out of an inline {::options} Ruby-hash literal such as
// "{default_lang: ruby, guess_lang: true}".
var (
	reSHDefaultLang = regexp.MustCompile(`default_lang:\s*(\w+)`)
	reSHGuessLang   = regexp.MustCompile(`guess_lang:\s*(true|false)`)
)

// applyHighlighterOpts folds an inline syntax_highlighter_opts hash literal into o.
func applyHighlighterOpts(o *SyntaxHighlighterOpts, val string) {
	if m := reSHDefaultLang.FindStringSubmatch(val); m != nil {
		o.DefaultLang = m[1]
	}
	if m := reSHGuessLang.FindStringSubmatch(val); m != nil {
		o.GuessLang = m[1] == "true"
	}
}

// normalizeHighlighter canonicalises a syntax_highlighter option value: Ruby's nil
// spellings collapse to "" (highlighting off), everything else is lower-cased so
// ":rouge"/"rouge" both select Rouge.
func normalizeHighlighter(val string) string {
	switch strings.ToLower(strings.TrimPrefix(val, ":")) {
	case "null", "nil", "~", "":
		return ""
	default:
		return strings.ToLower(strings.TrimPrefix(val, ":"))
	}
}
