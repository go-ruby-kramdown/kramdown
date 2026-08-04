// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestHandleRawHTMLTag covers kramdown's handle_raw_html_tag for script/style: a
// verbatim (unescaped) body, a case-insensitive close tag, the trailing-newline quirk,
// and the auto-close of an unterminated tag. Each expected value is byte-exact against
// kramdown 2.5.2 (parse_block_html on, auto_ids off).
func TestHandleRawHTMLTag(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// The body is emitted verbatim (nested tags and ">" are not escaped) and the
		// block script/style is followed by the extra blank line of the quirk.
		{"verbatim_nested", "<script>a<p>b</p></script>", "<script>a<p>b</p></script>\n\n"},
		{"style_gt", "<style>x > y</style>", "<style>x > y</style>\n\n"},
		{"ci_close", "<script>keep *raw*\n</SCRIPT>", "<script>keep *raw*\n</script>\n\n"},
		// An unterminated tag auto-closes at end of input, keeping its body verbatim.
		{"unterminated", "<script>unterminated body *x*", "<script>unterminated body *x*\n</script>\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := DefaultOptions()
			o.AutoIds = false
			o.ParseBlockHTML = true
			if got := ToHTML(tc.src, &o); got != tc.want {
				t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
			}
		})
	}
}
