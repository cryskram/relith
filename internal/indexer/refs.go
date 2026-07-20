package indexer

import (
	"regexp"
	"strings"
)

var wordRe = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*\(`)

var skipWords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true,
	"catch": true, "case": true, "return": true, "throw": true,
	"yield": true, "await": true, "delete": true,
	"import": true, "export": true, "include": true, "define": true,
	"typedef": true, "sizeof": true, "typeof": true, "instanceof": true,
	"require": true, "assert": true, "raise": true, "print": true,
	"printf": true, "sprintf": true, "fprintf": true,
	"expect": true, "describe": true, "it": true, "test": true,
	"var": true, "let": true, "const": true, "func": true, "fn": true,
	"def": true, "class": true, "struct": true, "enum": true,
	"trait": true, "interface": true, "impl": true, "type": true,
	"package": true, "new": true, "make": true, "append": true,
	"len": true, "cap": true, "copy": true, "close": true,
	"panic": true, "recover": true, "defer": true, "go": true,
	"select": true, "range": true, "map": true, "chan": true,
}

type Ref struct {
	Name    string
	Line    int
	Col     int
	Context string
}

func ExtractReferences(content string) []Ref {
	lines := strings.Split(content, "\n")
	var refs []Ref
	seen := map[string]bool{}

	for i, line := range lines {
		matches := wordRe.FindAllStringSubmatchIndex(line, -1)
		for _, m := range matches {
			if len(m) < 4 {
				continue
			}
			nameStart, nameEnd := m[2], m[3]
			name := line[nameStart:nameEnd]

			lower := strings.ToLower(name)
			if skipWords[lower] {
				continue
			}
			if len(name) < 2 {
				continue
			}

			key := name + ":" + line
			if seen[key] {
				continue
			}
			seen[key] = true

			col := strings.Index(line, name)
			if col < 0 {
				col = nameStart
			}

			context := strings.TrimSpace(line)
			if len(context) > 120 {
				context = context[:117] + "..."
			}

			refs = append(refs, Ref{
				Name:    name,
				Line:    i,
				Col:     col,
				Context: context,
			})
		}
	}

	return refs
}
