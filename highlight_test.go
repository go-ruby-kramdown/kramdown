// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestHighlightPlainWithResidualAttrs exercises the plain-path attribute handling
// when the language is consumed from an IAL that also carries other classes and an
// id, and the named lexer is unknown (so the block degrades to plain <pre><code>).
// The consumed language-<x> token is stripped from the <pre> class, the remaining
// class and the id survive, and the language moves to the <code> element — matching
// kramdown 2.5.2 byte-for-byte.
func TestHighlightPlainWithResidualAttrs(t *testing.T) {
	eq(t, "~~~\ncode\n~~~\n{: .language-foobar .extra #x}\n",
		"<pre class=\"extra\" id=\"x\"><code class=\"language-foobar\">code\n</code></pre>\n")
}

// TestHighlightInlineSyntaxHighlighterNull covers the {::options
// syntax_highlighter="null"} inline switch turning highlighting off, after which a
// labelled fence renders plain. (A document-initial {::options} tag leaves a blank
// line, hence the leading newline.)
func TestHighlightInlineSyntaxHighlighterNull(t *testing.T) {
	eq(t, "{::options syntax_highlighter=\"null\" /}\n\n~~~ruby\nx=1\n~~~\n",
		"\n<pre><code class=\"language-ruby\">x=1\n</code></pre>\n")
}

// TestHighlightInlineGuessLang covers the {::options syntax_highlighter_opts}
// inline guess_lang flag: an unlabelled fence is wrapped in the highlighter-rouge
// markup with Rouge's plaintext (unhighlighted) content.
func TestHighlightInlineGuessLang(t *testing.T) {
	eq(t, "{::options syntax_highlighter_opts=\"{guess_lang: true\\}\" /}\n\n~~~\nfoo bar\n~~~\n",
		"\n<div class=\"highlighter-rouge\"><div class=\"highlight\"><pre class=\"highlight\">"+
			"<code>foo bar\n</code></pre>\n</div></div>\n")
}
