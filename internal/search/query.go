package search

import (
	"strings"
	"unicode"
)

// fts5Special are single-character tokens that FTS5 interprets as operators
// even when they appear inside a larger token.
var fts5Special = []string{`"`, `(`, `)`, `*`, `^`, `+`, `-`, `~`, `:`}

// fts5KeywordOperators are multi-character FTS5 operators. They only take
// effect as whitespace/non-alphanumeric-delimited tokens (matched
// case-insensitively by FTS5), e.g. "foo AND bar", not inside "android".
var fts5KeywordOperators = map[string]bool{
	"AND":  true,
	"OR":   true,
	"NOT":  true,
	"NEAR": true,
}

func hasFTS5Operators(input string) bool {
	for _, op := range fts5Special {
		if strings.Contains(input, op) {
			return true
		}
	}
	for _, tok := range strings.FieldsFunc(input, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}) {
		if fts5KeywordOperators[strings.ToUpper(tok)] {
			return true
		}
	}
	return false
}

func escapeFTS5Term(term string) string {
	escaped := strings.NewReplacer(
		`"`, `""`,
		`(`, `(`,
		`)`, `)`,
	).Replace(term)
	return `"` + escaped + `"`
}

func buildMatchQuery(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	if hasFTS5Operators(input) {
		return input
	}

	terms := strings.Fields(input)
	if len(terms) == 0 {
		return ""
	}

	// No operators: every term is an FTS5 AND term, prefix-expanded so that
	// plain words also match longer identifiers (e.g. "sqlite" matches
	// "sqlite3_init").
	var parts []string
	for _, t := range terms {
		parts = append(parts, escapeFTS5Term(t)+"*")
	}
	return strings.Join(parts, " ")
}
