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
	// HardWrap, when true (kramdown's default), turns a trailing-two-spaces line
	// into a hard <br />. When false, only explicit "\\" forces a break.
	HardWrap bool
	// FootnoteNr is the starting number for footnotes (default 1).
	FootnoteNr int
}

// DefaultOptions returns the option set matching kramdown's own defaults, used
// when New is called with a nil option pointer.
func DefaultOptions() Options {
	return Options{
		AutoIds:     true,
		SmartQuotes: true,
		Typographic: true,
		HardWrap:    true,
		FootnoteNr:  1,
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
	return &Document{Root: root, Opts: o, Warnings: p.warnings, source: src, parserState: p}
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
