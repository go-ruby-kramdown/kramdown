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

	// --- htmlparse (12): HTML-input parsing still blocked on the html_to_native
	// mapping. The raw content model, the block/span content models (:parse_block_html),
	// the markdown="…" attribute and the span-level HTML front-end (parse_span_html) are
	// ported — parse_block_html / parse_as_span / html_and_codeblocks /
	// content_model/{deflists,tables} / invalid_html_1 / markdown_attr / cdata_section /
	// comment / span/05_html/* / empty_tag_in_cell have graduated. simple stays only on
	// the entity-output divergence (its "&nbsp;" is emitted as the named entity, not the
	// U+00A0 character — an entityout concern, below). These remaining ones need
	// html_to_native (ElementConverter, cluster 3) or the script/style verbatim body
	// (parse_as_raw, cluster 5).
	"block/03_paragraph/with_html_to_native.text":    true,
	"block/09_html/html_to_native/code.text":         true,
	"block/09_html/html_to_native/comment.text":      true,
	"block/09_html/html_to_native/emphasis.text":     true,
	"block/09_html/html_to_native/header.text":       true,
	"block/09_html/html_to_native/list_ol.text":      true,
	"block/09_html/html_to_native/list_ul.text":      true,
	"block/09_html/html_to_native/paragraph.text":    true,
	"block/09_html/html_to_native/table_simple.text": true,
	"block/09_html/html_to_native/typography.text":   true,
	"block/09_html/parse_as_raw.text":                true,
	"block/09_html/simple.text":                      true,

	// --- entityout (9): entity_output modes (:numeric / :symbolic / :as_input / :as_char) plus custom
	// smart_quotes specs. The gem re-encodes every character entity according to a
	// runtime table of ~2000 named HTML entities; this port emits the default
	// as-char/named form only.
	"span/02_emphasis/normal.text":                   true,
	"span/04_footnote/markers.text":                  true,
	"span/04_footnote/placement.text":                true,
	"span/04_footnote/regexp_problem.text":           true,
	"span/text_substitutions/entities.text":          true,
	"span/text_substitutions/entities_as_char.text":  true,
	"span/text_substitutions/entities_numeric.text":  true,
	"span/text_substitutions/entities_symbolic.text": true,
	"span/text_substitutions/typography.text":        true,

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

	// --- headeropt (2): header_offset / header_links options — remap header levels or inject anchor
	// links; not modelled by this port's Options.
	"block/04_header/header_type_offset.text": true,
	"block/04_header/with_header_links.text":  true,

	// --- cjk (1): remove_line_breaks_for_cjk — Unicode East-Asian-width line-break elision, a
	// locale/runtime behaviour.
	"cjk-line-break.text": true,

	// --- translit (1): transliterated_header_ids — auto-ids built from Unicode-to-ASCII
	// transliteration (the gem's stringex-style table).
	"block/04_header/with_auto_ids.text": true,
}
