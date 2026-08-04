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

// TestParseBlockHTMLBailouts exercises the two paths where the line-based block driver
// declines to consume: a span-only element (never a block construct) and a construct
// that ends mid-line (trailing content after the close tag), which is deferred to the
// block/span content-model cluster.
func TestParseBlockHTMLBailouts(t *testing.T) {
	p := newParser("", DefaultOptions())
	if n, ok := p.parseBlockHTML([]string{"<em>x</em>"}, 0, newEl(ElRoot)); ok || n != 0 {
		t.Errorf("span element: got (%d,%v), want (0,false)", n, ok)
	}
	root := newEl(ElRoot)
	if n, ok := p.parseBlockHTML([]string{"<div>x</div> y"}, 0, root); ok || n != 0 {
		t.Errorf("mid-line end: got (%d,%v), want (0,false)", n, ok)
	}
	if len(root.Children) != 0 {
		t.Errorf("mid-line bail must not commit children, got %d", len(root.Children))
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
