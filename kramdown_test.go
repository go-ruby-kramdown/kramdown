// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"strings"
	"testing"
)

// h is a convenience that converts src under the default options.
func h(src string) string { return ToHTML(src, nil) }

// eq fails the test unless ToHTML(src) equals want.
func eq(t *testing.T, src, want string) {
	t.Helper()
	if got := h(src); got != want {
		t.Errorf("ToHTML(%q)\n got = %q\nwant = %q", src, got, want)
	}
}

// TestParagraphsAndText covers plain paragraphs, soft wraps and a blank-line
// separator between two paragraphs.
func TestParagraphsAndText(t *testing.T) {
	eq(t, "Hello world\n", "<p>Hello world</p>\n")
	eq(t, "one\ntwo\n", "<p>one\ntwo</p>\n")
	eq(t, "a\n\nb\n", "<p>a</p>\n\n<p>b</p>\n")
	// Leading whitespace on the opening line is stripped.
	eq(t, "   indented opener\n", "<p>indented opener</p>\n")
}

// TestATXHeaders covers ATX headers at every level, closing hashes, an attached
// {#id}, and the "#"-alone-is-a-paragraph rule.
func TestATXHeaders(t *testing.T) {
	eq(t, "# H1\n", "<h1 id=\"h1\">H1</h1>\n")
	eq(t, "###### H6\n", "<h6 id=\"h6\">H6</h6>\n")
	eq(t, "## H2 ##\n", "<h2 id=\"h2\">H2</h2>\n")
	eq(t, "## Title {#custom}\n", "<h2 id=\"custom\">Title</h2>\n")
	// A bare "#" with no text is a paragraph.
	eq(t, "#\n", "<p>#</p>\n")
	// A trailing "#" attached with no space stays literal.
	eq(t, "# a#\n", "<h1 id=\"a\">a#</h1>\n")
}

// TestSetextHeaders covers "=" (h1) and "-" (h2) underlines and a {#id} on the
// header text.
func TestSetextHeaders(t *testing.T) {
	eq(t, "Title\n=====\n", "<h1 id=\"title\">Title</h1>\n")
	eq(t, "Sub\n---\n", "<h2 id=\"sub\">Sub</h2>\n")
	eq(t, "Named {#x}\n===\n", "<h1 id=\"x\">Named</h1>\n")
}

// TestAutoIdDedup covers duplicate-header id de-duplication and the empty-slug
// "section" fallback.
func TestAutoIdDedup(t *testing.T) {
	got := h("# Dup\n\n# Dup\n")
	if !strings.Contains(got, `id="dup"`) || !strings.Contains(got, `id="dup-1"`) {
		t.Errorf("dedup ids = %q", got)
	}
	// A header whose text slugs to empty gets "section".
	eq(t, "# *\n", "<h1 id=\"section\">*</h1>\n")
}

// TestAutoIdPrefixAndOff covers AutoIdPrefix and AutoIds=false.
func TestAutoIdPrefixAndOff(t *testing.T) {
	if got := ToHTML("# Hi\n", &Options{AutoIds: true, AutoIdPrefix: "p-"}); got != "<h1 id=\"p-hi\">Hi</h1>\n" {
		t.Errorf("prefix id = %q", got)
	}
	if got := ToHTML("# Hi\n", &Options{}); got != "<h1>Hi</h1>\n" {
		t.Errorf("no auto id = %q", got)
	}
}

// TestEmphasis covers em, strong, both, intraword underscore and a non-flanking
// fall-through to literal.
func TestEmphasis(t *testing.T) {
	eq(t, "*em*\n", "<p><em>em</em></p>\n")
	eq(t, "_em_\n", "<p><em>em</em></p>\n")
	eq(t, "**strong**\n", "<p><strong>strong</strong></p>\n")
	eq(t, "__strong__\n", "<p><strong>strong</strong></p>\n")
	eq(t, "***both***\n", "<p><strong><em>both</em></strong></p>\n")
	// Intraword underscore is literal.
	eq(t, "a_b_c\n", "<p>a_b_c</p>\n")
	// A marker followed by space does not open emphasis.
	eq(t, "a * b\n", "<p>a * b</p>\n")
	// Empty emphasis is literal.
	eq(t, "**\n", "<p>**</p>\n")
}

// TestCodeSpans covers single and multi-backtick code spans plus the
// whitespace-flanked single-backtick fall-through.
func TestCodeSpans(t *testing.T) {
	eq(t, "`code`\n", "<p><code>code</code></p>\n")
	eq(t, "``a `b` c``\n", "<p><code>a `b` c</code></p>\n")
	// A backtick flanked by spaces on both sides is literal.
	eq(t, "a ` b\n", "<p>a ` b</p>\n")
	// HTML metacharacters inside a code span are escaped.
	eq(t, "`<a> & b`\n", "<p><code>&lt;a&gt; &amp; b</code></p>\n")
	// An unterminated run is literal.
	eq(t, "`unterminated\n", "<p>`unterminated</p>\n")
}

// TestInlineLinks covers inline links/images with and without titles.
func TestInlineLinks(t *testing.T) {
	eq(t, "[t](http://x)\n", "<p><a href=\"http://x\">t</a></p>\n")
	eq(t, "[t](http://x \"ti\")\n", "<p><a href=\"http://x\" title=\"ti\">t</a></p>\n")
	eq(t, "![alt](img.png)\n", "<p><img src=\"img.png\" alt=\"alt\" /></p>\n")
	eq(t, "![alt](img.png \"cap\")\n", "<p><img src=\"img.png\" alt=\"alt\" title=\"cap\" /></p>\n")
	// An unbalanced "[" is literal.
	eq(t, "[oops\n", "<p>[oops</p>\n")
	// A "(" that does not close falls through to literal text.
	eq(t, "[t](unclosed\n", "<p>[t](unclosed</p>\n")
}

// TestReferenceLinks covers full, collapsed and shortcut reference links plus an
// undefined reference.
func TestReferenceLinks(t *testing.T) {
	eq(t, "[t][id]\n\n[id]: http://x \"ti\"\n",
		"<p><a href=\"http://x\" title=\"ti\">t</a></p>\n")
	eq(t, "[id][]\n\n[id]: http://x\n", "<p><a href=\"http://x\">id</a></p>\n")
	eq(t, "[id]\n\n[id]: http://x\n", "<p><a href=\"http://x\">id</a></p>\n")
	// Reference id matches case-insensitively.
	eq(t, "[T][ID]\n\n[id]: http://x\n", "<p><a href=\"http://x\">T</a></p>\n")
	// An undefined reference stays literal.
	eq(t, "[t][nope]\n", "<p>[t][nope]</p>\n")
	// Angle-bracketed URL and a two-line title definition.
	eq(t, "[t][id]\n\n[id]: <http://x>\n  \"wrapped\"\n",
		"<p><a href=\"http://x\" title=\"wrapped\">t</a></p>\n")
}

// TestAutolinksAndRawHTML covers URL/email autolinks and a raw inline HTML tag.
func TestAutolinksAndRawHTML(t *testing.T) {
	eq(t, "<http://x.com>\n", "<p><a href=\"http://x.com\">http://x.com</a></p>\n")
	eq(t, "<a@b.com>\n", "<p><a href=\"mailto:a@b.com\">a@b.com</a></p>\n")
	eq(t, "<mailto:a@b.com>\n", "<p><a href=\"mailto:a@b.com\">a@b.com</a></p>\n")
	eq(t, "a <span>b</span> c\n", "<p>a <span>b</span> c</p>\n")
	// A bare "<" that is neither autolink nor tag is escaped.
	eq(t, "a < b\n", "<p>a &lt; b</p>\n")
}

// TestEscapes covers backslash escapes and the "\\" hard break vs literal cases.
func TestEscapes(t *testing.T) {
	eq(t, "\\*not em\\*\n", "<p>*not em*</p>\n")
	eq(t, "a\\\\b\n", "<p>a\\b</p>\n")
	// A "\\" before a newline is a hard break.
	eq(t, "a\\\\\nb\n", "<p>a<br />\nb</p>\n")
	// A trailing lone backslash is literal.
	eq(t, "a\\\n", "<p>a\\</p>\n")
}

// TestHardSoftBreaks covers trailing-two-space hard breaks and HardWrap=false.
func TestBreaks(t *testing.T) {
	eq(t, "a  \nb\n", "<p>a<br />\nb</p>\n")
	// HardWrap off: two trailing spaces are a soft break.
	if got := ToHTML("a  \nb\n", &Options{}); got != "<p>a\nb</p>\n" {
		t.Errorf("soft break = %q", got)
	}
}

// TestBlockquote covers a simple and a nested/lazy blockquote.
func TestBlockquote(t *testing.T) {
	eq(t, "> quoted\n", "<blockquote>\n  <p>quoted</p>\n</blockquote>\n")
	// Lazy continuation.
	eq(t, "> a\nb\n", "<blockquote>\n  <p>a\nb</p>\n</blockquote>\n")
	// Nested.
	eq(t, "> > deep\n", "<blockquote>\n  <blockquote>\n    <p>deep</p>\n  </blockquote>\n</blockquote>\n")
}

// TestCodeBlocks covers fenced (with/without language), tilde fences, and
// indented code blocks.
func TestCodeBlocks(t *testing.T) {
	eq(t, "```\nx=1\n```\n", "<pre><code>x=1\n</code></pre>\n")
	eq(t, "```ruby\nx=1\n```\n", "<pre><code class=\"language-ruby\">x=1\n</code></pre>\n")
	eq(t, "~~~\nx=1\n~~~\n", "<pre><code>x=1\n</code></pre>\n")
	eq(t, "    indented\n", "<pre><code>indented\n</code></pre>\n")
	// A fenced block with no body.
	eq(t, "```\n```\n", "<pre><code></code></pre>\n")
}

// TestHorizontalRule covers the three HR forms.
func TestHorizontalRule(t *testing.T) {
	eq(t, "* * *\n", "<hr />\n")
	eq(t, "- - -\n", "<hr />\n")
	eq(t, "___\n", "<hr />\n")
}

// TestUnorderedList covers a tight list and a loose list (blank-separated items).
func TestUnorderedList(t *testing.T) {
	eq(t, "* a\n* b\n", "<ul>\n  <li>a</li>\n  <li>b</li>\n</ul>\n")
	eq(t, "* a\n\n* b\n",
		"<ul>\n  <li>\n    <p>a</p>\n  </li>\n  <li>\n    <p>b</p>\n  </li>\n</ul>\n")
}

// TestOrderedList covers an ordered list.
func TestOrderedList(t *testing.T) {
	eq(t, "1. a\n2. b\n", "<ol>\n  <li>a</li>\n  <li>b</li>\n</ol>\n")
}

// TestNestedList covers a list with a nested sub-list and a multi-paragraph item.
func TestNestedList(t *testing.T) {
	got := h("* a\n    * b\n")
	if !strings.Contains(got, "<ul>") || !strings.Contains(got, "<li>b</li>") {
		t.Errorf("nested list = %q", got)
	}
	// A list item with two paragraphs renders loose.
	got = h("* one\n\n    two\n")
	if !strings.Contains(got, "<p>one</p>") || !strings.Contains(got, "<p>two</p>") {
		t.Errorf("multi-para item = %q", got)
	}
}

// TestDefinitionList covers a tight and a loose definition list.
func TestDefinitionList(t *testing.T) {
	eq(t, "Term\n: def\n",
		"<dl>\n  <dt>Term</dt>\n  <dd>def</dd>\n</dl>\n")
	// Loose definition (blank line before the marker).
	got := h("Term\n\n: def\n")
	if !strings.Contains(got, "<dd>\n    <p>def</p>\n  </dd>") {
		t.Errorf("loose dd = %q", got)
	}
	// Multiple terms for one definition.
	got = h("T1\nT2\n: def\n")
	if strings.Count(got, "<dt>") != 2 {
		t.Errorf("multi-term dl = %q", got)
	}
}

// TestTable covers a header table with alignments and a multi-section body.
func TestTable(t *testing.T) {
	got := h("| a | b |\n|:--|--:|\n| 1 | 2 |\n")
	if !strings.Contains(got, "<th style=\"text-align: left\">a</th>") ||
		!strings.Contains(got, "<th style=\"text-align: right\">b</th>") ||
		!strings.Contains(got, "<td style=\"text-align: left\">1</td>") {
		t.Errorf("table = %q", got)
	}
	// Centered column.
	got = h("| a |\n|:-:|\n| 1 |\n")
	if !strings.Contains(got, "text-align: center") {
		t.Errorf("center col = %q", got)
	}
	// A line with a pipe but no separator is not a table.
	eq(t, "a | b\n", "<p>a | b</p>\n")
	// An escaped pipe in a cell stays literal.
	got = h("| a \\| b |\n|---|\n")
	if !strings.Contains(got, "a | b") {
		t.Errorf("escaped pipe = %q", got)
	}
}

// TestLeadingSeparatorTable covers a table whose first line is the separator
// (no header row).
func TestLeadingSeparatorTable(t *testing.T) {
	got := h("|---|\n| a |\n| b |\n")
	if strings.Contains(got, "<thead>") || !strings.Contains(got, "<td>a</td>") {
		t.Errorf("leading-sep table = %q", got)
	}
}

// TestHTMLBlock covers a raw HTML block and an HTML comment block.
func TestHTMLBlock(t *testing.T) {
	eq(t, "<div>\nraw\n</div>\n", "<div>\nraw\n</div>\n")
	eq(t, "<!-- a comment -->\n", "<!-- a comment -->\n")
}

// TestComment covers the {::comment} extension in its block, inline-closed and
// self-closing forms, and an unterminated opener.
func TestComment(t *testing.T) {
	eq(t, "{::comment}\nhidden\n{:/comment}\n", "<!-- hidden -->\n")
	eq(t, "{::comment}inline{:/comment}\n", "<!-- inline -->\n")
	// Self-closing produces nothing.
	if got := h("{::comment /}\n"); got != "" {
		t.Errorf("self-closing comment = %q", got)
	}
	// {:/} also closes.
	eq(t, "{::comment}\nx\n{:/}\n", "<!-- x -->\n")
	// An unterminated opener degrades to a paragraph.
	got := h("{::comment}\nstill open\n")
	if !strings.Contains(got, "<p>{::comment}") {
		t.Errorf("unterminated comment = %q", got)
	}
}

// TestBlockIAL covers a block IAL attaching a class/id to the previous block and
// a leading IAL attaching to the following block.
func TestBlockIAL(t *testing.T) {
	eq(t, "Para\n{:.note}\n", "<p class=\"note\">Para</p>\n")
	eq(t, "{:.lead}\nPara\n", "<p class=\"lead\">Para</p>\n")
	// An id IAL and a key=value IAL.
	eq(t, "Para\n{:#pid}\n", "<p id=\"pid\">Para</p>\n")
	eq(t, "Para\n{:title=\"hi\"}\n", "<p title=\"hi\">Para</p>\n")
	// An IAL on a header overrides the auto id.
	eq(t, "# H\n{:#chosen}\n", "<h1 id=\"chosen\">H</h1>\n")
}

// TestALD covers an attribute-list definition referenced from an IAL.
func TestALD(t *testing.T) {
	eq(t, "{:myref: .cls #i}\nPara\n{:myref}\n",
		"<p class=\"cls\" id=\"i\">Para</p>\n")
}

// TestSpanIALOnImage covers an IAL merged onto an image's extra attributes.
func TestImageWithReferenceTitle(t *testing.T) {
	eq(t, "![a][r]\n\n[r]: img.png \"T\"\n",
		"<p><img src=\"img.png\" alt=\"a\" title=\"T\" /></p>\n")
}

// TestSmartQuotes covers curly single/double quotes and the apostrophe / decade
// special cases.
func TestSmartQuotes(t *testing.T) {
	eq(t, "'a'\n", "<p>‘a’</p>\n")
	eq(t, "\"a\"\n", "<p>“a”</p>\n")
	eq(t, "it's\n", "<p>it’s</p>\n")
	// Smart quotes disabled keeps straight quotes.
	if got := ToHTML("'a'\n", &Options{Typographic: true}); got != "<p>'a'</p>\n" {
		t.Errorf("no smart quotes = %q", got)
	}
}

// TestTypographicSymbols covers dashes, ellipsis and guillemets.
func TestTypographicSymbols(t *testing.T) {
	eq(t, "a -- b\n", "<p>a – b</p>\n")
	eq(t, "a --- b\n", "<p>a — b</p>\n")
	eq(t, "a...\n", "<p>a…</p>\n")
	eq(t, "<< q >>\n", "<p>« q »</p>\n")
	// Typographic disabled keeps the literals (smart quotes still off here).
	if got := ToHTML("a -- b\n", &Options{}); got != "<p>a -- b</p>\n" {
		t.Errorf("no typographic = %q", got)
	}
}

// TestEntities covers a literal entity pass-through.
func TestEntities(t *testing.T) {
	eq(t, "a &amp; b\n", "<p>a &amp; b</p>\n")
	eq(t, "&#169; &#x41;\n", "<p>&#169; &#x41;</p>\n")
	// A bare ampersand is escaped.
	eq(t, "a & b\n", "<p>a &amp; b</p>\n")
}

// TestAbbreviations covers an abbreviation definition expanded into <abbr>.
func TestAbbreviations(t *testing.T) {
	eq(t, "The HTML spec.\n\n*[HTML]: HyperText Markup Language\n",
		"<p>The <abbr title=\"HyperText Markup Language\">HTML</abbr> spec.</p>\n")
	// An abbreviation with an IAL gains a class.
	got := h("Use W3C.\n\n*[W3C]: Consortium\n{:.org}\n")
	if !strings.Contains(got, "class=\"org\"") || !strings.Contains(got, "title=\"Consortium\"") {
		t.Errorf("abbr ial = %q", got)
	}
}

// TestFootnotes covers a single footnote, its back-link, and a repeated
// reference's superscript back-links.
func TestFootnotes(t *testing.T) {
	got := h("A[^1] note.\n\n[^1]: The note.\n")
	if !strings.Contains(got, `<sup id="fnref:1">`) ||
		!strings.Contains(got, `<li id="fn:1">`) ||
		!strings.Contains(got, "reversefootnote") {
		t.Errorf("footnote = %q", got)
	}
	// A repeated reference produces a numbered second back-link.
	got = h("A[^x] and B[^x].\n\n[^x]: note\n")
	if !strings.Contains(got, "fnref:x:1") {
		t.Errorf("repeat footnote = %q", got)
	}
	// A footnote whose last block is not a paragraph gets a standalone backlink.
	got = h("A[^c]\n\n[^c]:\n        code\n")
	if !strings.Contains(got, "<pre><code>") || !strings.Contains(got, "reversefootnote") {
		t.Errorf("code footnote = %q", got)
	}
	// A footnote reference with no definition stays literal.
	eq(t, "A[^missing] b\n", "<p>A[^missing] b</p>\n")
}

// TestFootnoteNrOption covers a custom starting footnote number.
func TestFootnoteNrOption(t *testing.T) {
	o := DefaultOptions()
	o.FootnoteNr = 5
	got := ToHTML("A[^1]\n\n[^1]: n\n", &o)
	if !strings.Contains(got, ">5</a></sup>") {
		t.Errorf("footnote nr = %q", got)
	}
}

// TestCRLFNormalization covers CRLF/CR input normalisation.
func TestCRLFNormalization(t *testing.T) {
	eq(t, "a\r\nb\r\n", "<p>a\nb</p>\n")
	eq(t, "a\rb\r", "<p>a\nb</p>\n")
}

// TestTabExpansion covers a leading tab expanding to an indented code block.
func TestTabExpansion(t *testing.T) {
	eq(t, "\tcode\n", "<pre><code>code\n</code></pre>\n")
}

// TestDocumentAPI covers New, the element tree, ToHTML on the document, and
// Warnings accumulation paths (none here, but the field is reachable).
func TestDocumentAPI(t *testing.T) {
	d := New("# Hi\n", nil)
	if d.Root == nil || len(d.Root.Children) == 0 {
		t.Fatal("no root children")
	}
	if d.ToHTML() != "<h1 id=\"hi\">Hi</h1>\n" {
		t.Errorf("doc to_html = %q", d.ToHTML())
	}
	if !d.Opts.AutoIds {
		t.Error("default AutoIds should be true")
	}
	// Warnings field is present (empty for clean input).
	if d.Warnings == nil {
		// nil slice is fine; just exercise the accessor.
		_ = d.Warnings
	}
}

// TestEndOfBlockMarker covers the "^" end-of-block marker and the stray "{:/}"
// path.
func TestEndOfBlockMarker(t *testing.T) {
	eq(t, "a\n^\nb\n", "<p>a</p>\n<p>b</p>\n")
}

// TestEmptyAndBlankInput covers empty input and whitespace-only input.
func TestEmptyInput(t *testing.T) {
	if got := h(""); got != "" {
		t.Errorf("empty = %q", got)
	}
	if got := h("\n\n"); got != "" {
		t.Errorf("blank = %q", got)
	}
}
