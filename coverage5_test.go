// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestBlockMathParsing covers the display-math block parser's recognition edges:
// a standalone single- and multi-line block, an escaped opener, trailing text after
// the closing "$$", a following non-boundary line, and an unterminated block — all
// of which except the first two fall back to paragraph/span handling.
func TestBlockMathParsing(t *testing.T) {
	cases := []struct{ in, want string }{
		// A standalone single-line block renders unindented as "\[value\]".
		{"$$5+5$$\n", "\\[5+5\\]\n"},
		// A multi-line block accumulates its interior lines verbatim.
		{"$$\n5+5\n$$\n", "\\[5+5\\]\n"},
		// An escaped opener ("\$$") is not recognised as a math block: it stays a
		// paragraph (span math, which would strip the backslash, is out of scope here).
		{`\$$5+5$$`, "<p>\\$$5+5$$</p>\n"},
		// Text after the closing "$$" makes it inline (not a standalone block), so the
		// literal "$$…$$" survives in the paragraph.
		{"$$5+5$$ tail\n", "<p>$$5+5$$ tail</p>\n"},
		// A following non-blank, non-"^" line means the "$$" is not at a block
		// boundary: the whole thing stays a paragraph.
		{"$$5+5$$\nmore text\n", "<p>$$5+5$$\nmore text</p>\n"},
		// An unterminated block (no closing "$$") is a plain paragraph.
		{"$$5+5\nunterminated\n", "<p>$$5+5\nunterminated</p>\n"},
	}
	for _, c := range cases {
		if got := ToHTML(c.in, nil); got != c.want {
			t.Errorf("ToHTML(%q):\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

// TestEscapeMathValue covers every replacement branch of the math escaper,
// including the ampersand and double-quote cases the corpus does not exercise.
func TestEscapeMathValue(t *testing.T) {
	got := escapeMathValue(`a & b < c > d "e" f`)
	want := `a &amp; b &lt; c &gt; d &quot;e&quot; f`
	if got != want {
		t.Errorf("escapeMathValue: got %q want %q", got, want)
	}
}

// TestFootnoteHelpers covers the footnote back-link placement helpers' dead-end
// branches: an empty child list, a container that descends to no paragraph/header,
// and back-link injection into HTML with no paragraph or header at all.
func TestFootnoteHelpers(t *testing.T) {
	if lastContentChild(nil) != nil {
		t.Error("lastContentChild(nil) should be nil")
	}
	// A blockquote whose only child is a blank descends but finds no paragraph.
	bq := newEl(ElBlockquote)
	bq.Children = []*Element{newEl(ElBlank)}
	if lastParagraphOrHeader([]*Element{bq}) != nil {
		t.Error("lastParagraphOrHeader should be nil for a blank-only blockquote")
	}
	// No paragraph or header present: the back-link is appended at the end.
	got := injectFootnoteBacklink("<div>x</div>\n", "BL")
	if got != "<div>x</div>\nBL" {
		t.Errorf("injectFootnoteBacklink fallback: got %q", got)
	}
}
