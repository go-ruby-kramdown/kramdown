// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"regexp"
	"testing"
)

// reFirstID extracts the value of the first ` id="…"` attribute in an HTML string,
// or "" (with ok=false) when none is present.
var reFirstID = regexp.MustCompile(` id="([^"]*)"`)

func firstID(html string) (string, bool) {
	m := reFirstID.FindStringSubmatch(html)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// TestHeaderAutoIDGemParity locks the header auto-id slug (kramdown's
// basic_generate_id / generate_id) to byte-exact parity with the reference gem
// (kramdown 2.5.2, Kramdown::Document.new(src).to_html). The battery covers the
// tricky cases: the double hyphen produced when a stripped punctuation run sits
// between two spaces ("Blockquote & code" -> "blockquote--code"), general
// punctuation, non-ASCII stripping, leading-digit trimming, the "section" fallback
// for an id that reduces to empty, and markup inside the header (whose delimiter
// characters are stripped from the raw source before slugging).
func TestHeaderAutoIDGemParity(t *testing.T) {
	cases := []struct {
		src string
		id  string
	}{
		// The gap example: "& " leaves two spaces -> a double hyphen.
		{"### Blockquote & code", "blockquote--code"},
		{"# Hello, World!", "hello-world"},
		{"## foo: bar", "foo-bar"},
		{"### 123 leading digits", "leading-digits"},
		{"# café résumé", "caf-rsum"},
		{"## a  b   c", "a--b---c"},
		{"# Foo & Bar & Baz", "foo--bar--baz"},
		{"# --- x ---", "x----"},
		{"## C++ & C#", "c--c"},
		// Ids that reduce to empty fall back to "section".
		{"# 42", "section"},
		{"# ...", "section"},
		{"# Über Café", "ber-caf"},
		// Markup delimiter characters are stripped from the raw source.
		{"# **bold** heading", "bold-heading"},
		{"# `code` in head", "code-in-head"},
		{"# [link](/x) here", "linkx-here"},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			got, ok := firstID(ToHTML(tc.src, nil))
			if !ok {
				t.Fatalf("no id emitted for %q", tc.src)
			}
			if got != tc.id {
				t.Errorf("auto-id for %q = %q, gem oracle = %q", tc.src, got, tc.id)
			}
		})
	}
}

// TestHeaderAutoIDGemParityDedup locks the de-duplication suffix (generate_id's
// "-N" counter) to the gem: repeated identical headers get -1, -2, … in order.
func TestHeaderAutoIDGemParityDedup(t *testing.T) {
	html := ToHTML("# Dup\n\n# Dup\n\n# Dup\n", nil)
	ids := reFirstID.FindAllStringSubmatch(html, -1)
	want := []string{"dup", "dup-1", "dup-2"}
	if len(ids) != len(want) {
		t.Fatalf("expected %d ids, got %d (%q)", len(want), len(ids), html)
	}
	for i, w := range want {
		if ids[i][1] != w {
			t.Errorf("dup id #%d = %q, gem oracle = %q", i, ids[i][1], w)
		}
	}
}

// TestHeaderAutoIDGemParityOptions locks the option-driven id variants to the gem:
// the auto_id_prefix prepend, transliterated_header_ids (ASCII-folding before the
// slug), and an explicit {#id} that overrides auto generation entirely.
func TestHeaderAutoIDGemParityOptions(t *testing.T) {
	prefix := DefaultOptions()
	prefix.AutoIdPrefix = "pre-"
	if got, _ := firstID(ToHTML("# Hello There", &prefix)); got != "pre-hello-there" {
		t.Errorf("auto_id_prefix id = %q, gem oracle = %q", got, "pre-hello-there")
	}

	translit := DefaultOptions()
	translit.TransliteratedHeaderIds = true
	if got, _ := firstID(ToHTML("# Über Café", &translit)); got != "uber-cafe" {
		t.Errorf("transliterated id = %q, gem oracle = %q", got, "uber-cafe")
	}

	if got, _ := firstID(ToHTML("# Heading {#custom}", nil)); got != "custom" {
		t.Errorf("explicit id = %q, gem oracle = %q", got, "custom")
	}
}
