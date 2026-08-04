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
}
