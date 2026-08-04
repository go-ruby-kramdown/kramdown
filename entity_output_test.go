// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestEntityOutputModes locks the :numeric and :symbolic entity_output rendering of
// the typographic-symbol / smart-quote elements, each verified byte-for-byte against
// kramdown 2.5.2. The default :as_char rendering is covered by the corpus suite.
func TestEntityOutputModes(t *testing.T) {
	cases := []struct{ mode, want string }{
		{"numeric", "<p>a&#8230; b&#8211;c &#8220;d&#8221;</p>\n"},
		{"symbolic", "<p>a&hellip; b&ndash;c &ldquo;d&rdquo;</p>\n"},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			o := DefaultOptions()
			o.AutoIds = false
			o.EntityOutput = tc.mode
			if got := ToHTML(`a... b--c "d"`, &o); got != tc.want {
				t.Errorf("%s:\n got %q\nwant %q", tc.mode, got, tc.want)
			}
		})
	}
}
