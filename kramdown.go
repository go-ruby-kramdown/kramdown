// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

// Options configures a conversion, mirroring the keyword options accepted by
// Kramdown::Document.new. Only the options that influence the HTML output of the
// supported feature set are honoured; the rest are tolerated for API parity.
type Options struct {
	// Input selects the parser dialect, mirroring kramdown's :input option (the name
	// of a Kramdown::Parser subclass). The empty value and "kramdown" select the
	// default kramdown parser; "GFM" selects the GitHub-Flavored-Markdown dialect,
	// which (among other things) renders "- [ ] "/"- [x] " list items as task-list
	// checkboxes. Only the task-list behaviour of the GFM parser is honoured here.
	Input string
	// AutoIds, when true (kramdown's default), assigns a generated id="" to every
	// header that lacks an explicit {#id}.
	AutoIds bool
	// AutoIdPrefix is prepended to every auto-generated header id (default "").
	AutoIdPrefix string
	// AutoIdStripping, when true, slugs a header id from the header's parsed plain
	// text (markup/HTML stripped) instead of its literal source. Mirrors kramdown's
	// deprecated :auto_id_stripping option (default false).
	AutoIdStripping bool
	// HeaderOffset shifts every header's output level by this amount, clamped to
	// 1..6 (an h1 with offset 1 renders as <h2>). Mirrors kramdown's :header_offset
	// option (default 0).
	HeaderOffset int
	// HeaderLinks, when true, prepends an empty self-anchor (<a href="#id"></a>) to
	// every header that carries a non-blank id. Mirrors kramdown's :header_links
	// option (default false).
	HeaderLinks bool
	// TransliteratedHeaderIds, when true, transliterates a header's text to ASCII
	// (via the vendored Stringex unidecoder table) before slugging it into an
	// auto-generated id. Mirrors kramdown's :transliterated_header_ids option
	// (default false).
	TransliteratedHeaderIds bool
	// SmartQuotes enables typographic substitution of quotes/dashes/ellipses
	// (kramdown's default).
	SmartQuotes bool
	// SmartQuotesSubst overrides the entity each smart-quote position maps to, in the
	// order [lsquo, rsquo, ldquo, rdquo] (kramdown's :smart_quotes array). A blank
	// entry falls back to that position's default name, so the zero value renders the
	// usual curly quotes. Mirrors e.g. :smart_quotes: apos,apos,quot,quot.
	SmartQuotesSubst [4]string
	// Typographic enables the --, ---, ... and <<>> substitutions (default true).
	Typographic bool
	// EntityOutput selects how a recognised HTML entity is rendered, mirroring
	// kramdown's :entity_output: "as_char" (the default) emits the entity's character,
	// except <, > and & which fall back to their named/numeric form; "as_input" keeps
	// the entity's literal input form; "numeric" emits "&#cp;"; and "symbolic" emits
	// "&name;" when the entity has a name, else "&#cp;". An empty value means as_char.
	EntityOutput string
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
	// ParseSpanHTML, when true (kramdown's default), parses the Markdown content of
	// a raw inline HTML element (so "<span>*x*</span>" emphasises its body). Set
	// false via an inline "{::options parse_span_html=\"false\" /}" extension, a raw
	// inline element's body is instead passed through verbatim.
	ParseSpanHTML bool
	// ParseBlockHTML, when true, gives every parsed block-level HTML element its
	// native content model (kramdown's HTML_CONTENT_MODEL): a :block element reparses
	// its body as Markdown blocks, a :span element span-parses its body, and a :raw
	// element keeps its content verbatim. When false (kramdown's default) every block
	// HTML element uses the raw content model. Mirrors kramdown's :parse_block_html.
	ParseBlockHTML bool
	// HtmlToNative, when true, runs kramdown's Parser::Html::ElementConverter over
	// every parsed raw-HTML element, mapping it to the equivalent native element where
	// possible (<b>/<strong> -> :strong, <i>/<em> -> :em, <h1>.. -> :header,
	// <code>/<pre> -> :codespan/:codeblock, a simple <table> -> :table, and the
	// list/paragraph/blockquote containers), converting entities in their text and
	// applying kramdown's whitespace normalisation. When false (the default) parsed
	// HTML elements are serialised verbatim. Mirrors kramdown's :html_to_native.
	HtmlToNative bool
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
	// RemoveLineBreaksForCJK, when true, elides a soft line break that sits between
	// two runs of East-Asian (Han/Hiragana/Katakana) characters, so source wrapped
	// one CJK "word" per line renders as unbroken text. Mirrors kramdown's
	// :remove_line_breaks_for_cjk option (default false).
	RemoveLineBreaksForCJK bool
	// TocLevels lists the header levels included in a {:toc} table of contents
	// (kramdown's :toc_levels). An empty/nil value means kramdown's default of every
	// level 1..6; e.g. []int{2, 3} restricts the TOC to h2 and h3.
	TocLevels []int
	// MathEngine selects how a "$$…$$" math element renders. "mathjax" (kramdown's
	// default) wraps the LaTeX verbatim in MathJax delimiters — "\[…\]" for a block
	// element, "\(…\)" for a span — with the value HTML-escaped; a math element that
	// carries IAL attributes is wrapped in a <div>/<span> instead. An empty value
	// (kramdown's :math_engine nil, the corpus' ":math_engine: ~") disables the engine:
	// the raw LaTeX is emitted inside a <div>/<span class="kdmath"> as "$$…$$"/"$…$".
	MathEngine string
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
		EntityOutput:      "as_char",
		HardWrap:          false,
		FootnoteNr:        1,
		FootnoteBacklink:  "&#8617;",
		ParseSpanHTML:     true,
		SyntaxHighlighter: "rouge",
		MathEngine:        "mathjax",
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
