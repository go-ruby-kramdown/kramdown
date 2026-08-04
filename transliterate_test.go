// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestTransliterate exercises the unidecoder decode paths, including the empty-input
// short-circuit, ASCII passthrough, a present block, and the missing-block "?" path
// (an astral codepoint whose high byte exceeds 0xff has no vendored block).
func TestTransliterate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},                         // empty short-circuit
		{"plain ASCII", "plain ASCII"},   // all-ASCII passthrough
		{"©", "(c)"},                     // Latin-1 block
		{"Đây-là-ví-dụ", "Day-la-vi-du"}, // Vietnamese, present blocks
		{"λ", "l"},                       // Greek block
		{"😀", "?"},                       // astral: missing block -> "?"
		{"a😀b", "a?b"},                   // astral between ASCII
	}
	for _, c := range cases {
		if got := transliterate(c.in); got != c.want {
			t.Errorf("transliterate(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
