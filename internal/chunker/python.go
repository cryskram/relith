package chunker

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	pyFuncPat  = regexp.MustCompile(`^(async\s+)?def\s+([A-Za-z_]\w*)`)
	pyClassPat = regexp.MustCompile(`^class\s+([A-Za-z_]\w*)`)
	pyLambdaRe = regexp.MustCompile(`lambda\s+[\w,\s]*:`)
)

func PythonChunker(content string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	var decls []declInfo
	inDecl := -1
	declIndent := -1
	decoratorStart := -1

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		indent := countIndent(raw)

		// Track decorators
		if strings.HasPrefix(trimmed, "@") {
			if decoratorStart < 0 {
				decoratorStart = i
			}
			continue
		}

		// Skip lambda expressions (they're not declarations)
		if pyLambdaRe.MatchString(trimmed) {
			continue
		}

		var name, kind string
		isDecl := false

		if m := pyClassPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "class"
			isDecl = true
		} else if m := pyFuncPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[2], "function"
			isDecl = true
		}

		if isDecl {
			if inDecl >= 0 {
				decls[len(decls)-1].endLine = i
			}

			declLine := i
			if decoratorStart >= 0 {
				declLine = decoratorStart
			}

			inDecl = i
			declIndent = indent
			col := strings.Index(raw, name)
			if col < 0 {
				col = strings.Index(trimmed, name)
			}
			decls = append(decls, declInfo{
				line:    declLine,
				endLine: -1,
				symbols: []Symbol{{Name: name, Kind: kind, Line: i, Col: col}},
			})
			decoratorStart = -1
			continue
		}

		decoratorStart = -1

		// Check if we're inside a declaration and indentation returned to base level
		if inDecl >= 0 && indent <= declIndent && trimmed != "" {
			decls[len(decls)-1].endLine = i
			inDecl = -1
		}
	}

	if inDecl >= 0 && len(decls) > 0 {
		decls[len(decls)-1].endLine = len(lines)
	}

	chunks := buildChunks(lines, decls)
	if chunks == nil {
		return lineBasedChunk(content, 50, 10)
	}
	return chunks
}

func countIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 4
		} else {
			break
		}
		if !unicode.IsSpace(ch) {
			break
		}
	}
	return count
}
