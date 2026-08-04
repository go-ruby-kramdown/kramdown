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
// tag raw and keeps parsing the remainder (an unterminated-tag edge case that the
// gem instead auto-closes; not exercised by the corpus).
func TestSpanHTMLNoCloseFallthrough(t *testing.T) {
	got := ToHTML(`x {::options parse_span_html="false" /} <span>*y* end`, nil)
	want := "<p>x  <span><em>y</em> end</p>\n"
	if got != want {
		t.Errorf("unterminated raw span:\n got %q\nwant %q", got, want)
	}
}

// TestAbbrevAcrossUnicodeAndNewlineWhitespace covers abbreviation matching where a
// keyword space spans a no-break space (U+00A0, a Unicode separator) and a soft
// newline, mirroring kramdown's [\s\p{Z}]+ whitespace class. A document that opens
// with an abbreviation definition renders a leading newline exactly as the gem does.
func TestAbbrevAcrossUnicodeAndNewlineWhitespace(t *testing.T) {
	// No-break space (U+00A0) between the two words of the abbreviation.
	got := ToHTML("*[foo bar]: baz\n\nhello foo\u00a0bar babble.\n", nil)
	want := "\n<p>hello <abbr title=\"baz\">foo\u00a0bar</abbr> babble.</p>\n"
	if got != want {
		t.Errorf("abbrev across nbsp:\n got %q\nwant %q", got, want)
	}
	// Soft newline between the two words.
	got = ToHTML("*[foo bar]: baz\n\nhello foo\nbar babble.\n", nil)
	want = "\n<p>hello <abbr title=\"baz\">foo\nbar</abbr> babble.</p>\n"
	if got != want {
		t.Errorf("abbrev across newline:\n got %q\nwant %q", got, want)
	}
}
