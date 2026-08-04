// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

// corpusExceptions are corpus cases that are structurally out of scope for a
// pure-Go, no-Ruby-runtime kramdown port: they depend on a Ruby-runtime library
// (a syntax highlighter, a math engine, the HTML5 front-end) or on output modes
// this port deliberately does not model. They are held apart from knownFailing so
// the closeable-gap ledger reflects real, reachable work. Like knownFailing this
// set is shrink-only — if one ever starts matching, the ratchet says so and it
// should graduate out. Each bucket documents WHY it diverges.
var corpusExceptions = map[string]bool{

	// --- entityout (6): smart-quote direction now matches the gem (emphasis/normal and
	// typography graduated via the parse-time smart_quotes port). These remaining cases
	// each need a feature this port does not yet model: the full named-entity table
	// (entities_numeric/symbolic re-encode &copy; etc.), a custom smart_quotes spec
	// (entities_as_char), the {:footnotes} placement directive (placement), or
	// footnote/header ordering (regexp_problem). entities stays on the :as_input
	// bare-"&" escaping.
	"span/04_footnote/placement.text":                true,
	"span/04_footnote/regexp_problem.text":           true,
	"span/text_substitutions/entities.text":          true,
	"span/text_substitutions/entities_as_char.text":  true,
	"span/text_substitutions/entities_numeric.text":  true,
	"span/text_substitutions/entities_symbolic.text": true,

	// --- highlight (2): Syntax highlighting cases this port cannot yet close by wiring the
	// pure-Go go-ruby-rouge highlighter. rouge/simple mixes a Ruby, an HTML and a
	// PHP block; go-ruby-rouge ships no PHP lexer, so the third block's token spans
	// cannot be reproduced. rouge/multiple additionally selects a bespoke
	// RougeHTMLFormatters formatter defined inside kramdown's own Ruby test harness
	// (it wraps every block in <div class="custom-class">), which no pure-Go wiring
	// can supply. The remaining rouge cases (block & span highlighting, default_lang,
	// guess_lang, syntax_highlighter: null, disable flags) now pass via the wiring.
	"block/06_codeblock/rouge/multiple.text": true,
	"block/06_codeblock/rouge/simple.text":   true,

	// --- math (4): Math via a math engine. The default MathJax engine's plain
	// "\[…\]"/"\(…\)" wrapping is now supported for single-line block math (so
	// gh_128 passes), but these remaining cases need the itex2mml/KaTeX or
	// :math_engine ~ fallback markup, or multi-line/edge-case block+span handling.
	"block/15_math/no_engine.text": true,
	"block/15_math/normal.text":    true,
	"span/math/no_engine.text":     true,
	"span/math/normal.text":        true,

	// --- toc (5): Table-of-contents generation with non-default toc options (toc_levels ranges,
	// exclusion). Depends on auto-id transliteration and the toc converter's runtime
	// tree walk.
	"block/16_toc/no_toc.text":             true,
	"block/16_toc/toc_exclude.text":        true,
	"block/16_toc/toc_levels.text":         true,
	"block/16_toc/toc_with_footnotes.text": true,
	"block/16_toc/toc_with_links.text":     true,
}
