// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

// Options configures a conversion, mirroring the keyword options accepted by
// Kramdown::Document.new. Only the options that influence the HTML output of the
// supported feature set are honoured; the rest are tolerated for API parity.
type Options struct {
	// AutoIds, when true (kramdown's default), assigns a generated id="" to every
	// header that lacks an explicit {#id}.
	AutoIds bool
	// AutoIdPrefix is prepended to every auto-generated header id (default "").
	AutoIdPrefix string
	// SmartQuotes enables typographic substitution of quotes/dashes/ellipses
	// (kramdown's default).
	SmartQuotes bool
	// Typographic enables the --, ---, ... and <<>> substitutions (default true).
	Typographic bool
	// HardWrap, when true, turns every soft newline into a <br />. Independent of
	// this, a line ending in two spaces (or "\\") is always a hard break. kramdown's
	// default is false.
	HardWrap bool
	// FootnoteNr is the starting number for footnotes (default 1).
	FootnoteNr int
	// FootnotePrefix is inserted between the "fn:"/"fnref:" marker and the footnote
	// name in every footnote id (default "").
	FootnotePrefix string
	// FootnoteBacklink is the (HTML-text-escaped) content of each reverse-footnote
	// link; the empty string suppresses back-links entirely (default "&#8617;").
	FootnoteBacklink string
	// FootnoteLinkText is a format string for the footnote reference's link text,
	// with "%s" replaced by the footnote number; empty means the bare number
	// (default "").
	FootnoteLinkText string
	// FootnoteBacklinkInline, when true, places each back-link inside the last
	// paragraph or header of a footnote's content (descending into nested blocks)
	// instead of appending it to (or after) only a top-level trailing paragraph.
	// Mirrors kramdown's :footnote_backlink_inline option (default false).
	FootnoteBacklinkInline bool
	// SyntaxHighlighter selects the code highlighter. "rouge" (kramdown's default)
	// routes code blocks/spans through the pure-Go go-ruby-rouge lexers; any other
	// value ("", "null", "minted", …) leaves them as plain <pre><code>.
	SyntaxHighlighter string
	// SyntaxHighlighterOpts carries the highlighter's sub-options (default_lang,
	// guess_lang, and the block:/span: disable flags).
	SyntaxHighlighterOpts SyntaxHighlighterOpts
	// LinkDefs supplies predefined link-reference definitions (kramdown's
	// :link_defs): a reference id maps to a URL and an optional title, resolvable by
	// "[text][id]" / "[id]" the same as a definition harvested from the source.
	LinkDefs map[string]LinkDef
	// TypographicSymbols overrides the replacement string kramdown emits for a named
	// typographic symbol (hellip, mdash, ndash, laquo, raquo, laquo_space,
	// raquo_space, lsquo, rsquo, ldquo, rdquo). A present entry is HTML-escaped and
	// emitted verbatim in place of the default entity; an absent key keeps the
	// default. Mirrors kramdown's :typographic_symbols option (default nil).
	TypographicSymbols map[string]string
}

// LinkDef is a predefined link-reference definition supplied via the LinkDefs
// option (kramdown's :link_defs): a destination URL and an optional title.
type LinkDef struct {
	URL   string
	Title string
}

// DefaultOptions returns the option set matching kramdown's own defaults, used
// when New is called with a nil option pointer.
func DefaultOptions() Options {
	return Options{
		AutoIds:           true,
		SmartQuotes:       true,
		Typographic:       true,
		HardWrap:          false,
		FootnoteNr:        1,
		FootnoteBacklink:  "&#8617;",
		SyntaxHighlighter: "rouge",
	}
}

// Document is a parsed kramdown source, the analogue of Kramdown::Document. It
// holds the element [Root], the resolved [Opts], and the [Warnings] accumulated
// while parsing (e.g. an undefined footnote reference), and renders HTML via
// ToHTML.
type Document struct {
	Root     *Element
	Opts     Options
	Warnings []string

	source      string  // original source, for span-level definition resolution
	parserState *parser // memoised parser holding harvested definitions
}

// New parses src under opts (nil selects DefaultOptions) and returns the parsed
// Document, mirroring Kramdown::Document.new(src, options). Parsing never fails;
// malformed constructs degrade to literal text exactly as kramdown does.
func New(src string, opts *Options) *Document {
	o := DefaultOptions()
	if opts != nil {
		o = *opts
	}
	p := newParser(src, o)
	root := p.parse()
	// p.opts may have been mutated by a {::options} extension during the parse; the
	// document (and its renderer) must observe those updated options.
	return &Document{Root: root, Opts: p.opts, Warnings: p.warnings, source: src, parserState: p}
}

// ToHTML renders the document to HTML, matching Kramdown::Document#to_html. Span
// parsing happens here, so any warnings it raises (e.g. an undefined footnote
// reference) are folded into [Document.Warnings] before returning.
func (d *Document) ToHTML() string {
	c := newHTMLConverter(d)
	out := c.convert()
	d.Warnings = d.parserState.warnings
	return out
}

// ToHTML is the one-shot convenience entry point: it parses src under opts and
// returns the HTML, equivalent to Kramdown::Document.new(src, options).to_html.
func ToHTML(src string, opts *Options) string {
	return New(src, opts).ToHTML()
}
