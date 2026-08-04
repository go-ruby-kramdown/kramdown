// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// gfmOpts returns a DefaultOptions set switched to the GFM input dialect.
func gfmOpts() *Options {
	o := DefaultOptions()
	o.Input = "GFM"
	return &o
}

// TestGFMTaskList asserts byte-exact parity with the reference kramdown gem
// (kramdown 2.5.2 + kramdown-parser-gfm 1.1.0) run as
// Kramdown::Document.new(src, input: 'GFM').to_html, across the task-list feature
// surface: unchecked/checked boxes, the "task-list"/"task-list-item" classes and
// their carry-forward semantics, ordered lists, nesting, loose lists, inline markup
// after the marker, and the non-matching guards.
func TestGFMTaskList(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "basic_mixed",
			src:  "- [ ] a\n- [x] b\n- normal\n",
			want: `<ul class="task-list">
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" />a</li>
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" checked="checked" />b</li>
  <li class="task-list-item">normal</li>
</ul>
`,
		},
		{
			name: "nested",
			src:  "- [ ] top\n  - [x] nested\n",
			want: `<ul class="task-list">
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" />top
    <ul class="task-list">
      <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" checked="checked" />nested</li>
    </ul>
  </li>
</ul>
`,
		},
		{
			name: "ordered",
			src:  "1. [ ] a\n2. [x] b\n",
			want: `<ol class="task-list">
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" />a</li>
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" checked="checked" />b</li>
</ol>
`,
		},
		{
			// A non-task item preceding the first task item stays unclassed, but the
			// list is still a task-list (gem's carry-forward is_tasklist flag).
			name: "normal_before_task",
			src:  "- normal first\n- [ ] task second\n",
			want: `<ul class="task-list">
  <li>normal first</li>
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" />task second</li>
</ul>
`,
		},
		{
			name: "uppercase_x_checked",
			src:  "- [X] upper\n",
			want: `<ul class="task-list">
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" checked="checked" />upper</li>
</ul>
`,
		},
		{
			// No whitespace after the bracket pair: not a task marker.
			name: "no_space_after_bracket",
			src:  "- [ ]x nospace\n",
			want: `<ul>
  <li>[ ]x nospace</li>
</ul>
`,
		},
		{
			name: "markup_after_marker",
			src:  "- [ ] **bold** and _em_\n",
			want: `<ul class="task-list">
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" /><strong>bold</strong> and <em>em</em></li>
</ul>
`,
		},
		{
			name: "extra_spaces",
			src:  "-   [ ]   extra spaces\n",
			want: `<ul class="task-list">
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" />extra spaces</li>
</ul>
`,
		},
		{
			// A loose list keeps the <p> wrapper; the checkbox is injected inside it.
			name: "loose_list",
			src:  "- [ ] a\n\n- [x] b\n",
			want: `<ul class="task-list">
  <li class="task-list-item">
    <p><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" />a</p>
  </li>
  <li class="task-list-item">
    <p><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" checked="checked" />b</p>
  </li>
</ul>
`,
		},
		{
			// Only the nested list is a task-list; the outer item's text is not a marker.
			name: "outer_plain_inner_task",
			src:  "- text\n  - [ ] nested only\n",
			want: `<ul>
  <li>text
    <ul class="task-list">
      <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" />nested only</li>
    </ul>
  </li>
</ul>
`,
		},
		{
			// A bare "[ ]" with no following content is not a task marker.
			name: "bare_bracket",
			src:  "- [ ]\n",
			want: `<ul>
  <li>[ ]</li>
</ul>
`,
		},
		{
			// An item whose first child is not a paragraph (a blockquote here) is skipped
			// by the scan; a later task item still makes the list a task-list.
			name: "first_child_not_paragraph",
			src:  "- > quote first\n- [ ] b\n",
			want: `<ul class="task-list">
  <li>
    <blockquote>
      <p>quote first</p>
    </blockquote>
  </li>
  <li class="task-list-item"><input type="checkbox" class="task-list-item-checkbox" disabled="disabled" />b</li>
</ul>
`,
		},
	}

	opts := gfmOpts()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ToHTML(tc.src, opts)
			if got != tc.want {
				t.Errorf("GFM task-list mismatch for %q\n--- got ---\n%s\n--- want ---\n%s", tc.src, got, tc.want)
			}
		})
	}
}

// TestGFMTaskListDefaultModeUnchanged confirms the task-list transformation is gated
// on the GFM dialect: under the default kramdown parser (empty Input, and the
// explicit "kramdown" input name) the "[ ] "/"[x] " markers render literally and the
// list gains no "task-list" classes — so the default-mode corpus is unaffected.
func TestGFMTaskListDefaultModeUnchanged(t *testing.T) {
	const src = "- [ ] a\n- [x] b\n"
	const want = `<ul>
  <li>[ ] a</li>
  <li>[x] b</li>
</ul>
`
	if got := ToHTML(src, nil); got != want {
		t.Errorf("default-mode (nil opts) mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	o := DefaultOptions()
	o.Input = "kramdown"
	if got := ToHTML(src, &o); got != want {
		t.Errorf("default-mode (Input=kramdown) mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
