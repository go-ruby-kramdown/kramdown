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
