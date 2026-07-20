package chunker

import (
	"regexp"
	"strings"
)

var (
	goFuncPat   = regexp.MustCompile(`^func\s+(\([^)]*\)\s+)?([A-Za-z_]\w*)`)
	goTypePat   = regexp.MustCompile(`^type\s+([A-Za-z_]\w*)(\[[^]]*\])?\s+`)
	goStructPat = regexp.MustCompile(`^type\s+([A-Za-z_]\w*)(\[[^]]*\])?\s+struct(\s*\{|\s*//|$)`)
	goIfacePat  = regexp.MustCompile(`^type\s+([A-Za-z_]\w*)(\[[^]]*\])?\s+interface(\s*\{|\s*//|$)`)
)

func GoChunker(content string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	var decls []declInfo
	braceDepth := 0
	inDecl := -1
	inString := false
	inLineComment := false
	stringChar := byte(0)

	updateBraces := func(line string, i int) {
		if inString || inLineComment {
			return
		}
		bb := 0
		for j := 0; j < len(line); j++ {
			ch := line[j]
			if ch == '"' || ch == '`' || ch == '\'' {
				if !inString {
					inString = true
					stringChar = ch
				} else if ch == stringChar {
					inString = false
				}
				continue
			}
			if ch == '/' && j+1 < len(line) && line[j+1] == '/' {
				inLineComment = true
				break
			}
			if inString {
				continue
			}
			if ch == '{' {
				bb++
			} else if ch == '}' {
				bb--
			}
		}
		braceDepth += bb
		if braceDepth < 0 {
			braceDepth = 0
		}
		inLineComment = false
	}

	for i := 0; i < len(lines); i++ {
		rawLine := lines[i]
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			updateBraces(rawLine, i)
			if inDecl >= 0 && braceDepth == 0 {
				decls[len(decls)-1].endLine = i + 1
				inDecl = -1
			}
			continue
		}

		// Skip comment-only, package, import lines
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "import") {
			updateBraces(rawLine, i)
			if inDecl >= 0 && braceDepth == 0 {
				decls[len(decls)-1].endLine = i + 1
				inDecl = -1
			}
			continue
		}

		var name, kind string
		col := 0
		isDecl := false

		if m := goFuncPat.FindStringSubmatch(trimmed); m != nil {
			name = m[2]
			kind = "function"
			if strings.HasPrefix(trimmed, "func (") {
				kind = "method"
			}
			col = strings.Index(rawLine, name)
			if col < 0 {
				col = strings.Index(trimmed, name)
			}
			isDecl = true
		} else if m := goTypePat.FindStringSubmatch(trimmed); m != nil {
			name = m[1]
			col = strings.Index(rawLine, name)
			if col < 0 {
				col = strings.Index(trimmed, name)
			}
			if goStructPat.MatchString(trimmed) {
				kind = "struct"
			} else if goIfacePat.MatchString(trimmed) {
				kind = "interface"
			} else {
				kind = "type"
			}
			// Check if the type has a brace body
			if !strings.Contains(trimmed, "{") {
				// Must check if next line continues with brace
				hasBrace := false
				if i+1 < len(lines) && strings.Contains(lines[i+1], "{") {
					hasBrace = true
				}
				if !hasBrace {
					kind = "type"
				}
			}
			isDecl = true
		}

		if isDecl {
			if inDecl >= 0 {
				decls[len(decls)-1].endLine = i
			}

			hasBrace := strings.Contains(rawLine, "{")

			if hasBrace {
				cBraces := strings.Count(rawLine, "{") - strings.Count(rawLine, "}")
				if cBraces > 0 {
					braceDepth = cBraces - 1
				} else {
					braceDepth = 0
				}
				inDecl = i
				decls = append(decls, declInfo{
					line:    i,
					endLine: -1,
					symbols: []Symbol{{Name: name, Kind: kind, Line: i, Col: col}},
				})
				if braceDepth == 0 {
					decls[len(decls)-1].endLine = i + 1
					inDecl = -1
				}
			} else {
				// Check if next line starts with { (type Foo struct\n{)
				if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "{" {
					inDecl = i
					braceDepth = 0
					decls = append(decls, declInfo{
						line:    i,
						endLine: -1,
						symbols: []Symbol{{Name: name, Kind: kind, Line: i, Col: col}},
					})
				} else {
					braceDepth = 0
					inDecl = -1
					decls = append(decls, declInfo{
						line:    i,
						endLine: i + 1,
						symbols: []Symbol{{Name: name, Kind: kind, Line: i, Col: col}},
					})
				}
			}
			continue
		}

		updateBraces(rawLine, i)
		if inDecl >= 0 && braceDepth == 0 {
			decls[len(decls)-1].endLine = i + 1
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
