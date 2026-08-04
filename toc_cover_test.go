// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import "testing"

// TestTOCCustomAttributes covers a {:toc} list that carries its own class and id: the
// generated list keeps those attributes (class then id, in IAL order) and the entry
// ids are prefixed with the custom id instead of the "markdown-toc" default. Verified
// byte-for-byte against kramdown 2.5.2.
func TestTOCCustomAttributes(t *testing.T) {
	o := DefaultOptions()
	o.AutoIds = true
	src := "* toc\n{:toc .foo #bar}\n\n# Head One\n\n## Head Two\n"
	want := `<ul class="foo" id="bar">
  <li><a href="#head-one" id="bar-head-one">Head One</a>    <ul>
      <li><a href="#head-two" id="bar-head-two">Head Two</a></li>
    </ul>
  </li>
</ul>

<h1 id="head-one">Head One</h1>

<h2 id="head-two">Head Two</h2>
`
	if got := ToHTML(src, &o); got != want {
		t.Errorf("toc with custom attrs:\n got %q\nwant %q", got, want)
	}
}

// TestTOCLevelsAndSecondList covers the TocLevels option restricting which headers
// appear, and a second {:toc} list (only the first is a placement site — the rest
// render as ordinary lists).
func TestTOCLevelsAndSecondList(t *testing.T) {
	o := DefaultOptions()
	o.AutoIds = true
	o.TocLevels = []int{2}
	// Only the h2 is in scope; the h1 is excluded. A second {:toc} list stays literal.
	src := "* toc\n{:toc}\n\n# One\n\n## Two\n\n* again\n{:toc}\n"
	got := ToHTML(src, &o)
	want := `<ul id="markdown-toc">
  <li><a href="#two" id="markdown-toc-two">Two</a></li>
</ul>

<h1 id="one">One</h1>

<h2 id="two">Two</h2>

<ul>
  <li>again</li>
</ul>
`
	if got != want {
		t.Errorf("toc_levels + second list:\n got %q\nwant %q", got, want)
	}
}
