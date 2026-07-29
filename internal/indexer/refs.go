package indexer

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

func isIdentByte(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

func ExtractReferences(content string) []Ref {
	var refs []Ref
	seen := make(map[string]bool)
	line := 0

	for i := 0; i < len(content); i++ {
		ch := content[i]
		if ch == '\n' {
			line++
			continue
		}
		if ch != '(' {
			continue
		}

		start := i - 1
		for start >= 0 && (content[start] == ' ' || content[start] == '\t') {
			start--
		}
		if start < 0 || !isIdentByte(content[start]) {
			continue
		}
		nameEnd := start + 1
		for start >= 0 && isIdentByte(content[start]) {
			start--
		}
		nameStart := start + 1
		if nameStart >= nameEnd {
			continue
		}
		if content[nameStart] >= '0' && content[nameStart] <= '9' {
			continue
		}

		name := content[nameStart:nameEnd]
		if len(name) < 2 {
			continue
		}

		var lowerBuf [64]byte
		nameLen := len(name)
		lower := lowerBuf[:]
		if nameLen > 64 {
			lower = make([]byte, nameLen)
		} else {
			lower = lowerBuf[:nameLen]
		}
		for j := 0; j < nameLen; j++ {
			b := name[j]
			if b >= 'A' && b <= 'Z' {
				lower[j] = b - 'A' + 'a'
			} else {
				lower[j] = b
			}
		}
		if skipWords[string(lower)] {
			continue
		}

		linStart := i
		for linStart > 0 && content[linStart-1] != '\n' {
			linStart--
		}
		linEnd := i
		for linEnd < len(content) && content[linEnd] != '\n' {
			linEnd++
		}
		lineStr := content[linStart:linEnd]
		trimmed := stringsTrimSpace(lineStr)
		if len(trimmed) > 120 {
			trimmed = trimmed[:117] + "..."
		}

		key := name + ":" + trimmed
		if seen[key] {
			continue
		}
		seen[key] = true

		col := nameStart - linStart
		if col < 0 {
			col = 0
		}

		refs = append(refs, Ref{
			Name:    name,
			Line:    line,
			Col:     col,
			Context: trimmed,
		})
	}

	return refs
}

func stringsTrimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	if start >= end {
		return ""
	}
	return s[start:end]
}
