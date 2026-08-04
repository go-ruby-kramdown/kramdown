// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	_ "embed"
	"strings"
	"sync"
)

// translitData is the vendored Stringex unidecoder table kramdown's
// Utils::Unidecoder uses for :transliterated_header_ids. Each line encodes one
// 256-entry Unicode block: a two-hex-digit high byte, then 256 US-separated
// (\x1f) transliteration strings for the low bytes 0x00..0xff. Control bytes in an
// entry are escaped as "\xHH" and a literal backslash as "\\". A block that is
// absent from the file (or a codepoint above U+FFFF, whose high byte exceeds 0xff)
// has no transliteration and decodes to "?", exactly as the gem's missing-file
// rescue does; a present block whose entry is empty contributes the empty string.
//
//go:embed translit_data.txt
var translitData string

// translitTable maps a Unicode high byte (codepoint >> 8, 0x00..0xff) to its 256
// low-byte transliterations. It is built once from translitData on first use.
var (
	translitTable map[int][]string
	translitOnce  sync.Once
)

// loadTranslitTable parses the embedded Stringex data into translitTable.
func loadTranslitTable() {
	translitTable = make(map[int][]string, 256)
	for _, line := range strings.Split(strings.TrimRight(translitData, "\n"), "\n") {
		fields := strings.Split(line, "\x1f")
		hi := hexByte(fields[0])
		entries := make([]string, len(fields)-1)
		for i, f := range fields[1:] {
			entries[i] = unescapeTranslit(f)
		}
		translitTable[hi] = entries
	}
}

// hexByte parses a two-digit lowercase hex string into an int.
func hexByte(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*16 + hexDigit(s[i])
	}
	return n
}

// hexDigit maps a lowercase hex digit byte to its value.
func hexDigit(b byte) int {
	if b >= '0' && b <= '9' {
		return int(b - '0')
	}
	return int(b-'a') + 10
}

// unescapeTranslit reverses the "\\"/"\xHH" escaping applied when the table was
// generated, reconstructing an entry's literal bytes.
func unescapeTranslit(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		// A generated escape is always "\\" or "\xHH", so the following byte selects
		// which and (for \x) two hex digits follow.
		if s[i+1] == '\\' {
			b.WriteByte('\\')
			i++
			continue
		}
		// s[i+1] == 'x'
		b.WriteByte(byte(hexByte(s[i+2 : i+4])))
		i += 3
	}
	return b.String()
}

// transliterate ports Utils::Unidecoder.decode: every non-ASCII rune is replaced by
// its Stringex transliteration (an absent block yields "?"), while ASCII runes pass
// through unchanged.
func transliterate(s string) string {
	translitOnce.Do(loadTranslitTable)
	if s == "" {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		if entries, ok := translitTable[int(r)>>8]; ok {
			b.WriteString(entries[int(r)&0xff])
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}
