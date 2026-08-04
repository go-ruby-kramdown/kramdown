package kramdown

import (
	"strings"
	"testing"
)

// TestCoverDefensiveBranches exercises the small defensive branches that the
// rendering corpus does not reach on its own (unknown symbol name, empty-string
// guards, the splitSub pre-sym short-circuit, and a fully-consumed IAL).
func TestCoverDefensiveBranches(t *testing.T) {
	// symChar fallthrough for an unknown entity name -> "".
	if got := symChar("definitely-not-a-symbol"); got != "" {
		t.Errorf("symChar(unknown) = %q, want empty", got)
	}

	// lastRune of the empty string -> "".
	if got := lastRune(""); got != "" {
		t.Errorf("lastRune(%q) = %q, want empty", "", got)
	}

	// substituteText early-returns on an empty run.
	c := &htmlConverter{}
	if els, last := c.substituteText("", ""); els != nil || last != "" {
		t.Errorf("substituteText(\"\") = %v, %q; want nil, \"\"", els, last)
	}

	// splitSub passes parts that already carry a sym straight through.
	in := []typoPart{{sym: "mdash"}, {text: "a...b"}}
	out := splitSub(in, reEllipsis, "hellip")
	if len(out) == 0 || out[0].sym != "mdash" {
		t.Errorf("splitSub did not preserve the pre-sym part: %+v", out)
	}

	// parseIAL consuming every token drives the loop's trim-to-empty exit.
	toks := parseIAL(`.cls #id key="v"`, nil)
	if len(toks) != 3 {
		t.Errorf("parseIAL token count = %d, want 3 (%+v)", len(toks), toks)
	}
}

// TestCoverIndentedCodeWithBlank covers the blank-line lookahead in
// parseIndentedCode: a blank line between two indented code blocks is folded
// into a single code block.
func TestCoverIndentedCodeWithBlank(t *testing.T) {
	got := ToHTML("    code1\n\n    code2\n", nil)
	if !strings.Contains(got, "code1") || !strings.Contains(got, "code2") {
		t.Errorf("indented code with interior blank lost content: %q", got)
	}

	// An indented code block ending in an indented blank line drives the
	// trailing-blank-trim loop.
	got = ToHTML("    code1\n    \n", nil)
	if !strings.Contains(got, "code1") || strings.Contains(got, "code1\n\n") {
		t.Errorf("trailing blank not trimmed from code block: %q", got)
	}
}

// TestCoverStandaloneImageNoAttrs renders a standalone image whose IAL carries
// neither a class nor an id, driving the not-present return of takeAttr for both
// attributes so the <figure> stays attribute-free (matching kramdown 2.5.2).
func TestCoverStandaloneImageNoAttrs(t *testing.T) {
	opts := DefaultOptions()
	opts.AutoIds = false
	got := ToHTML("![alt txt](pic.jpg){:standalone}", &opts)
	want := "<figure>\n  <img src=\"pic.jpg\" alt=\"alt txt\" />\n  <figcaption>alt txt</figcaption>\n</figure>\n"
	if got != want {
		t.Errorf("standalone image without id/class:\n got %q\nwant %q", got, want)
	}
}

// TestCoverListHelpers exercises the branch space of the low-level list-parsing
// helpers directly, including the defensive and rarely-reached edge branches
// (all-space input, tab expansion, unindented lazy exclusions).
func TestCoverListHelpers(t *testing.T) {
	if got := siblingMax(0); got != 0 { // indentation-1 < 0 clamps to 0
		t.Errorf("siblingMax(0)=%d want 0", got)
	}
	if got := siblingMax(2); got != 1 {
		t.Errorf("siblingMax(2)=%d want 1", got)
	}
	if got := siblingMax(9); got != 3 { // caps at 3
		t.Errorf("siblingMax(9)=%d want 3", got)
	}

	if got := expandLeadingTabs("abc", 1); got != "abc" { // no leading tab
		t.Errorf("expandLeadingTabs abc = %q", got)
	}
	if got := expandLeadingTabs("   ", 1); got != "   " { // all spaces, no tab
		t.Errorf("expandLeadingTabs spaces = %q", got)
	}
	if got := expandLeadingTabs("\t\tx", 0); got != strings.Repeat(" ", 8)+"x" {
		t.Errorf("expandLeadingTabs two tabs = %q", got)
	}
	if got := expandLeadingTabs(" \t x", 1); got != "    x" {
		t.Errorf("expandLeadingTabs space-tab = %q", got)
	}

	if !matchesContentRe("    x", 4) { // exactly one 4-space unit
		t.Error("matchesContentRe 4-space unit")
	}
	if !matchesContentRe("\tx", 2) { // tab counts as a full unit (alt2)
		t.Error("matchesContentRe tab unit")
	}
	if matchesContentRe("x", 4) { // not indented
		t.Error("matchesContentRe unindented")
	}
	if matchesContentRe("     ", 2) { // indented but blank
		t.Error("matchesContentRe blank")
	}

	if consumeIndentUnits("ab", 1) != -1 { // neither tab nor four spaces
		t.Error("consumeIndentUnits insufficient")
	}

	if !hasNonSpace("x") || hasNonSpace("   ") || hasNonSpace("\t") {
		t.Error("hasNonSpace classification")
	}

	if matchesLazyRe("", 2) { // blank never lazily continues
		t.Error("matchesLazyRe blank")
	}
	if !matchesLazyRe("plain", 5) { // indentation>3 clamps max to 3
		t.Error("matchesLazyRe plain")
	}
	if matchesLazyRe("{:.cls}", 2) { // a block IAL is a lazy boundary
		t.Error("matchesLazyRe block IAL")
	}
	if matchesLazyRe("<div>", 2) { // an HTML block start is a lazy boundary
		t.Error("matchesLazyRe html")
	}
	if !matchesLazyRe("     {:.cls}", 2) { // IAL beyond the indent is not a boundary
		t.Error("matchesLazyRe deep IAL")
	}

	if s, ok := stripListIndent("\tx", 4); s != "x" || !ok { // tab expands then strips
		t.Errorf("stripListIndent tab = %q,%v", s, ok)
	}
	if s, ok := stripListIndent("  x", 4); s != "  x" || ok { // under-indented
		t.Errorf("stripListIndent short = %q,%v", s, ok)
	}
}
