// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"strings"
	"testing"
)

// TestAttrDoubleQuoteEscape covers escapeHTMLAttr's '"' branch via an image alt
// containing a double quote.
func TestAttrDoubleQuoteEscape(t *testing.T) {
	got := h("![a \"b\" c](i.png)\n")
	if !strings.Contains(got, `alt="a &quot;b&quot; c"`) {
		t.Errorf("alt quote = %q", got)
	}
}

// TestFootnoteDefBetweenBlocks covers convertChildren skipping an ElFootnoteDef
// that sits between two rendered blocks (not at the document start).
func TestFootnoteDefBetweenBlocks(t *testing.T) {
	got := h("para one\n\n[^x]: a note\n\nA[^x] para two\n")
	if !strings.Contains(got, "<p>para one</p>") || !strings.Contains(got, "para two") {
		t.Errorf("fn def between = %q", got)
	}
}

// TestIALWhitespaceAndEmptyValue covers parseIAL's all-whitespace break and
// unquoteIAL's short-value branch.
func TestIALEdges(t *testing.T) {
	// An IAL that is only whitespace contributes nothing.
	eq(t, "P\n{:   }\n", "<p>P</p>\n")
	// An empty-string key value (len < 2 after the quotes are the whole token).
	got := h("P\n{:title=\"\"}\n")
	if !strings.Contains(got, `title=""`) {
		t.Errorf("empty value = %q", got)
	}
}

// TestClassMergeExisting covers applyIALToElement appending to an element that
// already carries a class (the existing!="" branch).
func TestClassMergeExisting(t *testing.T) {
	// A header auto-gets no class; give it one via IAL, then a second IAL adds more.
	got := h("P\n{:.a}\n{:.b}\n")
	if !strings.Contains(got, `class="a b"`) {
		t.Errorf("class append = %q", got)
	}
}

// TestFencedTrailingBlank covers parseFencedCode/parseParagraph trailing-blank trim
// and a fenced block whose content ends in blank lines.
func TestFencedTrailingBlanks(t *testing.T) {
	eq(t, "```\nx\n\n```\n", "<pre><code>x\n\n</code></pre>\n")
}

// TestBlockquoteThenHTMLBlock covers parseBlockquote's break at a new block (an
// HTML block on a non-quoted line ends the quote).
func TestBlockquoteThenHTMLBlock(t *testing.T) {
	got := h("> q\n<div>x</div>\n")
	if !strings.Contains(got, "<blockquote>") || !strings.Contains(got, "<div>x</div>") {
		t.Errorf("bq then html = %q", got)
	}
}

// TestFootnoteDefTrailingBlank covers harvestDefinitions trimming trailing blank
// lines of a footnote body.
func TestFootnoteDefTrailingBlankBody(t *testing.T) {
	got := h("A[^t]\n\n[^t]: the note\n    \n\nnext\n")
	if !strings.Contains(got, "the note") || !strings.Contains(got, "<p>next</p>") {
		t.Errorf("fn trailing blank = %q", got)
	}
}

// TestListSiblingDedentBreak covers parseList's m==nil break and the j>=len break
// in the continuation scan.
func TestListBreaks(t *testing.T) {
	// A list whose final item is followed by end-of-input after a blank line.
	eq(t, "* a\n\n", "<ul>\n  <li>a</li>\n</ul>\n")
	// A list immediately at end of input.
	eq(t, "* only\n", "<ul>\n  <li>only</li>\n</ul>\n")
}

// TestDefListFirstLineMarker covers tryDefinitionList's first-line-is-marker /
// blank rejection and a term-then-marker continuation.
func TestDefListEdges(t *testing.T) {
	// A line starting with ": " at top level is not a definition list (no term).
	got := h(": orphan\n")
	if strings.Contains(got, "<dl>") {
		t.Errorf("orphan marker = %q", got)
	}
	// A definition list continued by a new term + marker after a blank line.
	got = h("T1\n: d1\n\nT2\n: d2\n")
	if strings.Count(got, "<dt>") != 2 || strings.Count(got, "<dd>") != 2 {
		t.Errorf("two-term dl = %q", got)
	}
}

// TestLeadingSepWithSecondSeparator covers tryTable's leading-separator header
// detection (rows before a second separator become the header).
func TestLeadingSepHeader(t *testing.T) {
	got := h("|---|\n| a |\n|---|\n| b |\n")
	if !strings.Contains(got, "<thead>") || !strings.Contains(got, "<th>a</th>") ||
		!strings.Contains(got, "<td>b</td>") {
		t.Errorf("leadsep header = %q", got)
	}
}

// TestBangNotImage covers span.run's "!" that is not followed by a valid image
// (left literal).
func TestBangLiteral(t *testing.T) {
	// "![x]" with no destination and no matching reference stays literal.
	eq(t, "![x] y\n", "<p>![x] y</p>\n")
	// A bare "!" not followed by "[".
	eq(t, "a ! b\n", "<p>a ! b</p>\n")
}

// TestEmphasisUnderscoreIntrawordClose covers findEmphClose skipping an underscore
// closer that is intraword on its right.
func TestEmphasisUnderscoreClose(t *testing.T) {
	// "_a_b_" : the middle "_" is intraword (followed by b), so the closer is the
	// final one.
	eq(t, "_a_b_\n", "<p><em>a_b</em></p>\n")
}

// TestAbbrevSortLongestFirst covers applyAbbreviations' sort over two definitions.
func TestAbbrevTwoDefs(t *testing.T) {
	got := h("HTML and CSS.\n\n*[HTML]: markup\n*[CSS]: styles\n")
	if !strings.Contains(got, `title="markup"`) || !strings.Contains(got, `title="styles"`) {
		t.Errorf("two abbrevs = %q", got)
	}
}

// TestFootnoteRefInImageAlt covers plainText's ElFootnoteRef branch via an image
// whose alt text carries a footnote reference.
func TestFootnoteRefInAlt(t *testing.T) {
	got := h("![pre [^n] post](i.png)\n\n[^n]: note\n")
	if !strings.Contains(got, `alt="pre n post"`) {
		t.Errorf("fn in alt = %q", got)
	}
}

// TestGuillemetNoSpace covers symChar/splitGuillemets' plain laquo/raquo arms (no
// adjacent space).
func TestGuillemetNoSpace(t *testing.T) {
	eq(t, "<<x>>\n", "<p>«x»</p>\n")
}

// TestMixedDashEllipsis covers splitSub's already-resolved-part skip and
// substituteText combining a dash and an ellipsis in one run.
func TestMixedDashEllipsis(t *testing.T) {
	eq(t, "a -- b ... c\n", "<p>a – b … c</p>\n")
}

// TestSetAttrReplaceInPlace covers setAttr replacing an existing attribute value
// in place (an id set twice keeps a single id attribute).
func TestSetAttrReplaceInPlace(t *testing.T) {
	// A header with an explicit {#id} then an IAL id: the IAL overrides in place.
	eq(t, "# H {#a}\n{:#b}\n", "<h1 id=\"b\">H</h1>\n")
}

// TestIALTrailingSpaceToken covers parseIAL's TrimLeft-to-empty break (a trailing
// space after the last token).
func TestIALTrailingSpace(t *testing.T) {
	eq(t, "P\n{:.cls }\n", "<p class=\"cls\">P</p>\n")
}

// TestGuillemetOpenNoTrailing covers splitGuillemets' laquo with idx>0 (text before
// the opener) and a closing ">>" at end.
func TestGuillemetWithText(t *testing.T) {
	eq(t, "pre <<q\n", "<p>pre «q</p>\n")
	// The spaced guillemets carry a non-breaking space toward the enclosed word.
	const nb = " "
	eq(t, "a << b >> c\n", "<p>a «"+nb+"b"+nb+"» c</p>\n")
}

// TestSmartQuotesEmptyAndDouble covers substituteText's empty-input guard via a
// hard-break-only paragraph and a double-quote run.
func TestSmartQuotesDoubleRun(t *testing.T) {
	// Two separate quoted words exercise the prevChar reset between them.
	eq(t, "\"a\" \"b\"\n", "<p>“a” “b”</p>\n")
}

// TestDLContinuationTermMarker covers tryDefinitionList's "blank then term then
// marker" continuation branch (the j+1 lookahead).
func TestDLContinuationTermThenMarker(t *testing.T) {
	got := h("T1\n: d1\n\nT2\n: d2\n")
	if !strings.Contains(got, "<dt>T1</dt>") || !strings.Contains(got, "<dt>T2</dt>") {
		t.Errorf("dl term-marker cont = %q", got)
	}
}

// TestListItemThenHTMLBlock covers parseList's gather break at a non-indented HTML
// block following an item.
func TestListItemThenHTMLBlock(t *testing.T) {
	got := h("* a\n<div>x</div>\n")
	if !strings.Contains(got, "<li>a</li>") || !strings.Contains(got, "<div>x</div>") {
		t.Errorf("item then html = %q", got)
	}
}

// TestDLThenPlainAndHTML covers tryDefinitionList's blank-then-non-continuation
// break and a definition body terminated by an HTML block.
func TestDLTerminations(t *testing.T) {
	// A definition list, a blank, then a plain paragraph ends the list.
	got := h("T\n: d\n\nplain para\n")
	if !strings.Contains(got, "<dl>") || !strings.Contains(got, "<p>plain para</p>") {
		t.Errorf("dl then plain = %q", got)
	}
	// A definition body terminated by an HTML block.
	got = h("T\n: d\n<div>x</div>\n")
	if !strings.Contains(got, "<dd>d</dd>") || !strings.Contains(got, "<div>x</div>") {
		t.Errorf("dd then html = %q", got)
	}
}

// TestMultipleSymbolsInRun covers splitSub/splitDashes skipping already-resolved
// symbol parts when several substitutions occur in one text run.
func TestMultipleSymbolsInRun(t *testing.T) {
	eq(t, "a...b---c...d\n", "<p>a…b—c…d</p>\n")
}

// TestThreeTightItems covers parseList iterating several sibling markers (the
// sibling-marker path and tight-list assembly).
func TestThreeTightItems(t *testing.T) {
	eq(t, "* a\n* b\n* c\n", "<ul>\n  <li>a</li>\n  <li>b</li>\n  <li>c</li>\n</ul>\n")
}

// TestLastRuneEmptyAndPrevFlush covers lastRune's empty branch and smartQuotes'
// prevOf reading from the flushed buffer.
func TestLastRuneAndPrev(t *testing.T) {
	// Two quotes back to back: the second reads prevChar from the just-emitted text.
	eq(t, "say \"hi\"\n", "<p>say “hi”</p>\n")
	// A leading quote with empty preceding context (lastRune of "" path).
	eq(t, "\"x\n", "<p>“x</p>\n")
}
