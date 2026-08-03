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

	// block/06_codeblock (5)
	"block/06_codeblock/error.text":               true,
	"block/06_codeblock/lazy.text":                true,
	"block/06_codeblock/no_newline_at_end_1.text": true,
	"block/06_codeblock/whitespace.text":          true,
	"block/06_codeblock/with_blank_line.text":     true,

	// block/08_list (11)
	"block/08_list/item_ial.text":            true,
	"block/08_list/lazy.text":                true,
	"block/08_list/lazy_and_nested.text":     true,
	"block/08_list/list_and_hr.text":         true,
	"block/08_list/list_and_others.text":     true,
	"block/08_list/mixed.text":               true,
	"block/08_list/nested.text":              true,
	"block/08_list/other_first_element.text": true,
	"block/08_list/simple_ol.text":           true,
	"block/08_list/simple_ul.text":           true,
	"block/08_list/special_cases.text":       true,

	// block/10_ald (1)
	"block/10_ald/simple.text": true,

	// block/11_ial (3)
	"block/11_ial/auto_id_and_ial.text": true,
	"block/11_ial/nested.text":          true,
	"block/11_ial/simple.text":          true,

	// block/12_extension (6)
	"block/12_extension/comment.text":    true,
	"block/12_extension/ignored.text":    true,
	"block/12_extension/nomarkdown.text": true,
	"block/12_extension/options.text":    true,
	"block/12_extension/options2.text":   true,
	"block/12_extension/options3.text":   true,

	// block/13_definition_list (7)
	"block/13_definition_list/auto_ids.text":         true,
	"block/13_definition_list/deflist_ial.text":      true,
	"block/13_definition_list/item_ial.text":         true,
	"block/13_definition_list/separated_by_eob.text": true,
	"block/13_definition_list/simple.text":           true,
	"block/13_definition_list/too_much_space.text":   true,
	"block/13_definition_list/with_blocks.text":      true,

	// block/14_table (5)
	"block/14_table/errors.text":              true,
	"block/14_table/escaping.text":            true,
	"block/14_table/footer.text":              true,
	"block/14_table/simple.text":              true,
	"block/14_table/table_with_footnote.text": true,

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

	// span/03_codespan (1)
	"span/03_codespan/highlighting.text": true,

	// span/04_footnote (8)
	"span/04_footnote/backlink_inline.text":    true,
	"span/04_footnote/backlink_text.text":      true,
	"span/04_footnote/definitions.text":        true,
	"span/04_footnote/footnote_link_text.text": true,
	"span/04_footnote/footnote_nr.text":        true,
	"span/04_footnote/footnote_prefix.text":    true,
	"span/04_footnote/inside_footnote.text":    true,
	"span/04_footnote/without_backlink.text":   true,

	// span/abbreviations (3)
	"span/abbreviations/abbrev.text":         true,
	"span/abbreviations/abbrev_defs.text":    true,
	"span/abbreviations/abbrev_in_html.text": true,

	// span/autolinks (1)
	"span/autolinks/url_links.text": true,

	// span/extension (3)
	"span/extension/comment.text":    true,
	"span/extension/nomarkdown.text": true,
	"span/extension/options.text":    true,

	// span/ial (1)
	"span/ial/simple.text": true,

	// span/line_breaks (1)
	"span/line_breaks/normal.text": true,

	// span/text_substitutions (1)
	"span/text_substitutions/typography_subst.text": true,
}
