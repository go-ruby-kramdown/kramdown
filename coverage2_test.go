// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"strings"
	"testing"
)

// TestMultiBlockDefinition covers tryDefinitionList's multi-line / lazy / blank
// continuation of a definition body and a definition with a nested block.
func TestMultiBlockDefinition(t *testing.T) {
	// A definition whose body continues over an indented line and a lazy line.
	got := h("Term\n: line one\n  line two\nlazy\n")
	if !strings.Contains(got, "line one\n") || !strings.Contains(got, "lazy") {
		t.Errorf("multiline def = %q", got)
	}
	// A definition body with an interior blank line that continues (indented).
	got = h("Term\n: para one\n\n    para two\n")
	if !strings.Contains(got, "<p>para one</p>") || !strings.Contains(got, "<p>para two</p>") {
		t.Errorf("blank-continued def = %q", got)
	}
	// Two definitions for one term, the second separated by a blank (loose).
	got = h("Term\n: first\n\n: second\n")
	if strings.Count(got, "<dd>") < 1 || !strings.Contains(got, "second") {
		t.Errorf("two defs = %q", got)
	}
}

// TestFootnoteMultiParagraph covers a footnote whose definition has two paragraphs
// (renderFootnoteBody's non-final block path and the back-link in the last one).
func TestFootnoteMultiParagraph(t *testing.T) {
	got := h("A[^m]\n\n[^m]: first para\n\n    second para\n")
	if !strings.Contains(got, "<p>first para</p>") ||
		!strings.Contains(got, "second para") ||
		!strings.Contains(got, "reversefootnote") {
		t.Errorf("multi-para footnote = %q", got)
	}
}

// TestHarvestLinkDefVariants covers harvestDefinitions branches: an indented link
// definition and a footnote definition with a trailing IAL.
func TestHarvestVariants(t *testing.T) {
	// A reference definition indented up to three spaces.
	eq(t, "[t][id]\n\n   [id]: http://x\n", "<p><a href=\"http://x\">t</a></p>\n")
	// A footnote definition followed by a block IAL line (the IAL is consumed).
	got := h("A[^f]\n\n[^f]: note\n{:.fnclass}\n")
	if !strings.Contains(got, "note") {
		t.Errorf("footnote+ial = %q", got)
	}
}

// TestSetAttrAppendAndReplace covers setAttr appending a new attribute and the
// getAttr miss branch via an image with a title plus an extra IAL key.
func TestSetAttrAppend(t *testing.T) {
	// An image with a title and an extra IAL key=value (appended after title).
	got := h("![a](i.png \"cap\"){:data-x=\"1\"}\n")
	if !strings.Contains(got, `title="cap"`) || !strings.Contains(got, `data-x="1"`) {
		t.Errorf("image extra attr = %q", got)
	}
}

// TestEscapeAttrEntityPreserved covers escapeHTMLAttr's "<" and the existing-entity
// pass-through in an href via an autolink-like reference.
func TestEscapeHrefEntity(t *testing.T) {
	// A link URL containing a bare "&" gets escaped to &amp; in the href.
	eq(t, "[t](http://x?a&b)\n", "<p><a href=\"http://x?a&amp;b\">t</a></p>\n")
	// A URL with an already-formed entity is preserved.
	eq(t, "[t](http://x?a&amp;b)\n", "<p><a href=\"http://x?a&amp;b\">t</a></p>\n")
}

// TestSmartQuoteEdges covers smartQuotes prevChar carried across a flushed buffer
// and a double-quote opening at the start of a run.
func TestSmartQuoteCrossNode(t *testing.T) {
	// Text then a single quote then text (closing), with buffer-derived prevChar.
	eq(t, "ab'cd\n", "<p>ab’cd</p>\n")
	// A double-open quote at start, closed after a word.
	eq(t, "\"hi\" there\n", "<p>“hi” there</p>\n")
	// A single quote preceded by a typographic symbol (prevChar via symbol).
	eq(t, "a...'b'\n", "<p>a…‘b’</p>\n")
}

// TestTypographyDisabledKeepsText covers substituteText's SmartQuotes-off path
// (Typographic on, SmartQuotes off) keeping the quote literal next to a dash.
func TestTypographyMixedOptions(t *testing.T) {
	got := ToHTML("a -- 'b'\n", &Options{Typographic: true})
	if got != "<p>a – 'b'</p>\n" {
		t.Errorf("mixed typo = %q", got)
	}
}

// TestParseCommentInlineOpenerText covers parseComment when the opener carries the
// closer on the same later line via index > 0.
func TestParseCommentTrailingClose(t *testing.T) {
	eq(t, "{::comment}\nbody text {:/comment}\n", "<!-- body text -->\n")
}

// TestTableNoSeparatorWithinTwo covers tryTable returning nil when no separator is
// found within the first two lines.
func TestTableNoEarlySeparator(t *testing.T) {
	// A pipe-bearing block whose separator is on the third line is not a table (the
	// "---" is then read as an em-dash by the typography pass).
	eq(t, "| a |\n| b |\n|---|\n", "<p>| a |\n| b |\n|—|</p>\n")
}

// TestParenCloseNested covers matchParenClose tracking nested parens in a
// destination.
func TestInlineLinkNestedParens(t *testing.T) {
	eq(t, "[t](http://x(y)z)\n", "<p><a href=\"http://x(y)z\">t</a></p>\n")
}

// TestEmphasisLongRun covers tryEmphasis's >3-marker run treated as a single
// opener.
func TestEmphasisLongRun(t *testing.T) {
	// Four asterisks: kramdown treats the opener as a single em over the rest.
	got := h("****x****\n")
	if !strings.Contains(got, "x") {
		t.Errorf("long emphasis = %q", got)
	}
}

// TestListBlankThenNonList covers parseList's blank run followed by a non-item line
// (the list ends).
func TestListEndsAtBlankNonItem(t *testing.T) {
	got := h("* a\n\ntext\n")
	if !strings.Contains(got, "<ul>") || !strings.Contains(got, "<p>text</p>") {
		t.Errorf("list then text = %q", got)
	}
}

// TestIndentedCodeEndsAtParagraph covers parseIndentedCode's blank-then-text break.
func TestIndentedCodeThenText(t *testing.T) {
	got := h("    code\ntext\n")
	if !strings.Contains(got, "<pre><code>code") {
		t.Errorf("indented then unindented = %q", got)
	}
}

// TestRawHTMLSpanInText covers renderSpan's ElRawHTMLSpan branch and an entity span.
func TestRawHTMLAndEntitySpans(t *testing.T) {
	eq(t, "x <em>y</em> z\n", "<p>x <em>y</em> z</p>\n")
	eq(t, "&copy; 2026\n", "<p>&copy; 2026</p>\n")
}
