// Copyright (c) the go-ruby-kramdown/kramdown authors
//
// SPDX-License-Identifier: BSD-3-Clause

package kramdown

import (
	"strconv"
	"strings"
)

// symPart is one (entity name, code point) component of a typographic symbol; the
// guillemet "_space" variants expand to two components.
type symPart struct {
	name string
	cp   int
}

// symParts maps a typographic-symbol name to its component entities. A plain symbol
// has one component; the guillemet space variants pair the bracket with an adjacent
// non-breaking space, matching kramdown's SmartQuotes/Typographic output.
var symParts = map[string][]symPart{
	"lsquo":       {{"lsquo", 0x2018}},
	"rsquo":       {{"rsquo", 0x2019}},
	"ldquo":       {{"ldquo", 0x201C}},
	"rdquo":       {{"rdquo", 0x201D}},
	"ndash":       {{"ndash", 0x2013}},
	"mdash":       {{"mdash", 0x2014}},
	"hellip":      {{"hellip", 0x2026}},
	"laquo":       {{"laquo", 0x00AB}},
	"raquo":       {{"raquo", 0x00BB}},
	"laquo_space": {{"laquo", 0x00AB}, {"nbsp", 0x00A0}},
	"raquo_space": {{"nbsp", 0x00A0}, {"raquo", 0x00BB}},
}

// entityOut renders one entity according to the entity_output mode: "symbolic" emits
// "&name;", "numeric" emits "&#cp;", and any other value (the default :as_char) emits
// the literal character. It mirrors kramdown's entity_to_str for these symbols, whose
// characters are never in the escaped set so :as_char always yields the character.
func entityOut(name string, cp int, mode string) string {
	switch mode {
	case "symbolic":
		return "&" + name + ";"
	case "numeric":
		return "&#" + strconv.Itoa(cp) + ";"
	default:
		return string(rune(cp))
	}
}

// renderSym renders a typographic-symbol / smart-quote element under the given
// entity_output mode, expanding each component through entityOut.
func renderSym(name, mode string) string {
	var b strings.Builder
	for _, p := range symParts[name] {
		b.WriteString(entityOut(p.name, p.cp, mode))
	}
	return b.String()
}

// typoSym builds an ElTypographicSym element for the named entity, emitted by the
// span parser for a dash / ellipsis / guillemet substitution.
func typoSym(name string) *Element {
	e := newEl(ElTypographicSym)
	e.Value = name
	return e
}

// smartQuote builds an ElSmartQuote element for one of the four quote positions
// (lsquo/rsquo/ldquo/rdquo), whose rendered entity is chosen by the :smart_quotes
// option at conversion time.
func smartQuote(name string) *Element {
	e := newEl(ElSmartQuote)
	e.Value = name
	return e
}
