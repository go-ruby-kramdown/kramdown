// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestHTMLToNative exercises the :html_to_native ElementConverter and its dedicated
// renderer across the element kinds the corpus does not reach, each expected value
// verified byte-for-byte against kramdown 2.5.2 (auto_ids off, html_to_native on).
func TestHTMLToNative(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// convert_a: an href-bearing anchor becomes a native :a; a bare anchor stays raw.
		{"a_href", `See <a href="u">link</a> here`, "<p>See <a href=\"u\">link</a> here</p>\n"},
		{"a_nohref", `See <a name="x">anch</a> here`, "<p>See <a name=\"x\">anch</a> here</p>\n"},
		// SIMPLE_ELEMENTS containers rendered by the dedicated block renderer.
		{"blockquote", "<blockquote>quoted</blockquote>", "<blockquote>\nquoted</blockquote>\n"},
		{"ol_li", "<ol>\n<li>one</li>\n</ol>", "<ol>\n  <li>one</li>\n</ol>\n"},
		{"dl", "<dl>\n<dt>term</dt>\n<dd>desc</dd>\n</dl>", "<dl>\n  <dt>term</dt>\n  <dd>desc</dd>\n</dl>\n"},
		{"hr", "para\n\n<hr>\n\nmore", "<p>para</p>\n\n<hr />\n\n<p>more</p>\n"},
		{"empty_li", "<ul>\n<li></li>\n</ul>", "<ul>\n  <li></li>\n</ul>\n"},
		{"li_mixed", "<ul>\n<li>lead<p>para</p></li>\n</ul>", "<ul>\n  <li>lead    <p>para</p>\n  </li>\n</ul>\n"},
		{"dd_mixed", "<dl>\n<dt>t</dt>\n<dd>lead<p>p</p></dd>\n</dl>", "<dl>\n  <dt>t</dt>\n  <dd>lead    <p>p</p>\n  </dd>\n</dl>\n"},
		// convert_html_element (block category, non-raw): a span content model renders
		// inline; an empty body closes with a bare tag pair.
		{"caption", "<caption>Cap</caption>", "<caption>Cap</caption>\n"},
		{"caption_empty", "<caption></caption>", "<caption></caption>\n"},
		// convert_em whitespace rule: leading/trailing space (or an empty body) keeps the
		// element raw rather than converting it to :em.
		{"em_trailing", "text <em>a </em> end", "<p>text <em>a </em> end</p>\n"},
		{"em_leading", "text <em> a</em> end", "<p>text <em> a</em> end</p>\n"},
		{"em_empty", "text <em></em> end", "<p>text <em></em> end</p>\n"},
		// strip_whitespace trims the first and last text children of a converted <p>.
		{"strip_pp", "<p> a <em>b</em> c </p>", "<p>a <em>b</em> c</p>\n"},
		// A <div> whose comments/paragraphs are converted renders child-based.
		{"div_para", "<div>\n  <p>x</p>\n</div>", "<div>\n  <p>x</p>\n</div>\n"},
		// A comment nested in a converted header / emphasis serialises inline.
		{"h_comment", "<h1>hi<!--c--></h1>", "<h1>hi<!--c--></h1>\n"},
		{"em_comment", "para <em>x<!--c-->y</em> z", "<p>para <em>x<!--c-->y</em> z</p>\n"},
		// A top-level comment (parent nil -> "div") stays a block comment on its own line.
		{"top_comment", "<!--top-->", "<!--top-->\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := DefaultOptions()
			o.AutoIds = false
			o.HtmlToNative = true
			if got := ToHTML(tc.src, &o); got != tc.want {
				t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestHTMLToNativeInlineOption covers enabling :html_to_native through an inline
// "{::options html_to_native=…}" extension, verified byte-exact against kramdown 2.5.2.
func TestHTMLToNativeInlineOption(t *testing.T) {
	o := DefaultOptions()
	o.AutoIds = false
	got := ToHTML("{::options html_to_native=\"true\" /}\n\ntext <b>bold</b> here", &o)
	want := "\n<p>text <strong>bold</strong> here</p>\n"
	if got != want {
		t.Errorf("inline html_to_native:\n got %q\nwant %q", got, want)
	}
}

// TestConvertNativeBlockDefault covers convertNativeBlock's defensive default arm: a
// node type the converted tree never actually produces yields the empty string.
func TestConvertNativeBlockDefault(t *testing.T) {
	c := newHTMLConverter(New("", nil))
	if got := c.convertNativeBlock(newEl(ElImg), 0); got != "" {
		t.Errorf("default arm = %q, want empty", got)
	}
}
