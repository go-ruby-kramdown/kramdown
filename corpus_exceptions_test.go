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

	// --- highlight (1): a syntax-highlighting case that is structurally out of
	// scope for a pure-Go, no-Ruby-runtime port. rouge/multiple sets
	// `formatter: RougeHTMLFormatters` in its options — a custom Rouge formatter
	// subclass defined *only inside kramdown's own Ruby test harness*
	// (kramdown 2.5.2 test/test_files.rb), whose #stream wraps every block's token
	// stream in an extra <div class="custom-class">…</div>. It is not a kramdown
	// library feature: the options file names an arbitrary Ruby class that the gem's
	// test process happens to have loaded, so no pure-Go wiring can resolve or run
	// it. (The three code blocks themselves — two Ruby, one PHP — now tokenise
	// byte-for-byte; only the harness-injected wrapper div is unreachable.)
	// rouge/simple is closed by the go-ruby-rouge PHP lexer + the tag-based
	// language class; the remaining rouge cases (block & span highlighting,
	// default_lang, guess_lang, syntax_highlighter: null, disable flags) pass too.
	"block/06_codeblock/rouge/multiple.text": true,
}
