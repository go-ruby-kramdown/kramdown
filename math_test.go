// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestMathEngineVariants covers the math-rendering branches the vendored corpus
// does not reach: an inline/block math element carrying IAL attributes under both
// the default MathJax engine and the disabled (kdmath) engine, and a
// leading-backslash "\$$…$$" whose line does not end cleanly (so the backslash is
// left in place and the whole line stays literal). Each expected value is byte-exact
// against the kramdown 2.5.2 gem.
func TestMathEngineVariants(t *testing.T) {
	mathjax := DefaultOptions() // MathEngine "mathjax"
	none := DefaultOptions()    // MathEngine disabled
	none.MathEngine = ""

	cases := []struct {
		name string
		src  string
		opts *Options
		want string
	}{
		{
			// MathJax inline math with a span IAL: <span class="c">$raw$</span>.
			name: "mathjax_span_attr",
			src:  "x $$a$${:.c} y",
			opts: &mathjax,
			want: "<p>x <span class=\"c\">$a$</span> y</p>\n",
		},
		{
			// Disabled engine, inline math with a span IAL: the class is merged with
			// "kdmath" and the raw LaTeX kept as "$a$".
			name: "no_engine_span_attr",
			src:  "x $$a$${:.c}",
			opts: &none,
			want: "<p>x <span class=\"c kdmath\">$a$</span></p>\n",
		},
		{
			// Disabled engine, block math with a block IAL: class merged with "kdmath".
			name: "no_engine_block_attr",
			src:  "{:.cls}\n$$5+5$$",
			opts: &none,
			want: "<div class=\"cls kdmath\">$$\n5+5\n$$</div>\n",
		},
		{
			// A leading "\$$…$$" whose line has trailing text does not end cleanly:
			// kramdown keeps the backslash, so the whole line is literal paragraph text.
			name: "backslash_not_clean_end",
			src:  "\\$$5+5$$ tail",
			opts: &mathjax,
			want: "<p>$$5+5$$ tail</p>\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToHTML(tc.src, tc.opts); got != tc.want {
				t.Errorf("ToHTML(%q):\n got %q\nwant %q", tc.src, got, tc.want)
			}
		})
	}
}
