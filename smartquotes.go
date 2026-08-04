// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

// This file ports Kramdown::Parser::Kramdown#parse_smart_quotes (the RubyPants-derived
// SQ_RULES). Curly-quote direction is decided during span parsing — not in a post-parse
// pass — because the choice depends on the raw source context of each straight quote:
// the immediately preceding character (a backslash-escaped bracket, an opening paren, a
// closing word) and the characters that follow. Emitting the decision here, over the raw
// text, is what lets this port match the gem byte-for-byte on quote direction.

// sqItem is one ordered emission from the smart-quote parser: a typographic-symbol name
// (sym) or, for the rare doubled-quote-at-dispatch case, a literal text fragment (text).
type sqItem struct {
	sym  string
	text string
}

// isRubySpace reports whether b is in Ruby's \s class ([ \t\r\n\f\v]).
func isRubySpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\f', '\v':
		return true
	}
	return false
}

// isDigitByte reports whether b is an ASCII digit (Ruby \d).
func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }

// isSQPunct reports whether b is in kramdown's SQ_PUNCT class, the punctuation a quote
// may "close" against when it directly precedes non-word content.
func isSQPunct(b byte) bool {
	switch b {
	case '!', '"', '#', '$', '%', '\'', '(', ')', '*', '+', ',', '-', '.', '/',
		':', ';', '<', '=', '>', '?', '@', '[', '\\', ']', '^', '_', '`', '{',
		'|', '}', '~':
		return true
	}
	return false
}

// isSQClose reports whether b is in kramdown's SQ_CLOSE class ([^ \\\t\r\n\[{(-]): the
// characters after which a straight quote is treated as a closing quote.
func isSQClose(b byte) bool {
	switch b {
	case ' ', '\\', '\t', '\r', '\n', '[', '{', '(', '-':
		return false
	}
	return true
}

// resolveQuote maps an opening/closing decision plus the straight-quote character to the
// corresponding typographic-symbol name.
func resolveQuote(closing bool, quote byte) string {
	if closing {
		if quote == '"' {
			return "rdquo"
		}
		return "rsquo"
	}
	if quote == '"' {
		return "ldquo"
	}
	return "lsquo"
}

// smartQuoteAt ports parse_smart_quotes for the straight quote at src[q]. prevAvailable
// reports whether the immediately preceding source byte is buffered plain text (kramdown
// dispatches the parser one character earlier in that case, capturing it as the rules'
// leading group); prev is that byte. It returns the ordered emissions and the number of
// forward source bytes consumed (one quote, or two for the doubled-quote rules). A
// straight quote always matches one of the final catch-all rules, so it never returns nil.
func smartQuoteAt(src string, q int, prevAvailable bool, prev byte) ([]sqItem, int) {
	qc := src[q]
	at := func(i int) byte {
		if i < len(src) {
			return src[i]
		}
		return 0 // past end: not a word/space/digit character
	}
	n1, n2, n3 := at(q+1), at(q+2), at(q+3)
	preSpaceOrNone := !prevAvailable || isRubySpace(prev)

	// Rules 1, 2 and 8 anchor on the quote itself, so they only apply when the parser is
	// dispatched at the quote (no preceding plain-text character to capture).
	if !prevAvailable {
		// R1: /("|')(?=[_*]{1,2}\S)/ -> opening quote before emphasis markers.
		if n1 == '_' || n1 == '*' {
			twoThenNonSpace := (n2 == '_' || n2 == '*') && n3 != 0 && !isRubySpace(n3)
			oneThenNonSpace := n2 != 0 && !isRubySpace(n2)
			if twoThenNonSpace || oneThenNonSpace {
				return []sqItem{{sym: resolveQuote(false, qc)}}, 1
			}
		}
		// R2: /("|')(?=SQ_PUNCT(?!\.\.)\B)/ -> closing quote before punctuation.
		if isSQPunct(n1) && !(n2 == '.' && n3 == '.') && (isWordByte(n1) == isWordByte(n2)) {
			return []sqItem{{sym: resolveQuote(true, qc)}}, 1
		}
	}

	// R3: /(\s?)"'(?=\w)/ -> an opening double then single quote.
	if qc == '"' && n1 == '\'' && isWordByte(n2) && preSpaceOrNone {
		return []sqItem{{sym: "ldquo"}, {sym: "lsquo"}}, 2
	}
	// R4: /(\s?)'"(?=\w)/ -> an opening single then double quote.
	if qc == '\'' && n1 == '"' && isWordByte(n2) && preSpaceOrNone {
		return []sqItem{{sym: "lsquo"}, {sym: "ldquo"}}, 2
	}
	// R5: /(\s?)'(?=\d\ds)/ -> a decade abbreviation apostrophe (the '80s).
	if qc == '\'' && isDigitByte(n1) && isDigitByte(n2) && n3 == 's' && preSpaceOrNone {
		return []sqItem{{sym: "rsquo"}}, 1
	}
	// R6: /(\s)('|")(?=\w)/ -> opening quote after whitespace, before a word.
	if prevAvailable && isRubySpace(prev) && isWordByte(n1) {
		return []sqItem{{sym: resolveQuote(false, qc)}}, 1
	}
	// R7: /(SQ_CLOSE)('|")/ -> closing quote after a "close" character.
	if prevAvailable {
		if isSQClose(prev) {
			return []sqItem{{sym: resolveQuote(true, qc)}}, 1
		}
	} else if n1 == '\'' || n1 == '"' {
		// Dispatched at the quote with a second quote following: the first is literal text,
		// the second closes (kramdown's group1 is the first quote char here).
		return []sqItem{{text: string(qc)}, {sym: resolveQuote(true, n1)}}, 2
	}
	// R8: /("|')(?=\s|s\b|$)/ -> closing quote before space, a bare "s", or end.
	if !prevAvailable {
		if isRubySpace(n1) || n1 == 0 || (n1 == 's' && !isWordByte(n2)) {
			return []sqItem{{sym: resolveQuote(true, qc)}}, 1
		}
	}
	// R9 / R10: any remaining quote is an opening one.
	return []sqItem{{sym: resolveQuote(false, qc)}}, 1
}
