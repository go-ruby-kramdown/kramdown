# Vendored kramdown conformance corpus

These `*.text` / `*.html` / `*.options` files are the upstream
[`kramdown`](https://github.com/gettalong/kramdown) gem's own `to_html` test
corpus (`test/testcases/**`), vendored verbatim from **kramdown 2.5.2** so this
pure-Go port can be gated byte-for-byte against the reference implementation's
expected output.

Only the files the HTML oracle reads are vendored (`*.text`, `*.html`,
`*.options`, and the per-directory `options` files); the `man/` converter cases
and the non-HTML output fixtures (`*.latex`, `*.kramdown`, `*.man`,
`*.htmlinput`) are omitted.

kramdown is licensed under the MIT license; its license text is preserved
alongside this corpus in `KRAMDOWN-COPYING`. Copyright (C) Thomas Leitner.
