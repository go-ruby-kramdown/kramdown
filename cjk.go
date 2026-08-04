// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "regexp"

// reRedundantCJKLineBreak mirrors kramdown's REDUNDANT_LINE_BREAK_REGEX: a soft
// newline sandwiched between runs of Han/Hiragana/Katakana characters. Such a break
// is a source-formatting artefact (CJK text does not use spaces or line breaks
// between characters) and is elided under the :remove_line_breaks_for_cjk option.
var reRedundantCJKLineBreak = regexp.MustCompile(`([\p{Han}\p{Hiragana}\p{Katakana}]+)\n([\p{Han}\p{Hiragana}\p{Katakana}]+)`)

// fixCJKLineBreak ports Utils::Html#fix_cjk_line_break: it repeatedly removes a
// newline between two CJK runs until none remain. The loop is required because a
// single pass only joins non-overlapping pairs, so a chain of CJK lines collapses one
// gap per iteration (e.g. "一\n二\n三" -> "一二\n三" -> "一二三").
func fixCJKLineBreak(s string) string {
	for {
		next := reRedundantCJKLineBreak.ReplaceAllString(s, "$1$2")
		if next == s {
			return s
		}
		s = next
	}
}
