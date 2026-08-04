// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"strings"
	"testing"
)

// sqSyms renders the smart-quote result for the quote at index q of src as a compact
// string: symbol names joined by "+", with a literal text fragment shown as t"...".
func sqSyms(src string, q int, pa bool, pb byte) (string, int) {
	items, consumed := smartQuoteAt(src, q, pa, pb)
	parts := make([]string, len(items))
	for i, it := range items {
		if it.sym != "" {
			parts[i] = it.sym
		} else {
			parts[i] = "t" + it.text
		}
	}
	return strings.Join(parts, "+"), consumed
}

// TestSmartQuoteAtRules exercises every SQ_RULE branch of smartQuoteAt, at the quote
// position 0 (dispatched at the quote, prev unavailable) or 1 (prev buffered).
func TestSmartQuoteAtRules(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		q        int
		pa       bool
		pb       byte
		wantSyms string
		wantN    int
	}{
		// R1: opening quote before emphasis markers (one and two markers).
		{"r1-one", `"*x`, 0, false, 0, "ldquo", 1},
		{"r1-two", `"**x`, 0, false, 0, "ldquo", 1},
		// R2: closing quote before punctuation (both sides non-word -> \B holds).
		{"r2", `"!)`, 0, false, 0, "rdquo", 1},
		// R2 guard: a following ".." disables it, so a bare "... opens (R10).
		{"r2-dotdot", `"...`, 0, false, 0, "ldquo", 1},
		// R2 boundary: punctuation then a word is a \b boundary -> R2 skipped -> R10.
		{"r2-wordbound", `"!x`, 0, false, 0, "ldquo", 1},
		// R3 / R4: doubled opening quotes.
		{"r3", `"'w`, 0, false, 0, "ldquo+lsquo", 2},
		{"r4", `'"w`, 0, false, 0, "lsquo+ldquo", 2},
		// R3 with a non-space preceding char skips R3, falls to R7 (prev is SQ_CLOSE).
		{"r3-skip", `x"'w`, 1, true, 'x', "rdquo", 1},
		// R5: decade apostrophe; and its skip when preceded by a non-space char.
		{"r5", `'80s`, 0, false, 0, "rsquo", 1},
		{"r5-skip", `x'80s`, 1, true, 'x', "rsquo", 1},
		// R6: opening quote after whitespace before a word.
		{"r6", ` 'w`, 1, true, ' ', "lsquo", 1},
		// R7: closing after a SQ_CLOSE char; skipped after "(" (not SQ_CLOSE) -> R9.
		{"r7", `x'`, 1, true, 'x', "rsquo", 1},
		{"r7-open-paren", `('`, 1, true, '(', "lsquo", 1},
		// R7 doubled at dispatch: first quote literal, second closes.
		{"r7-double", `""x`, 0, false, 0, `t"+rdquo`, 2},
		// R8: closing before space, a bare "s", or end of input; else opens (R10).
		{"r8-space", `" `, 0, false, 0, "rdquo", 1},
		{"r8-end", `"`, 0, false, 0, "rdquo", 1},
		{"r8-sboundary", `"s `, 0, false, 0, "rdquo", 1},
		{"r8-sword", `"sx`, 0, false, 0, "ldquo", 1},
		// R9 / R10: any remaining quote opens (prev is not SQ_CLOSE, space, or a match).
		{"r9", `-'`, 1, true, '-', "lsquo", 1},
		{"r10", `"x`, 0, false, 0, "ldquo", 1},
	}
	for _, c := range cases {
		got, n := sqSyms(c.src, c.q, c.pa, c.pb)
		if got != c.wantSyms || n != c.wantN {
			t.Errorf("%s: smartQuoteAt(%q,%d,%v,%q)=%q,%d want %q,%d",
				c.name, c.src, c.q, c.pa, c.pb, got, n, c.wantSyms, c.wantN)
		}
	}
}

// TestSmartQuoteDisabled covers parseInto's quote path when smart_quotes is off: the
// quote is kept literal.
func TestSmartQuoteDisabled(t *testing.T) {
	o := DefaultOptions()
	o.SmartQuotes = false
	o.AutoIds = false
	if got := ToHTML("say \"hi\"\n", &o); got != "<p>say \"hi\"</p>\n" {
		t.Errorf("smart_quotes off = %q", got)
	}
}

// TestSmartQuoteDoubledLiteral covers the doubled-quote-at-start path where the first
// quote is emitted as literal text and the second closes (R7's dispatch branch).
func TestSmartQuoteDoubledLiteral(t *testing.T) {
	o := DefaultOptions()
	o.AutoIds = false
	if got := ToHTML("\"\"x\n", &o); got != "<p>\"”x</p>\n" {
		t.Errorf("doubled quote = %q", got)
	}
}
