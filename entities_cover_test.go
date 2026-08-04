// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestEntityLookupInvalidNumeric covers numericEntity's rejection branches: a numeric
// reference that overflows int32 and one that names a code point above U+10FFFF both
// fail to resolve, so ToHTML applies kramdown's amp fallback (the leading "&" is
// escaped and the remainder kept literally).
func TestEntityLookupInvalidNumeric(t *testing.T) {
	for _, base := range []int{10, 16} {
		if _, _, ok := numericEntity("99999999999999", base); ok {
			t.Errorf("base %d: overflow should not resolve", base)
		}
	}
	// U+110000 (one past the last legal code point) in decimal and hex.
	if _, _, ok := numericEntity("1114112", 10); ok {
		t.Error("decimal 1114112 should be rejected as out of range")
	}
	if _, _, ok := numericEntity("110000", 16); ok {
		t.Error("hex 110000 should be rejected as out of range")
	}
	o := DefaultOptions()
	o.AutoIds = false
	if got := ToHTML("a &#1114112; b\n", &o); got != "<p>a &amp;#1114112; b</p>\n" {
		t.Errorf("out-of-range decimal fallback = %q", got)
	}
}

// TestEntityOutputAsInput locks the :as_input mode: a resolvable entity keeps its
// literal input form while a bare or invalid "&" is escaped, verified against the gem.
func TestEntityOutputAsInput(t *testing.T) {
	o := DefaultOptions()
	o.AutoIds = false
	o.EntityOutput = "as_input"
	got := ToHTML("A&O &copy; \\& &#x3bb; &bogus;\n", &o)
	want := "<p>A&amp;O &copy; \\&amp; &#x3bb; &amp;bogus;</p>\n"
	if got != want {
		t.Errorf("as_input:\n got %q\nwant %q", got, want)
	}
}

// TestSmartQuoteSubstitution covers the :smart_quotes option array wiring: overriding
// the four positions with apos/quot renders straight quotes, exactly as the gem does.
func TestSmartQuoteSubstitution(t *testing.T) {
	o := DefaultOptions()
	o.AutoIds = false
	o.SmartQuotesSubst = [4]string{"apos", "apos", "quot", "quot"}
	if got := ToHTML("\"a\" 'b'\n", &o); got != "<p>\"a\" 'b'</p>\n" {
		t.Errorf("smart_quotes subst = %q", got)
	}
	// A numeric entity_output still numbers a smart quote whose char is not escaped.
	o.EntityOutput = "numeric"
	o.SmartQuotesSubst = [4]string{}
	if got := ToHTML("'x'\n", &o); got != "<p>&#8216;x&#8217;</p>\n" {
		t.Errorf("smart_quotes numeric = %q", got)
	}
}
