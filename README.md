<div align="center">

<img src="https://raw.githubusercontent.com/go-ruby-kramdown/brand/main/social/go-ruby-kramdown-kramdown.png" alt="go-ruby-kramdown/kramdown" width="640">

# go-ruby-kramdown/kramdown

Pure-Go (CGO=0), MRI-faithful reimplementation of the Ruby
[`kramdown`](https://kramdown.gettalong.org/) Markdown-to-HTML converter.

[![CI](https://github.com/go-ruby-kramdown/kramdown/actions/workflows/ci.yml/badge.svg)](https://github.com/go-ruby-kramdown/kramdown/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25-brightgreen)](https://github.com/go-ruby-kramdown/kramdown/actions/workflows/ci.yml)
[![Conformance](https://img.shields.io/badge/conformance-196%2F198%20(98.99%25)-1a7f37)](#conformance--testing)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-ruby-kramdown/kramdown.svg)](https://pkg.go.dev/github.com/go-ruby-kramdown/kramdown)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)

</div>

## About

`go-ruby-kramdown` renders the [kramdown](https://kramdown.gettalong.org/syntax.html)
dialect of Markdown to HTML. Across the shared kramdown test corpus, **196 of 198
cases (98.99%) render byte-exact against the Ruby `kramdown` 2.5.2 gem** — see
[Conformance & testing](#conformance--testing) for the measured number and the
short list of what is not yet supported. It is a member of the [go-ruby-*](https://github.com/go-embedded-ruby)
family of pure-Go Ruby modules that [go-embedded-ruby](https://github.com/go-embedded-ruby)
(`rbgo`) binds as native modules — there is **no cgo and no external process**: the
converter is a self-contained Go package that cross-compiles to every Go target.

## Install

```sh
go get github.com/go-ruby-kramdown/kramdown
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-kramdown/kramdown"
)

func main() {
	html := kramdown.ToHTML("# Hello *kramdown*\n\nA paragraph with a footnote.[^1]\n\n[^1]: the note.\n", nil)
	fmt.Print(html)
}
```

For finer control, parse into a `*Document` and inspect warnings:

```go
doc := kramdown.New(src, &kramdown.Options{AutoIds: true, SmartQuotes: true})
html := doc.ToHTML()
for _, w := range doc.Warnings {
	// e.g. an undefined footnote reference
	fmt.Println("warning:", w)
}
```

`ToHTML(src, nil)` uses `DefaultOptions()`, which mirrors kramdown's own defaults
(`AutoIds`, `SmartQuotes`, `Typographic`, `HardWrap` all on; footnotes numbered
from 1).

## Options

| Field          | Default | Meaning                                                              |
|----------------|---------|----------------------------------------------------------------------|
| `AutoIds`      | `true`  | Assign a generated `id=""` to headers lacking an explicit `{#id}`.   |
| `AutoIdPrefix` | `""`    | Prefix prepended to every auto-generated header id.                  |
| `SmartQuotes`  | `true`  | Curly quotes / apostrophes via the SmartQuotes substitution.         |
| `Typographic`  | `true`  | `--`→en-dash, `---`→em-dash, `...`→ellipsis, `<< >>`→guillemets.      |
| `HardWrap`     | `true`  | Trailing two-spaces → `<br />`; when off, only `\\` forces a break.   |
| `FootnoteNr`   | `1`     | Starting number for footnotes.                                       |

## Supported syntax

| Area        | Coverage                                                                         |
|-------------|----------------------------------------------------------------------------------|
| Headers     | ATX (`#`) + Setext (`===`/`---`), explicit `{#id}` and auto-ids                   |
| Blocks      | Paragraphs, blockquotes, horizontal rules, indented + fenced code (language class)|
| Lists       | Unordered, ordered, definition lists; lazy and nested                            |
| Tables      | Pipe tables with per-column alignment                                            |
| Inline      | `*em*`/`**strong**`, `` `code` ``, links & images (inline / reference / with attrs), autolinks |
| Footnotes   | `[^id]` references + definitions, with back-links and ordering                   |
| Attributes  | Inline Attribute Lists `{:.class #id key="v"}`, ALDs, `{::comment}` / span IALs   |
| Abbreviations | `*[HTML]: ...` definitions applied to matching text                            |
| Typography  | Smart quotes, `--`/`---`/`...`/`<< >>` substitutions                              |
| HTML        | Raw inline + block HTML passthrough; entity and backslash escapes                |

Edge cases deliberately outside the common feature set are documented in the tests.

## Conformance & testing

A differential oracle compares output to the real `kramdown` gem where it is
installed. Against the shared kramdown test corpus, **196 of 198 cases (98.99%)
render byte-for-byte identically to the Ruby `kramdown` 2.5.2 gem**. A shrink-only
ratchet locks this in: a case outside the known-failing ledger that stops matching
fails CI, and a ledgered case that starts matching must be graduated out. The
deterministic, ruby-free tests alone hold **100% statement coverage**, so the
no-ruby, Windows, and qemu CI lanes stay green. The package is verified on
Linux/macOS/Windows and cross-tested on `amd64`, `arm64`, `riscv64`, `loong64`,
`ppc64le`, and `s390x`.

### Not yet supported (the remaining 2 corpus cases)

`go-ruby-kramdown` is **not** spec-complete. The two cases that do not yet match
the gem both depend on a Ruby-runtime library the pure-Go port cannot supply:

- **Rouge syntax-highlighted code blocks** (2 cases) — the common Rouge paths are
  wired through the pure-Go highlighter, but `rouge/simple` needs a PHP lexer (not
  yet shipped) and `rouge/multiple` selects a bespoke formatter defined inside
  kramdown's own Ruby test harness, which no pure-Go wiring can supply.

### Already done

The port covers a large body of kramdown behaviour, including a full port of
kramdown's `Parser::Html` raw-HTML front-end and `html_to_native` conversion,
table-of-contents generation, the HTML named-entity table, footnotes with
back-links and ordering, smart quotes and typographic substitutions, header
options with auto-ids and transliterated IDs, and CJK line-break handling.

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright (c) the go-ruby-kramdown/kramdown authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
