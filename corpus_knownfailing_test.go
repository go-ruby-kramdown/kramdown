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

	// block/11_ial (1)
	// simple: needs deferred, nested ALD-reference resolution with kramdown's
	// update_attr_with_ial ordering (refs resolved first; multiple "{:name: …}" ALD
	// definitions accumulated rather than overwritten), a list terminated by a
	// following standalone block IAL (its trailing indented lines becoming separate
	// IAL-decorated code blocks), and accumulation of consecutive leading block IALs —
	// core attribute-model/list reworks out of scope here. (nested graduated with the
	// block content model + markdown-attribute front-end.)
	"block/11_ial/simple.text": true,
}
