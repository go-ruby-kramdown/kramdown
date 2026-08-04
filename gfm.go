// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "regexp"

// This file ports the task-list handling of kramdown-parser-gfm's GFM parser
// (Kramdown::Parser::GFM#parse_list). Under the :input => "GFM" dialect a list
// item whose first paragraph begins with "[ ] " or "[x] " is turned into a
// task-list checkbox: the marker is replaced, in the item's raw text, by a raw
// <input> element (which the span parser then passes through verbatim), and the
// enclosing <ul>/<ol> and the affected <li> elements gain "task-list" /
// "task-list-item" classes.

// gfmBoxUnchecked / gfmBoxChecked are the exact <input> strings kramdown-parser-gfm
// substitutes for an unchecked ("[ ] ") / checked ("[x] ") task marker.
const (
	gfmBoxUnchecked = `<input type="checkbox" class="task-list-item-checkbox" disabled="disabled" />`
	gfmBoxChecked   = `<input type="checkbox" class="task-list-item-checkbox" disabled="disabled" checked="checked" />`
)

// reGFMTaskUnchecked / reGFMTaskChecked mirror the GFM parser's anchored markers:
//
//	/\A\s*\[ \]\s+/   (unchecked)
//	/\A\s*\[x\]\s+/i  (checked, case-insensitive)
//
// Both require at least one whitespace character after the bracket pair, so a bare
// "[ ]"/"[x]" with no following content is left untouched, and both are anchored to
// the start of the item's text. Go's default (non-multiline) `^` matches only the
// start of the string, matching Ruby's `\A`.
var (
	reGFMTaskUnchecked = regexp.MustCompile(`^\s*\[ \]\s+`)
	reGFMTaskChecked   = regexp.MustCompile(`(?i)^\s*\[x\]\s+`)
)

// applyGFMTaskList applies the GFM task-list transformation to a freshly parsed
// list, following Kramdown::Parser::GFM#parse_list. It scans each item's first
// paragraph: when that paragraph's raw text opens with a task marker, the marker is
// rewritten to the corresponding <input> element and the item is flagged. Once any
// item in the list is a task item, that item and every subsequent item receive the
// "task-list-item" class and the list itself receives the "task-list" class —
// exactly reproducing the gem's carry-forward `is_tasklist` flag (items preceding
// the first task item stay unclassed).
func applyGFMTaskList(list *Element) {
	isTasklist := false
	for _, li := range list.Children {
		if len(li.Children) == 0 || li.Children[0].Type != ElP {
			continue
		}
		p := li.Children[0]
		raw, _ := p.Options["raw"].(string)
		matched := false
		switch {
		case reGFMTaskUnchecked.MatchString(raw):
			raw = reGFMTaskUnchecked.ReplaceAllLiteralString(raw, gfmBoxUnchecked)
			matched = true
		case reGFMTaskChecked.MatchString(raw):
			raw = reGFMTaskChecked.ReplaceAllLiteralString(raw, gfmBoxChecked)
			matched = true
		}
		if matched {
			p.Options["raw"] = raw
			isTasklist = true
		}
		if isTasklist {
			li.setAttr("class", "task-list-item")
		}
	}
	if isTasklist {
		list.setAttr("class", "task-list")
	}
}
