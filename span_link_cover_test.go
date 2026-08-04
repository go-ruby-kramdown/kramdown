// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestLinkFootnoteMarkerInText covers matchBracket / footnoteMarkerEnd rejecting a
// non-marker "[^ ]" (a space is neither word nor hyphen), so the brackets balance
// normally and the link text keeps the literal "[^ ]" — byte-exact with the gem.
func TestLinkFootnoteMarkerInText(t *testing.T) {
	eq(t, "[a [^ ] b](u)\n", "<p><a href=\"u\">a [^ ] b</a></p>\n")
}

// TestAngleDestNoCloseNoTitle covers inlineLink's angle-destination fallback: when
// "(<url>" is followed by neither ")" nor a valid title the link fails and the text
// stays literal — byte-exact with the gem.
func TestAngleDestNoCloseNoTitle(t *testing.T) {
	eq(t, "[t](<3>x)\n", "<p>[t](&lt;3&gt;x)</p>\n")
}

// TestLinkTitleTrailingSpace covers parseInlineTitle's whitespace-before-')'
// scan (the "\s*?\)" tail): "(u \"ti\" )" still yields title "ti".
func TestLinkTitleTrailingSpace(t *testing.T) {
	eq(t, "[t](u \"ti\" )\n", "<p><a href=\"u\" title=\"ti\">t</a></p>\n")
}

// TestImageAltEscapeReduction covers reduceEscaped / isEscapedChar over an image's
// raw alt: "\*" reduces to "*" (escapable) while "\q" is kept verbatim (not
// escapable), exactly as kramdown's ESCAPED_CHARS gsub.
func TestImageAltEscapeReduction(t *testing.T) {
	eq(t, "![a\\*b\\qc](u)\n", "<p><img src=\"u\" alt=\"a*b\\qc\" /></p>\n")
}

// TestReferenceImageWithIAL covers buildLink's image branch applying a reference
// definition's IAL before src/alt, matching the gem's add_link attribute order.
func TestReferenceImageWithIAL(t *testing.T) {
	eq(t, "![a][x]\n\n[x]: u.png\n{: .c}\n", "<p><img class=\"c\" src=\"u.png\" alt=\"a\" /></p>\n\n")
}

// TestLinkDefTrailingALDStops covers harvestDefinitions' trailing-IAL scan
// stopping at an ALD definition ("{:name: …}") rather than swallowing it as the
// link definition's IAL.
func TestLinkDefTrailingALDStops(t *testing.T) {
	eq(t, "[x]: u\n{:name: .c}\n\nSee [x]\n", "\n<p>See <a href=\"u\">x</a></p>\n")
}

// TestJoinIAL covers all three arms of joinIAL, including the empty-second-operand
// arm that the parser reaches only for an empty "{:}" IAL body.
func TestJoinIAL(t *testing.T) {
	if got := joinIAL("", "b"); got != "b" {
		t.Errorf("joinIAL empty-a = %q", got)
	}
	if got := joinIAL("a", ""); got != "a" {
		t.Errorf("joinIAL empty-b = %q", got)
	}
	if got := joinIAL("a", "b"); got != "a b" {
		t.Errorf("joinIAL both = %q", got)
	}
}

// TestCollectLinkDefIALStopsAtALD covers collectLinkDefIAL breaking out when a
// later line in the leading-IAL run is an ALD definition, so the run is not treated
// as a link definition's IAL.
func TestCollectLinkDefIALStopsAtALD(t *testing.T) {
	lines := []string{"{: .a}", "{:name: .c}", "[y]: u"}
	if j, attrs, ok := collectLinkDefIAL(lines, 0); ok || j != 0 || attrs != "" {
		t.Errorf("collectLinkDefIAL = %d,%q,%v; want 0,\"\",false", j, attrs, ok)
	}
}
