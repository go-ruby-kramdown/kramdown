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

	// block/03_paragraph (1)
	"block/03_paragraph/standalone_image.text": true,

	// block/04_header (3)
	"block/04_header/atx_header.text":             true,
	"block/04_header/setext_header.text":          true,
	"block/04_header/with_auto_id_stripping.text": true,

	// block/05_blockquote (1)
	"block/05_blockquote/lazy.text": true,

	// block/06_codeblock (1)
	// whitespace: the show-whitespaces converter renders literal tabs inside the
	// code as <span class="ws-tab">\t</span>, but this port expands tabs to spaces
	// in a global pre-pass, so the literal tab is no longer available.
	"block/06_codeblock/whitespace.text": true,

	// block/08_list (4)
	"block/08_list/lazy_and_nested.text":     true,
	"block/08_list/list_and_others.text":     true,
	"block/08_list/mixed.text":               true,
	"block/08_list/other_first_element.text": true,

	// block/11_ial (2)
	"block/11_ial/nested.text": true,
	"block/11_ial/simple.text": true,

	// block/12_extension (1)
	// options: needs parse_block_html/parse_span_html (the HTML5 front-end).
	// (options3, which needed the rouge token markup, now passes via the go-ruby-rouge wiring.)
	"block/12_extension/options.text": true,

	// block/13_definition_list (3)
	"block/13_definition_list/auto_ids.text":    true,
	"block/13_definition_list/item_ial.text":    true,
	"block/13_definition_list/with_blocks.text": true,

	// block/14_table (2)
	// errors: a table directly after a harvested link-reference definition (no
	// blank line) requires kramdown's after_block_boundary semantics, which this
	// port's definition pre-pass does not model. simple: one row uses an unclosed
	// inline <em> HTML element that kramdown auto-closes at the cell boundary
	// (span-HTML content model), out of scope until the HTML front-end lands.
	"block/14_table/errors.text": true,
	"block/14_table/simple.text": true,

	// encoding.text (1)
	"encoding.text": true,

	// span/01_link (6)
	"span/01_link/image_in_a.text":                true,
	"span/01_link/inline.text":                    true,
	"span/01_link/link_defs.text":                 true,
	"span/01_link/link_defs_with_ial.text":        true,
	"span/01_link/links_with_angle_brackets.text": true,
	"span/01_link/reference.text":                 true,

	// span/02_emphasis (3)
	"span/02_emphasis/empty.text":   true,
	"span/02_emphasis/errors.text":  true,
	"span/02_emphasis/nesting.text": true,

	// span/04_footnote (3)
	"span/04_footnote/backlink_inline.text": true,
	"span/04_footnote/definitions.text":     true,
	"span/04_footnote/inside_footnote.text": true,

	// span/abbreviations (2)
	"span/abbreviations/abbrev.text":         true,
	"span/abbreviations/abbrev_in_html.text": true,

	// span/extension (1)
	"span/extension/options.text": true,

	// span/text_substitutions (1)
	"span/text_substitutions/typography_subst.text": true,
}
