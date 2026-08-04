// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"bufio"
	"embed"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// corpusFS embeds the upstream kramdown gem's own to_html test corpus
// (test/testcases/**, vendored verbatim from kramdown 2.5.2 — see
// testdata/testcases/README.md). Each *.text is a source document and the
// sibling *.html is the exact output the reference implementation produces;
// options come from a sibling <base>.options file, else a per-directory
// `options` file, else kramdown's file-runner default of {auto_ids: false,
// footnote_nr: 1}.
//
//go:embed testdata/testcases
var corpusFS embed.FS

const corpusRoot = "testdata/testcases"

// applyCorpusOptions folds the recognised keys of a kramdown YAML options file
// into o. It parses the flat, top-level scalar keys this port honours plus the one
// nested block it models — :syntax_highlighter_opts, whose default_lang/guess_lang
// scalars and block:/span: {disable} sub-maps drive the Rouge wiring. Keys and
// nested blocks it does not model are ignored, which is why cases relying on them
// stay in the ratchet's knownFailing ledger.
func applyCorpusOptions(o Options, text string) Options {
	sc := bufio.NewScanner(strings.NewReader(text))
	inSHOpts := false   // inside the :syntax_highlighter_opts subtree
	inTypoSyms := false // inside the :typographic_symbols subtree
	inLinkDefs := false // inside the :link_defs subtree
	subCtx := ""        // "block"/"span" nested map, or "" at the opts' top level
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			switch {
			case inSHOpts:
				applyCorpusSHOptsLine(&o.SyntaxHighlighterOpts, line, &subCtx)
			case inTypoSyms:
				applyCorpusTypoSymLine(&o.TypographicSymbols, line)
			case inLinkDefs:
				applyCorpusLinkDefLine(o.LinkDefs, line)
			}
			continue // nested value of an unmodelled option
		}
		inSHOpts, inTypoSyms, inLinkDefs, subCtx = false, false, false, ""
		line = strings.TrimPrefix(line, ":") // ruby symbol key marker
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		switch key {
		case "auto_ids":
			o.AutoIds = val == "true"
		case "auto_id_prefix":
			o.AutoIdPrefix = val
		case "auto_id_stripping":
			o.AutoIdStripping = val == "true"
		case "header_offset":
			if n, err := strconv.Atoi(val); err == nil {
				o.HeaderOffset = n
			}
		case "header_links":
			o.HeaderLinks = val == "true"
		case "transliterated_header_ids":
			o.TransliteratedHeaderIds = val == "true"
		case "footnote_nr":
			if n, err := strconv.Atoi(val); err == nil {
				o.FootnoteNr = n
			}
		case "hard_wrap":
			o.HardWrap = val == "true"
		case "parse_block_html":
			o.ParseBlockHTML = val == "true"
		case "html_to_native":
			o.HtmlToNative = val == "true"
		case "smart_quotes":
			if val == "false" {
				o.SmartQuotes = false
			} else {
				o.SmartQuotes = true
				for i, name := range strings.Split(val, ",") {
					if i < len(o.SmartQuotesSubst) {
						o.SmartQuotesSubst[i] = strings.TrimSpace(name)
					}
				}
			}
		case "remove_line_breaks_for_cjk":
			o.RemoveLineBreaksForCJK = val == "true"
		case "entity_output":
			o.EntityOutput = strings.TrimPrefix(val, ":")
		case "toc_levels":
			o.TocLevels = parseTocLevels(val)
		case "footnote_prefix":
			o.FootnotePrefix = val
		case "footnote_backlink":
			o.FootnoteBacklink = val
		case "footnote_backlink_inline":
			o.FootnoteBacklinkInline = val == "true"
		case "footnote_link_text":
			o.FootnoteLinkText = val
		case "syntax_highlighter":
			o.SyntaxHighlighter = normalizeHighlighter(val)
		case "syntax_highlighter_opts":
			inSHOpts = true
		case "typographic_symbols":
			inTypoSyms = true
			o.TypographicSymbols = map[string]string{}
		case "link_defs":
			inLinkDefs = true
			o.LinkDefs = map[string]LinkDef{}
		}
	}
	return o
}

// parseTocLevels mirrors kramdown's :toc_levels normalisation for the two string
// forms the corpus uses: a "a..b" range or a comma-separated list of levels.
func parseTocLevels(val string) []int {
	var levels []int
	if lo, hi, ok := strings.Cut(val, ".."); ok {
		l, _ := strconv.Atoi(strings.TrimSpace(lo))
		h, _ := strconv.Atoi(strings.TrimSpace(hi))
		for i := l; i <= h; i++ {
			levels = append(levels, i)
		}
		return levels
	}
	for _, p := range strings.Split(val, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			levels = append(levels, n)
		}
	}
	return levels
}

// applyCorpusLinkDefLine folds one indented "id: [url, title]" line of a
// :link_defs block into m, mirroring kramdown's option: the value is a YAML array
// whose first element is the URL and optional second element the title.
func applyCorpusLinkDefLine(m map[string]LinkDef, line string) {
	key, val, ok := strings.Cut(strings.TrimSpace(line), ":")
	if !ok {
		return
	}
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)
	val = strings.TrimPrefix(val, "[")
	val = strings.TrimSuffix(val, "]")
	parts := strings.SplitN(val, ",", 2)
	def := LinkDef{URL: strings.TrimSpace(parts[0])}
	if len(parts) > 1 {
		def.Title = strings.TrimSpace(parts[1])
	}
	m[key] = def
}

// applyCorpusTypoSymLine folds one indented "name: value" line of a
// :typographic_symbols block into the override map, mirroring kramdown's option
// (the symbol name keys hellip/mdash/…, the quoted scalar is the replacement).
func applyCorpusTypoSymLine(m *map[string]string, line string) {
	key, val, ok := strings.Cut(strings.TrimSpace(line), ":")
	if !ok {
		return
	}
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)
	val = strings.Trim(val, `"'`)
	if *m == nil {
		*m = map[string]string{}
	}
	(*m)[key] = val
}

// applyCorpusSHOptsLine folds one indented line of a :syntax_highlighter_opts
// block into o. A key with an empty value (block:/span:) opens a nested map tracked
// through subCtx; default_lang/guess_lang set the scalar fields; a disable: true
// under block:/span: flips the corresponding Disable flag. Unmodelled keys (wrap,
// css, line_numbers, …) are ignored.
func applyCorpusSHOptsLine(o *SyntaxHighlighterOpts, line string, subCtx *string) {
	trimmed := strings.TrimSpace(line)
	// Two-space indent is the opts' top level; deeper indent is inside block:/span:.
	topLevel := len(line)-len(strings.TrimLeft(line, " ")) <= 2
	key, val, ok := strings.Cut(trimmed, ":")
	if !ok {
		return
	}
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)
	if topLevel {
		*subCtx = ""
		switch key {
		case "default_lang":
			o.DefaultLang = val
		case "guess_lang":
			o.GuessLang = val == "true"
		case "block", "span":
			if val == "" {
				*subCtx = key
			}
		}
		return
	}
	if key == "disable" && val == "true" {
		switch *subCtx {
		case "block":
			o.BlockDisable = true
		case "span":
			o.SpanDisable = true
		}
	}
}

// resolveCorpusOptions builds the Options for a corpus case, mirroring the gem's
// file runner: when an options file (a sibling <case>.options or a per-dir
// `options`) exists, its YAML *replaces* the runner defaults, so unspecified keys
// fall back to kramdown's own defaults (e.g. auto_ids on). Only when no options
// file is present does the runner baseline {auto_ids: false, footnote_nr: 1} apply.
func resolveCorpusOptions(textPath string) Options {
	base := strings.TrimSuffix(textPath, ".text")
	if b, err := corpusFS.ReadFile(base + ".options"); err == nil {
		return applyCorpusOptions(DefaultOptions(), string(b))
	}
	if b, err := corpusFS.ReadFile(path.Join(path.Dir(textPath), "options")); err == nil {
		return applyCorpusOptions(DefaultOptions(), string(b))
	}
	o := DefaultOptions()
	o.AutoIds = false
	return o
}

// TestCorpusRatchet renders every *.text/​*.html pair of the vendored kramdown
// corpus and compares byte-for-byte against the gem's expected output. It is a
// shrink-only ratchet: every case outside knownFailing MUST match, and any case
// listed in knownFailing that starts matching is flagged so the ledger can only
// shrink (preventing a fix here / break there swap at equal count).
func TestCorpusRatchet(t *testing.T) {
	var pass, fail int
	var unexpectedFail, nowPassing []string
	err := fs.WalkDir(corpusFS, corpusRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".text") {
			return nil
		}
		want, err := corpusFS.ReadFile(strings.TrimSuffix(p, ".text") + ".html")
		if err != nil {
			return nil // no .html sibling: not a to_html case
		}
		src, err := corpusFS.ReadFile(p)
		if err != nil {
			return err
		}
		opts := resolveCorpusOptions(p)
		got := ToHTML(string(src), &opts)
		rel := strings.TrimPrefix(p, corpusRoot+"/")
		allowedFail := knownFailing[rel] || corpusExceptions[rel]
		if got == string(want) {
			pass++
			if allowedFail {
				nowPassing = append(nowPassing, rel)
			}
		} else {
			fail++
			if !allowedFail {
				unexpectedFail = append(unexpectedFail, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	total := pass + fail
	t.Logf("corpus conformance: %d/%d byte-exact (%.2f%%); knownFailing=%d, exceptions=%d",
		pass, total, 100*float64(pass)/float64(total), len(knownFailing), len(corpusExceptions))

	sort.Strings(unexpectedFail)
	for _, f := range unexpectedFail {
		t.Errorf("REGRESSION: %s is not in knownFailing but does not match the gem's output", f)
	}
	sort.Strings(nowPassing)
	for _, f := range nowPassing {
		t.Errorf("RATCHET: %s now matches the gem — remove it from knownFailing (the set only shrinks)", f)
	}
}
