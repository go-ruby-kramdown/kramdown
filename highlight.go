// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"strings"

	rouge "github.com/go-ruby-rouge/rouge"
)

// This file wires the pure-Go go-ruby-rouge syntax highlighter into kramdown's
// code-block and code-span rendering, mirroring the gem's default
// syntax_highlighter: :rouge. When Rouge owns a lexer for the resolved language it
// emits the gem's <div class="language-<x> highlighter-rouge"><div class="highlight">
// <pre class="highlight"><code>…token-spans…</code></pre></div></div> markup
// byte-for-byte; otherwise (highlighter disabled, unknown lexer, or no language and
// no guess_lang) the block degrades to the plain <pre><code> form.

// SyntaxHighlighterOpts mirrors the recognised keys of kramdown's
// :syntax_highlighter_opts hash that influence HTML output. Nested per-context
// blocks (block:/span:) collapse to the two Disable flags this port honours.
type SyntaxHighlighterOpts struct {
	// DefaultLang is the language assumed for a code block/span that carries none
	// (kramdown's default_lang).
	DefaultLang string
	// GuessLang, when true, asks Rouge to sniff the language of an unlabelled block
	// (kramdown's guess_lang). A failed guess yields Rouge's plaintext lexer, which
	// still produces the highlighter-rouge wrapper with unhighlighted content.
	GuessLang bool
	// BlockDisable suppresses highlighting for code blocks (block: {disable: true}).
	BlockDisable bool
	// SpanDisable suppresses highlighting for code spans (span: {disable: true}).
	SpanDisable bool
}

// reLangClass matches the first language-<x> token of a class attribute, the way
// kramdown's Converter#extract_code_language reads a code element's language.
var reLangClass = regexp.MustCompile(`\blanguage-(\w+)`)

// rougeEnabled reports whether the document's highlighter is Rouge (kramdown's
// default). Any other value — "", "null", "minted", "coderay" — leaves this port on
// its plain <pre><code> path.
func (c *htmlConverter) rougeEnabled() bool {
	return c.doc.Opts.SyntaxHighlighter == "rouge"
}

// peekLangClass returns the language-<x> language named by e's class attribute
// without mutating it, or "" when absent.
func peekLangClass(e *Element) string {
	if cls, ok := e.getAttr("class"); ok {
		if m := reLangClass.FindStringSubmatch(cls); m != nil {
			return m[1]
		}
	}
	return ""
}

// stripLangToken removes the language-<lang> token from a space-separated class
// value, returning the remaining classes (possibly empty).
func stripLangToken(cls, lang string) string {
	fields := strings.Fields(cls)
	kept := fields[:0]
	for _, f := range fields {
		if f == "language-"+lang {
			continue
		}
		kept = append(kept, f)
	}
	return strings.Join(kept, " ")
}

// rougeHighlight renders text with Rouge's lexer for lang (or the guessed lexer
// when lang is empty and guess is set), returning the inner token markup and the
// language-class prefix ("language-<lang> " or "" for a guess) to place on the
// wrapper. The second result is false when no lexer applies (highlighter off,
// unknown named lexer, or no language and no guess), signalling the plain path.
func (c *htmlConverter) rougeHighlight(text, lang string, guess bool) (inner, langClass string, ok bool) {
	var lx rouge.Lexer
	if lang != "" {
		if lx = rouge.FindFancy(lang); lx == nil {
			return "", "", false
		}
		// The wrapper's language-<x> class is the resolved lexer's canonical tag,
		// not the raw fenced spec (kramdown builds it from lexer.tag): an alias or a
		// "tag?opts" fancy spec — e.g. php?start_inline=1 — collapses to its tag
		// (language-php).
		langClass = "language-" + lx.Tag() + " "
	} else if guess {
		// A guessed block carries no language-<x> class (kramdown leaves the code
		// element's language nil), so the wrapper is just highlighter-rouge.
		lx = rouge.Guess(text)
	} else {
		return "", "", false
	}
	// Format the resolved lexer instance directly (rather than re-resolving by tag)
	// so a fancy-spec option such as start_inline is honoured. FindFancy/Guess
	// always return a registered lexer, so this cannot fail.
	inner = rouge.HTMLFormatter{}.Format(lx.Lex(text))
	return inner, langClass, true
}
