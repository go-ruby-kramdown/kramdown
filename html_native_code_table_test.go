// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// nativeNoHL renders src with the :html_to_native transform and the syntax
// highlighter disabled (so native code blocks take the plain <pre><code> path), the
// configuration the code/table edge cases below are verified against kramdown 2.5.2.
func nativeNoHL(src string) string {
	o := DefaultOptions()
	o.AutoIds = false
	o.HtmlToNative = true
	o.SyntaxHighlighter = ""
	return ToHTML(src, &o)
}

// TestNativeCodeConversion exercises convert_code / convert_pre across the branches the
// corpus code case does not reach, each verified byte-for-byte against kramdown 2.5.2.
func TestNativeCodeConversion(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// A <pre> wrapping a lone <code class="language-x"> hoists the language onto the
		// emitted <code>, and the block still degrades to plain <pre><code>.
		{"pre_code_lang", `<pre><code class="language-ruby">x = 1</code></pre>`,
			"<pre><code class=\"language-ruby\">x = 1\n</code></pre>\n"},
		// An empty inline <code> collapses to an empty code span.
		{"code_empty", "a <code></code> b", "<p>a <code></code> b</p>\n"},
		// Numeric, hex and the built-in named entities decode to their characters, which
		// the renderer re-escapes; an unknown named entity keeps its literal form.
		{"code_entities", "x <code>a &amp; b &#65; &#x42; &unknown;</code> y",
			"<p>x <code>a &amp; b A B &amp;unknown;</code> y</p>\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nativeNoHL(tc.src); got != tc.want {
				t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestNativeTableConversion exercises is_simple_table? and convert_table across the
// simple/non-simple decision branches, each verified byte-for-byte against kramdown
// 2.5.2 (a non-simple table serialises verbatim).
func TestNativeTableConversion(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// A cell holding a block element is not phrasing content -> not simple -> raw.
		{"cell_block", "<table>\n<tr><td><div>x</div></td></tr>\n</table>",
			"<table>\n<tr><td><div>x</div></td></tr>\n</table>\n"},
		// Rows with differing cell counts -> not simple -> raw.
		{"ragged", "<table>\n<tr><td>a</td><td>b</td></tr>\n<tr><td>c</td></tr>\n</table>",
			"<table>\n<tr><td>a</td><td>b</td></tr>\n<tr><td>c</td></tr>\n</table>\n"},
		// A justify alignment disqualifies the table -> raw.
		{"justify", `<table>` + "\n" + `<tr><td style="text-align: justify">a</td></tr>` + "\n</table>",
			"<table>\n<tr><td style=\"text-align: justify\">a</td></tr>\n</table>\n"},
		// Rows whose alignment disagrees -> raw.
		{"mixed_align", `<table>` + "\n" + `<tr><td style="text-align: left">a</td></tr>` + "\n" +
			`<tr><td style="text-align: right">b</td></tr>` + "\n</table>",
			"<table>\n<tr><td style=\"text-align: left\">a</td></tr>\n" +
				"<tr><td style=\"text-align: right\">b</td></tr>\n</table>\n"},
		// A thead with no tbody is not a valid sectioned simple table -> raw.
		{"thead_only", "<table>\n<thead>\n<tr><th>a</th></tr>\n</thead>\n</table>",
			"<table>\n<thead>\n<tr><th>a</th></tr>\n</thead>\n</table>\n"},
		// A block element nested one level below a phrasing element in a cell is still
		// non-phrasing content -> not simple -> raw (exercises the recursive check).
		{"nested_block", "<table>\n<tr><td><b><div>x</div></b></td></tr>\n</table>",
			"<table>\n<tr><td><b><div>x</div></b></td></tr>\n</table>\n"},
		// An empty simple-table cell renders a non-breaking space.
		{"empty_cell", "<table>\n<tr><td></td><td>b</td></tr>\n<tr><td>c</td><td>d</td></tr>\n</table>",
			"<table>\n  <tbody>\n    <tr>\n      <td> </td>\n      <td>b</td>\n    </tr>\n" +
				"    <tr>\n      <td>c</td>\n      <td>d</td>\n    </tr>\n  </tbody>\n</table>\n"},
		// A simple single-cell table with a residual (non-text-align) style keeps the
		// remaining style and appends the column's text-align.
		{"style_plus_align", `<table>` + "\n" + `<tr><td style="width: 5em; text-align: left">a</td></tr>` + "\n</table>",
			"<table>\n  <tbody>\n    <tr>\n      <td style=\"width: 5em; text-align: left\">a</td>\n    </tr>\n  </tbody>\n</table>\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nativeNoHL(tc.src); got != tc.want {
				t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestNativeCodeHelpers covers the pure decode/entity helpers directly, including the
// error and out-of-range arms the integration cases cannot reach.
func TestNativeCodeHelpers(t *testing.T) {
	// decodeCodeText: plain text, numeric, hex, and the unknown-entity amp fallback.
	if got := decodeCodeText("plain"); got != "plain" {
		t.Errorf("decodeCodeText plain = %q", got)
	}
	if got := decodeCodeText("&#65;&#x42;&lt;"); got != "AB<" {
		t.Errorf("decodeCodeText nums = %q", got)
	}
	if got := decodeCodeText("a &nope; b"); got != "a &nope; b" {
		t.Errorf("decodeCodeText unknown = %q", got)
	}

	// codePointChar: valid, malformed digits, and out-of-range code points.
	if s, ok := codePointChar("65", 10); !ok || s != "A" {
		t.Errorf("codePointChar 65 = %q,%v", s, ok)
	}
	if _, ok := codePointChar("zz", 16); ok {
		t.Error("codePointChar malformed should fail")
	}
	if _, ok := codePointChar("9999999999", 10); ok {
		t.Error("codePointChar overflow should fail")
	}
	if _, ok := codePointChar("1114112", 10); ok { // one past unicode.MaxRune
		t.Error("codePointChar > MaxRune should fail")
	}

	// namedCodeEntity: every built-in, a symChar-backed name, and an unknown name.
	for name, want := range map[string]string{
		"lt": "<", "gt": ">", "amp": "&", "quot": "\"", "apos": "'", "nbsp": "\u00a0", "hellip": "\u2026",
	} {
		if got, ok := namedCodeEntity(name); !ok || got != want {
			t.Errorf("namedCodeEntity(%q) = %q,%v want %q", name, got, ok, want)
		}
	}
	if _, ok := namedCodeEntity("nope"); ok {
		t.Error("namedCodeEntity unknown should fail")
	}

	// chompRecordSeparator: CRLF, LF, CR and no trailing separator.
	for in, want := range map[string]string{"a\r\n": "a", "a\n": "a", "a\r": "a", "a": "a"} {
		if got := chompRecordSeparator(in); got != want {
			t.Errorf("chompRecordSeparator(%q) = %q want %q", in, got, want)
		}
	}

	// stringSliceEqual: unequal length, unequal element, and equal.
	if stringSliceEqual([]string{"a"}, []string{"a", "b"}) {
		t.Error("stringSliceEqual length mismatch")
	}
	if stringSliceEqual([]string{"a"}, []string{"b"}) {
		t.Error("stringSliceEqual element mismatch")
	}
	if !stringSliceEqual([]string{"a", "b"}, []string{"a", "b"}) {
		t.Error("stringSliceEqual equal")
	}
}
