// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestSpanOptionsExtension covers the span-level {::options …} extension: it folds
// its recognised key="val" pairs into the parser options and renders nothing, both
// self-closing and with a discarded body up to a {:/options} stop tag. Setting
// parse_span_html=false leaves a raw inline element's body unparsed.
func TestSpanOptionsExtension(t *testing.T) {
	cases := []struct{ in, want string }{
		// Self-closing options turning off span-HTML parsing: the <span> body stays
		// literal (matches kramdown 2.5.2).
		{
			`This is an {::options parse_span_html="false" /} option <span>*true*</span>!`,
			"<p>This is an  option <span>*true*</span>!</p>\n",
		},
		// Non-self-closing options with a body and stop tag: the body is discarded and
		// the option still applies for the rest of the document.
		{
			`a {::options parse_span_html="false"}zzz{:/options} <span>*y*</span> b`,
			"<p>a  <span>*y*</span> b</p>\n",
		},
		// A raw element carrying attributes is passed through verbatim when span-HTML
		// parsing is off (drives the tag-name extraction past its attributes).
		{
			`x {::options parse_span_html="false" /} <span class="c">*y*</span> z`,
			"<p>x  <span class=\"c\">*y*</span> z</p>\n",
		},
	}
	for _, c := range cases {
		if got := ToHTML(c.in, nil); got != c.want {
			t.Errorf("ToHTML(%q):\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

// TestSpanHTMLNoCloseFallthrough covers the fall-through when span-HTML parsing is
// off but an opening element has no matching close tag: the port emits the opening
// tag raw and keeps parsing the remainder (the idx < 0 branch). This is an
// unterminated-tag edge case the gem instead auto-closes at the paragraph boundary
// (keeping the body raw); it is not exercised by the corpus.
func TestSpanHTMLNoCloseFallthrough(t *testing.T) {
	got := ToHTML(`x {::options parse_span_html="false" /} <span>*y* end`, nil)
	want := "<p>x  <span><em>y</em> end</p>\n"
	if got != want {
		t.Errorf("unterminated raw span:\n got %q\nwant %q", got, want)
	}
}
