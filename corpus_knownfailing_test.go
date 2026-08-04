// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

// knownFailing is the ratchet ledger: corpus cases (keyed by their .text path
// relative to testdata/testcases) that this port does not yet render byte-for-byte
// like the kramdown 2.5.2 gem BUT which are in scope to fix. TestCorpusRatchet
// enforces that this set only ever shrinks: an entry that starts passing must be
// removed. Structural, runtime-dependent divergences live separately in
// corpusExceptions (see corpus_exceptions_test.go).
//
// Grouped by kramdown feature; each cluster is closed in its own PR.
var knownFailing = map[string]bool{

	// block/06_codeblock (1)
	// whitespace: the show-whitespaces converter renders literal tabs inside the
	// code as <span class="ws-tab">\t</span>, but this port expands tabs to spaces
	// in a global pre-pass, so the literal tab is no longer available.
	"block/06_codeblock/whitespace.text": true,

	// block/11_ial (2)
	// nested: kramdown parses the "<div>…</div>" raw block as an HTML element and
	// injects the leading/trailing IAL's class/id into the opening tag (and reparses
	// a "markdown=\"1\"" body), which needs the HTML-element block front-end this
	// port renders verbatim instead. simple: needs deferred, nested ALD-reference
	// resolution with kramdown's update_attr_with_ial ordering (refs resolved first;
	// multiple "{:name: …}" ALD definitions accumulated rather than overwritten), a
	// list terminated by a following standalone block IAL (its trailing indented
	// lines becoming separate IAL-decorated code blocks), and accumulation of
	// consecutive leading block IALs — core attribute-model/list reworks out of
	// scope here.
	"block/11_ial/nested.text": true,
	"block/11_ial/simple.text": true,

	// block/12_extension (1)
	// options: needs parse_block_html/parse_span_html (the HTML5 front-end).
	// (options3, which needed the rouge token markup, now passes via the go-ruby-rouge wiring.)
	"block/12_extension/options.text": true,

	// block/14_table (1)
	// simple: one row uses an unclosed inline <em> HTML element that kramdown
	// auto-closes at the cell boundary (span-HTML content model), rendering the
	// stray "</em>" in the next cell as escaped text; out of scope until the span
	// HTML front-end lands.
	"block/14_table/simple.text": true,

	// encoding.text (1)
	"encoding.text": true,

	// span/01_link (2)
	// link_defs + reference need kramdown's block-boundary-aware link-definition
	// parser (bare URLs containing spaces, the "space before a quote" invalidation
	// guard, definitions harvested only at a block boundary rather than mid-paragraph,
	// multi-line titles, the predefined :link_defs option, and literal-tab
	// preservation) — a block-parser rework tracked separately. The inline/reference
	// span algorithm, angle-bracket destinations, nested-image alt and per-definition
	// IAL now match the gem.
	"span/01_link/link_defs.text": true,
	"span/01_link/reference.text": true,

	// span/04_footnote (3)
	"span/04_footnote/backlink_inline.text": true,
	"span/04_footnote/definitions.text":     true,
	"span/04_footnote/inside_footnote.text": true,

	// span/abbreviations (2)
	"span/abbreviations/abbrev.text":         true,
	"span/abbreviations/abbrev_in_html.text": true,

	// span/extension (1)
	"span/extension/options.text": true,
}
