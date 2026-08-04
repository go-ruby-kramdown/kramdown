// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// htmlRawOpts returns the option set the raw-HTML corpus cases run under (kramdown's
// file-runner default with auto_ids off), so these unit tests match the gem oracle.
func htmlRawOpts() Options {
	o := DefaultOptions()
	o.AutoIds = false
	return o
}

// TestRawHTMLFrontEnd checks the raw-content-model serialisation against outputs
// captured from the kramdown 2.5.2 gem (ruby -e "require 'kramdown'").
func TestRawHTMLFrontEnd(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"empty_div", "<div></div>\n", "<div></div>\n"},
		{"pi_in_raw", "<div>\n<?php echo 1;?>\n</div>\n", "<div>\n<?php echo 1;?>\n</div>\n"},
		{"comment_in_raw", "<div>\n<!-- c -->\n</div>\n", "<div>\n<!-- c -->\n</div>\n"},
		{"cdata_in_raw", "<figure><![CDATA[hello]]></figure>\n", "<figure>hello</figure>\n"},
		{"id_blank_dropped", "<p id=\"  \">x</p>\n", "<p>x</p>\n"},
		{"top_level_comment", "<!-- top -->\n", "<!-- top -->\n"},
		{"void_selfclosed", "<hr>\n", "<hr />\n"},
	}
	o := htmlRawOpts()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToHTML(tc.in, &o); got != tc.want {
				t.Errorf("ToHTML(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestParseBlockHTMLBailouts covers the block driver's decision paths: a span-only
// element is never a block construct (declined), and a construct that ends mid-line
// (trailing content after the close tag) commits the element and re-injects the line
// remainder as its own block so the caller resumes on it.
func TestParseBlockHTMLBailouts(t *testing.T) {
	p := newParser("", DefaultOptions())
	if n, ok := p.parseBlockHTML([]string{"<em>x</em>"}, 0, newEl(ElRoot)); ok || n != 0 {
		t.Errorf("span element: got (%d,%v), want (0,false)", n, ok)
	}
	root := newEl(ElRoot)
	lines := []string{"<div>x</div> y"}
	if n, ok := p.parseBlockHTML(lines, 0, root); !ok || n != 0 {
		t.Errorf("mid-line end: got (%d,%v), want (0,true)", n, ok)
	}
	if len(root.Children) != 1 || root.Children[0].Type != ElHTMLElement {
		t.Errorf("mid-line end must commit the element, got %d children", len(root.Children))
	}
	if lines[0] != " y" {
		t.Errorf("mid-line end must re-inject the remainder, got %q", lines[0])
	}
}

// TestBlockContentModel checks the :parse_block_html block content model end to end:
// a comment interrupting Markdown blocks inside an element, a stray non-matching close
// tag falling through to a paragraph (escaped), and the surrounding indentation — all
// against the kramdown 2.5.2 gem's output.
func TestBlockContentModel(t *testing.T) {
	o := DefaultOptions()
	o.AutoIds = false
	o.ParseBlockHTML = true
	cases := []struct{ name, in, want string }{
		{"comment_only", "<div>\n<!-- c -->\n</div>\n", "<div>\n  <!-- c -->\n</div>\n"},
		{"comment_between", "<div>\ntext\n<!-- x -->\nmore\n</div>\n",
			"<div>\n  <p>text</p>\n  <!-- x -->\n  <p>more</p>\n</div>\n"},
		{"stray_close", "<div>\n</foo>\ntext\n</div>\n", "<div>\n  <p>&lt;/foo&gt;\ntext</p>\n</div>\n"},
		{"empty", "<div></div>\n", "<div></div>\n"},
		{"auto_close", "<div>\nfoo\n", "<div>\n  <p>foo</p>\n</div>\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToHTML(tc.in, &o); got != tc.want {
				t.Errorf("ToHTML(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestBlockContentModelHelpers exercises the block content model helper predicates'
// edge branches directly, including the input-end (no trailing newline) region path.
func TestBlockContentModelHelpers(t *testing.T) {
	p := newParser("", DefaultOptions())
	el := newEl(ElHTMLElement)
	el.Value = "div"

	// collectMarkdownRegion reaching end of input without a trailing newline.
	hp := &htmlParser{p: p, sc: &htmlScanner{s: "foo\nbar"}}
	if reg, adv := hp.collectMarkdownRegion(el); reg != "foo\nbar" || adv != 7 {
		t.Errorf("collectMarkdownRegion = (%q,%d), want (\"foo\\nbar\",7)", reg, adv)
	}

	// matchCloseTagFor: span-element close, mismatched name, upper-case fold, OPT_SPACE
	// prefix with trailing content, non-close text.
	if _, ok := matchCloseTagFor("</span>", el); ok {
		t.Error("span-element close must not match")
	}
	if _, ok := matchCloseTagFor("</foo>", el); ok {
		t.Error("mismatched close must not match")
	}
	if n, ok := matchCloseTagFor("</DIV>", el); !ok || n != 6 {
		t.Errorf("upper-case close: got (%d,%v), want (6,true)", n, ok)
	}
	if n, ok := matchCloseTagFor("  </div> x", el); !ok || n != 8 {
		t.Errorf("indented close: got (%d,%v), want (8,true)", n, ok)
	}
	if _, ok := matchCloseTagFor("plain text", el); ok {
		t.Error("non-close text must not match")
	}

	// boundaryAt: a column-0 comment and a plain line.
	if !(&htmlParser{p: p, sc: &htmlScanner{s: "<!-- c -->\n"}}).boundaryAt(0, el) {
		t.Error("column-0 comment must be a boundary")
	}
	if (&htmlParser{p: p, sc: &htmlScanner{s: "plain\n"}}).boundaryAt(0, el) {
		t.Error("plain line must not be a boundary")
	}
}

// TestMatchStartTag covers the tag matcher's accept and reject branches, including the
// quote/value rejections RE2 cannot express directly.
func TestMatchStartTag(t *testing.T) {
	accept := []struct {
		in        string
		name      string
		selfClose bool
		n         int
	}{
		{`<a href="x">`, "a", false, 12},
		{`<br/>`, "br", true, 5},
		{`<p class >z`, "p", false, 10},
	}
	for _, tc := range accept {
		name, _, sc, n, ok := matchStartTag(tc.in)
		if !ok || name != tc.name || sc != tc.selfClose || n != tc.n {
			t.Errorf("matchStartTag(%q) = (%q,%v,%d,%v)", tc.in, name, sc, n, ok)
		}
	}
	reject := []string{
		``,           // empty
		`<1>`,        // name must start with a letter/underscore
		`<a b="c>`,   // unterminated quoted value
		`<a b=>`,     // '=' with no value
		`<http://x>`, // not a well-formed tag (kramdown falls through to text)
		`<a `,        // never closes
	}
	for _, in := range reject {
		if _, _, _, _, ok := matchStartTag(in); ok {
			t.Errorf("matchStartTag(%q) accepted, want reject", in)
		}
	}
}

// TestParseHTMLAttributes covers name lower-casing, unquoted values, duplicate
// overwrite, valueless attributes and the unterminated-quote fallback.
func TestParseHTMLAttributes(t *testing.T) {
	eq := func(got []Attr, want []Attr) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	cases := []struct {
		in    string
		inTag bool
		want  []Attr
	}{
		{` cLaSs="A" lang=en`, true, []Attr{{"class", "A"}, {"lang", "en"}}},
		{` NamE:SpAC='v'`, false, []Attr{{"NamE:SpAC", "v"}}},
		{` a="1" a="2"`, true, []Attr{{"a", "2"}}},
		{` class`, true, []Attr{{"class", ""}}},
		{` a="b`, true, []Attr{{"a", "b"}}}, // unterminated: consume the rest as value
		{` !bad`, true, nil},                // no attribute name to scan
	}
	for _, tc := range cases {
		if got := parseHTMLAttributes(tc.in, tc.inTag); !eq(got, tc.want) {
			t.Errorf("parseHTMLAttributes(%q,%v) = %v, want %v", tc.in, tc.inTag, got, tc.want)
		}
	}
}

// TestHTMLScannerPrimitives covers the scanner helpers' edge branches.
func TestHTMLScannerPrimitives(t *testing.T) {
	sc := &htmlScanner{s: "ab"}
	if sc.getch() != "a" || sc.getch() != "b" {
		t.Fatal("getch sequence")
	}
	if !sc.eos() || sc.getch() != "" {
		t.Fatal("getch past end must be empty")
	}
	if _, ok := (&htmlScanner{s: "no tags here"}).scanUntilRawStart(); ok {
		t.Error("scanUntilRawStart must report no match")
	}
}
