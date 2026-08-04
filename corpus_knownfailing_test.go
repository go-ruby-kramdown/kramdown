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
// Grouped by kramdown feature; each cluster is closed in its own PR. Empty: every
// in-scope corpus case now renders byte-for-byte like the gem; the only remaining
// divergences are the runtime-dependent cases in corpusExceptions.
var knownFailing = map[string]bool{}
