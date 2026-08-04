// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestLinkDefinitionEdges covers matchLinkDef's remaining branches. The first three
// (angle URL with a next-line title, an empty URL that is not a definition, and the
// bare-URL "space before a quote" guard) are byte-for-byte identical to kramdown
// 2.5.2. The last two document the fall-through to a bare URL when "<…>" is
// unterminated or trailed by non-title text; kramdown agrees on the URL text but
// additionally escapes ">" as "&gt;" in the href (a pre-existing href-escaping
// detail unrelated to link-definition parsing and not exercised by the corpus).
func TestLinkDefinitionEdges(t *testing.T) {
	cases := []struct{ in, want string }{
		// Angle URL (with a space) and its title on the next line.
		{
			"[a]: <u v.html>\n     \"t\"\n\nsee [a]\n",
			"\n<p>see <a href=\"u v.html\" title=\"t\">a</a></p>\n",
		},
		// An empty URL is not a definition; the marker stays literal.
		{
			"[a]:\n\n[a] here\n",
			"<p>[a]:</p>\n\n<p>[a] here</p>\n",
		},
		// A bare URL followed by whitespace and an (unbalanced) quote is invalidated by
		// the space-before-quote guard, so it stays a paragraph.
		{
			"[a]: url \"unclosed\n\nsee [a]\n",
			"<p>[a]: url “unclosed</p>\n\n<p>see [a]</p>\n",
		},
		// Even when a trailing title parses, the guard still rejects a URL part that
		// itself contains a space-before-quote (here "b 'c'").
		{
			"[a]: b 'c' \"d\"\n\nsee [a]\n",
			"<p>[a]: b ‘c’ “d”</p>\n\n<p>see [a]</p>\n",
		},
		// An unterminated "<…>" falls through to a bare URL (the "<" is literal).
		{
			"[a]: <unclosed\n\nsee [a]\n",
			"\n<p>see <a href=\"&lt;unclosed\">a</a></p>\n",
		},
		// A closed "<…>" trailed by non-title text also falls back to a bare URL that
		// spans the whole remainder.
		{
			"[a]: <u.html> extra words\n\nsee [a]\n",
			"\n<p>see <a href=\"&lt;u.html> extra words\">a</a></p>\n",
		},
	}
	for _, c := range cases {
		if got := ToHTML(c.in, nil); got != c.want {
			t.Errorf("ToHTML(%q):\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}
