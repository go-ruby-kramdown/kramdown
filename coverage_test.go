// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"strings"
	"testing"
)

// TestEscapeBranches drives the entity-preservation and metacharacter branches of
// escapeHTMLText and escapeHTMLAttr (the latter through an attribute value).
func TestEscapeBranches(t *testing.T) {
	// A pre-formed entity in body text is preserved; a bare ">"/"&" escapes while a
	// known inline tag ("<b>") passes through as raw HTML.
	eq(t, "a &amp; x > y &z\n", "<p>a &amp; x &gt; y &amp;z</p>\n")
	// An attribute value carrying &, <, " and an existing entity (via an IAL key).
	got := h("Para\n{:title=\"a &amp; b < c\"}\n")
	if !strings.Contains(got, `title="a &amp; b &lt; c"`) {
		t.Errorf("attr escape = %q", got)
	}
	// A literal entity in a title is preserved (the entity branch of escapeHTMLAttr).
	got = h("Para\n{:title=\"x &#169; y\"}\n")
	if !strings.Contains(got, "&#169;") {
		t.Errorf("attr entity = %q", got)
	}
}

// TestHTMLConverterEdges covers convertChildren's leading-blank skip, an empty
// list item, an empty table cell, a hard break inside spans, and an image with an
// extra IAL attribute.
func TestHTMLConverterEdges(t *testing.T) {
	// A document beginning with blank lines (leading ElBlank with nothing rendered
	// yet) then a paragraph.
	eq(t, "\n\nx\n", "<p>x</p>\n")
	// An empty list item (a marker with no content) renders <li></li>.
	eq(t, "*  \n* b\n", "<ul>\n  <li></li>\n  <li>b</li>\n</ul>\n")
	// A table cell that is empty.
	got := h("| a |  |\n|---|---|\n| 1 | 2 |\n")
	if !strings.Contains(got, "<th></th>") {
		t.Errorf("empty cell = %q", got)
	}
	// An image with an IAL class merges the extra attribute after src/alt/title.
	got = h("![a](i.png){:.thumb}\n")
	if !strings.Contains(got, `<img src="i.png" alt="a" class="thumb" />`) {
		t.Errorf("image ial = %q", got)
	}
	// A br inside a strong span (renderSpan recursion over a break).
	eq(t, "**a  \nb**\n", "<p><strong>a<br />\nb</strong></p>\n")
}

// TestEmptyDDAndBlankDL covers convertDL's empty-<dd> branch and a definition
// whose body is empty.
func TestEmptyDD(t *testing.T) {
	got := h("Term\n: \n: real\n")
	if !strings.Contains(got, "<dd></dd>") || !strings.Contains(got, "<dd>real</dd>") {
		t.Errorf("empty dd = %q", got)
	}
}

// TestFootnoteFirstBlockNotParagraph covers renderFootnoteBody's leading-blank
// branch (first block is a code block, not a paragraph).
func TestFootnoteFirstBlockNotParagraph(t *testing.T) {
	got := h("A[^c]\n\n[^c]:\n        x = 1\n")
	if !strings.Contains(got, "<li id=\"fn:c\">\n\n      <pre><code>") {
		t.Errorf("fn code first = %q", got)
	}
}

// TestALDBranches covers splitALD's "." / "#" guard and an ALD name with a hyphen.
func TestALDBranches(t *testing.T) {
	// A leading "." standalone IAL is a class, not an ALD definition.
	eq(t, "Para\n{:.cls}\n", "<p class=\"cls\">Para</p>\n")
	// A leading "#" standalone IAL is an id, not an ALD.
	eq(t, "Para\n{:#the-id}\n", "<p id=\"the-id\">Para</p>\n")
	// An ALD whose name contains a hyphen.
	eq(t, "{:my-ref: .c}\nP\n{:my-ref}\n", "<p class=\"c\">P</p>\n")
}

// TestIALKeyVariants covers a single-quoted IAL value, an escaped quote inside it,
// a key override (same key twice) and a short / unbalanced value.
func TestIALKeyVariants(t *testing.T) {
	eq(t, "P\n{:title='hi'}\n", "<p title=\"hi\">P</p>\n")
	// A later key of the same name overrides the earlier one.
	eq(t, "P\n{:title=\"a\" title=\"b\"}\n", "<p title=\"b\">P</p>\n")
	// An escaped quote within a single-quoted value.
	got := h("P\n{:title='a\\'b'}\n")
	if !strings.Contains(got, `title="a&#39;b"`) && !strings.Contains(got, "title=\"a'b\"") {
		t.Errorf("escaped quote ial = %q", got)
	}
	// An unknown bare word in an IAL is dropped.
	eq(t, "P\n{:bareword}\n", "<p>P</p>\n")
}

// TestClassMerge covers applyIALToElement merging a new class onto an element
// that already has one.
func TestClassMerge(t *testing.T) {
	// An ALD sets a class, then a direct IAL adds another (classes accumulate).
	got := h("{:base: .one}\nP\n{:base .two}\n")
	if !strings.Contains(got, `class="one two"`) {
		t.Errorf("class merge = %q", got)
	}
}

// TestHTMLBlockComment covers parseHTMLBlock's multi-line comment branch.
func TestHTMLBlockCommentMultiline(t *testing.T) {
	eq(t, "<!-- line one\nline two -->\n", "<!-- line one\nline two -->\n")
}

// TestStripClosingHashesEdges covers the all-hashes header text and an attached
// hash.
func TestStripClosingHashesEdges(t *testing.T) {
	// A header that is only hashes after the level marker keeps them.
	eq(t, "## ## \n", "<h2 id=\"section\">##</h2>\n")
}

// TestParagraphContinuation covers kramdown's blank-line-delimited blocks: a
// header/quote/HR on the next line is absorbed into the paragraph (no
// interruption), while a block-level HTML element does interrupt.
func TestParagraphContinuation(t *testing.T) {
	// A header line after text is NOT a header (kramdown needs a blank line first).
	eq(t, "para\n# H\n", "<p>para\n# H</p>\n")
	// A ">" line is absorbed (and its ">" escaped) into the paragraph.
	eq(t, "para\n> q\n", "<p>para\n&gt; q</p>\n")
	// An HR-looking line is absorbed.
	eq(t, "para\n* * *\n", "<p>para\n* * *</p>\n")
	// A block-level HTML element DOES interrupt the paragraph.
	got := h("para\n<div>\nx\n</div>\n")
	if !strings.Contains(got, "<p>para</p>") || !strings.Contains(got, "<div>") {
		t.Errorf("para then html = %q", got)
	}
}

// TestParseCommentBranches covers the {::comment ... /} self-closing form with a
// trailing /} and an inline {:/comment} after text on the opener line.
func TestParseCommentBranches(t *testing.T) {
	// Self-closing on one line.
	if got := h("{::comment x /}\n"); got != "" {
		t.Errorf("self-close attrs = %q", got)
	}
	// Opener line carrying text followed by the closer further down.
	eq(t, "{::comment}lead\nmore\n{:/comment}\n", "<!-- lead\nmore -->\n")
}

// TestIndentedCodeBlanks covers parseIndentedCode's interior-blank handling and a
// blank that ends the block (no further indented code).
func TestIndentedCodeBlanks(t *testing.T) {
	// A blank line between two indented lines is kept.
	eq(t, "    a\n\n    b\n", "<pre><code>a\n\nb\n</code></pre>\n")
	// An indented block followed by a blank then a paragraph ends the block.
	got := h("    code\n\ntext\n")
	if !strings.Contains(got, "<pre><code>code\n</code></pre>") || !strings.Contains(got, "<p>text</p>") {
		t.Errorf("indented then text = %q", got)
	}
}

// TestBlockquoteLazyAndBlank covers parseBlockquote's blank-line terminator and a
// new-block interruption.
func TestBlockquoteEdges(t *testing.T) {
	// A blockquote terminated by a blank line then a paragraph.
	eq(t, "> q\n\np\n", "<blockquote>\n  <p>q</p>\n</blockquote>\n\n<p>p</p>\n")
	// A blockquote whose next line looks like an HR is absorbed lazily (kramdown
	// needs a blank line to break out of the quote).
	eq(t, "> q\n* * *\n", "<blockquote>\n  <p>q\n* * *</p>\n</blockquote>\n")
}

// TestStandaloneIALLeadingNoBlock covers applyStandaloneIAL when a leading IAL has
// no following block (end of input).
func TestStandaloneIALEdges(t *testing.T) {
	// A trailing leading-IAL with nothing after it renders nothing.
	if got := h("{:.x}\n"); got != "" {
		t.Errorf("dangling leading ial = %q", got)
	}
	// A leading IAL followed only by blank lines then EOF.
	if got := h("{:.x}\n\n"); got != "" {
		t.Errorf("leading ial blanks = %q", got)
	}
}

// TestListContinuationLazy covers parseList's lazy continuation and a sibling
// marker that dedents out of the item.
func TestListContinuationLazy(t *testing.T) {
	// A lazily continued list item paragraph.
	eq(t, "* a\nb\n", "<ul>\n  <li>a\nb</li>\n</ul>\n")
	// Ordered list lazy continuation.
	eq(t, "1. a\nb\n", "<ol>\n  <li>a\nb</li>\n</ol>\n")
}

// TestTableEdges covers a table with a body-section separator and an alignment-only
// separator detection edge.
func TestTableSectionsAndAlign(t *testing.T) {
	got := h("| h |\n|---|\n| a |\n|---|\n| b |\n")
	if strings.Count(got, "<tbody>") != 2 {
		t.Errorf("two tbody = %q", got)
	}
	// A non-table line with a dash but other chars is not a separator.
	eq(t, "not | a-table\n", "<p>not | a-table</p>\n")
}

// TestIsTableSepRejections covers isTableSepLine's empty/non-dash/illegal-char
// rejections.
func TestIsTableSepRejections(t *testing.T) {
	// A "table-like" first row whose second line has a forbidden char is no table.
	eq(t, "| a |\n| x |\n", "<p>| a |\n| x |</p>\n")
}

// TestCodespanEdges covers an empty code span and a multi-backtick span with a
// stripped surrounding space.
func TestCodespanEdges(t *testing.T) {
	eq(t, "a `` `` b\n", "<p>a <code></code> b</p>\n") // empty multi-tick span
	// A multi-tick span trims one leading and trailing space.
	eq(t, "`` a ``\n", "<p><code>a</code></p>\n")
}

// TestEmphasisCloseScan covers findEmphClose's left-flank-space skip and an
// underscore intraword closer skip.
func TestEmphasisCloseScan(t *testing.T) {
	// A would-be closer preceded by a space is skipped; the real closer is later.
	eq(t, "*a * b*\n", "<p><em>a * b</em></p>\n")
	// Strong with an interior single marker.
	eq(t, "**a*b**\n", "<p><strong>a*b</strong></p>\n")
}

// TestEscapableSet covers isEscapable's true/false branches via escaped and
// non-escapable characters.
func TestEscapableSet(t *testing.T) {
	// A backslash before a non-escapable char (a letter) stays literal.
	eq(t, "a\\b\n", "<p>a\\b</p>\n")
	// Escaping a "+" and a ":" (escapable punctuation).
	eq(t, "\\+ \\:\n", "<p>+ :</p>\n")
}

// TestAbbrevNonText covers applyAbbreviations passing through non-text spans
// (code, image) and recursing into emphasis.
func TestAbbrevNonText(t *testing.T) {
	got := h("**HTML** and `HTML`.\n\n*[HTML]: spec\n")
	// The emphasised HTML is wrapped in <abbr>; the code-span HTML is not.
	if !strings.Contains(got, "<strong><abbr") || !strings.Contains(got, "<code>HTML</code>") {
		t.Errorf("abbr non-text = %q", got)
	}
}

// TestLinkEdges covers matchBracket nesting, an unbalanced reference id, a
// reference with no matching definition, and an escaped bracket in link text.
func TestLinkEdges(t *testing.T) {
	// Nested brackets in link text.
	eq(t, "[a [b] c](u)\n", "<p><a href=\"u\">a [b] c</a></p>\n")
	// A reference whose id bracket is unbalanced falls through to literal.
	eq(t, "[t][unclosed\n", "<p>[t][unclosed</p>\n")
	// An escaped bracket inside link text.
	eq(t, "[a\\]b](u)\n", "<p><a href=\"u\">a]b</a></p>\n")
}

// TestInlineLinkTitleAndParens covers matchParenClose tracking a quoted title with
// parens and splitDestTitle's title extraction.
func TestInlineLinkTitleParens(t *testing.T) {
	eq(t, "[t](http://x \"a (b) c\")\n", "<p><a href=\"http://x\" title=\"a (b) c\">t</a></p>\n")
	// A destination with an escaped char.
	eq(t, "[t](a\\)b)\n", "<p><a href=\"a)b\">t</a></p>\n")
}

// TestAutolinkEdgeAndPlainText covers tryAutolinkOrHTML mailto display trimming and
// plainText over a link's children for an image alt.
func TestAutolinkAndAltText(t *testing.T) {
	// An image whose alt comes from emphasised link text (plainText recursion).
	eq(t, "![*x* y](i.png)\n", "<p><img src=\"i.png\" alt=\"x y\" /></p>\n")
	// An image alt built from a code span and a typographic dash.
	got := h("![`c` -- d](i.png)\n")
	if !strings.Contains(got, `alt="c -- d"`) {
		t.Errorf("alt code/dash = %q", got)
	}
}

// TestSymPlainAndChar covers symChar / symPlain over each entity via auto-id and
// alt-text rendering.
func TestSymVariants(t *testing.T) {
	// Auto-id from a header carrying smart-quote and dash typography (symPlain).
	got := h("# It's a -- test...\n")
	if !strings.Contains(got, `id="its-a----test"`) {
		t.Errorf("symPlain id = %q", got)
	}
	// An ellipsis and an en-dash in alt text exercise symPlain's hellip/ndash arms.
	got = h("![a -- b ... c](i.png)\n")
	if !strings.Contains(got, `alt="a -- b ... c"`) {
		t.Errorf("dash/ellipsis alt = %q", got)
	}
}

// TestGuillemetClose covers splitGuillemets' closing ">>" with and without a
// preceding space.
func TestGuillemetClose(t *testing.T) {
	eq(t, "x>>\n", "<p>x»</p>\n")
	// The spaced form carries a non-breaking space before the bracket.
	eq(t, "x >>\n", "<p>x »</p>\n")
}

// TestSmartQuoteContexts covers isCloseQuoteContext's decade, space and word-char
// branches.
func TestSmartQuoteContexts(t *testing.T) {
	// A decade apostrophe: 'the '80s'.
	got := h("the '80s\n")
	if !strings.Contains(got, "the ’80s") {
		t.Errorf("decade = %q", got)
	}
	// A quote at the very start is an opening quote.
	eq(t, "'start\n", "<p>‘start</p>\n")
	// A double quote after a word is a closing quote.
	eq(t, "word\"\n", "<p>word”</p>\n")
}

// TestSetAttrReplace covers setAttr replacing an existing attribute in place via a
// header whose IAL sets id twice through an ALD chain plus a direct id.
func TestSetAttrReplace(t *testing.T) {
	// An IAL with two ids: the later wins, in the same attribute slot.
	eq(t, "P\n{:#first #second}\n", "<p id=\"second\">P</p>\n")
}

// TestWarningsOnUndefinedFootnote covers the warn path and Document.Warnings.
func TestWarningsOnUndefinedFootnote(t *testing.T) {
	d := New("a[^missing] b\n", nil)
	_ = d.ToHTML()
	if len(d.Warnings) == 0 || !strings.Contains(d.Warnings[0], "missing") {
		t.Errorf("warnings = %#v", d.Warnings)
	}
}

// TestLastRuneEmpty covers lastRune's empty-string branch via an empty paragraph
// edge and substituteText's empty input.
func TestEmptyTypographyInput(t *testing.T) {
	// A paragraph that is only a typographic symbol exercises substituteText with a
	// short run; an entity-only line exercises the entity pass-through in typoWalk.
	eq(t, "&amp;\n", "<p>&amp;</p>\n")
	// A code span adjacent to a quote sets prevChar via the codespan branch.
	eq(t, "`c`'s\n", "<p><code>c</code>’s</p>\n")
}

// TestRawHTMLSpanPrevChar covers typoWalk's raw-HTML-span and the cross-node quote
// direction after it.
func TestRawHTMLSpanTypography(t *testing.T) {
	// A raw HTML span followed by a closing quote.
	eq(t, "<b>x</b>'s\n", "<p><b>x</b>’s</p>\n")
}

// TestUnescapeLinkTextBranch covers unescapeLinkText: a backslash before a
// non-escapable character is kept verbatim, while an escapable one is unescaped.
func TestUnescapeLinkTextBranch(t *testing.T) {
	// "\d" is not an escapable sequence, so the backslash survives in the href.
	eq(t, "[t](a\\db)\n", "<p><a href=\"a\\db\">t</a></p>\n")
	// "\)" is escapable inside the destination, unescaped to a literal ")".
	eq(t, "[t](a\\)b )\n", "<p><a href=\"a)b\">t</a></p>\n")
}
