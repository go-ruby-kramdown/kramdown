// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestOutputHeaderLevelClamp exercises both clamps of output_header_level, including
// the negative-offset lower clamp not reached by the corpus (its offset is +1).
func TestOutputHeaderLevelClamp(t *testing.T) {
	cases := []struct {
		level, offset, want int
	}{
		{1, 0, 1},
		{3, 1, 4},
		{6, 1, 6},  // upper clamp: 7 -> 6
		{2, 5, 6},  // upper clamp
		{1, -1, 1}, // lower clamp: 0 -> 1
		{2, -5, 1}, // lower clamp
		{3, -1, 2}, // no clamp
	}
	for _, c := range cases {
		if got := outputHeaderLevel(c.level, c.offset); got != c.want {
			t.Errorf("outputHeaderLevel(%d,%d)=%d want %d", c.level, c.offset, got, c.want)
		}
	}
}

// TestHeaderOffsetNegative confirms a negative header_offset clamps h1 to <h1> in the
// rendered output (the lower-clamp path end-to-end).
func TestHeaderOffsetNegative(t *testing.T) {
	o := DefaultOptions()
	o.AutoIds = false
	o.HeaderOffset = -2
	got := ToHTML("## Two\n", &o)
	if want := "<h1>Two</h1>\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
