package chunker

import (
	"regexp"
	"strings"
)

var (
	jsFuncPat       = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\*?\s+([A-Za-z_$][\w$]*)`)
	jsClassPat      = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:default\s+)?class\s+([A-Za-z_$][\w$]*)`)
	jsMethodPat     = regexp.MustCompile(`(?m)^\s*(?:(?:get|set)\s+)?([A-Za-z_$][\w$]*)\s*\([^)]*\)\s*\{`)
	jsArrowPatFull  = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\(`)
	jsArrowPatShort = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s+)?([A-Za-z_$][\w$]*)\s*=>`)
	jsRustFnPat     = regexp.MustCompile(`(?m)^\s*(?:pub(?:\(\w+\)\s+)?)?(?:unsafe\s+)?(?:async\s+)?fn\s+([A-Za-z_$][\w$]*)`)
	jsRustImplPat   = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?(?:unsafe\s+)?impl\s+(?:(?:<[^>]*>)\s+)?([A-Za-z_$][\w$]*)`)
	jsRustTraitPat  = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?trait\s+([A-Za-z_$][\w$]*)`)
	jsRustStructPat = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?struct\s+([A-Za-z_$][\w$]*)`)
	jsRustEnumPat   = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?enum\s+([A-Za-z_$][\w$]*)`)
	jsCommentRe     = regexp.MustCompile(`(?m)^\s*(//|/\*|\*)`)
	jsKeywords      = map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
	}
)

type jsDecl struct {
	line    int
	endLine int
	name    string
	kind    string
	col     int
}

func isJSDeclStart(line string) (name, kind string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || jsCommentRe.MatchString(line) {
		return "", "", false
	}

	if m := jsFuncPat.FindStringSubmatch(trimmed); m != nil {
		return m[1], "function", true
	}
	if m := jsClassPat.FindStringSubmatch(trimmed); m != nil {
		return m[1], "class", true
	}
	if m := jsArrowPatFull.FindStringSubmatch(trimmed); m != nil {
		return m[1], "function", true
	}
	if m := jsArrowPatShort.FindStringSubmatch(trimmed); m != nil {
		if m[1] != m[2] {
			return m[1], "function", true
		}
	}
	if m := jsMethodPat.FindStringSubmatch(trimmed); m != nil {
		if !jsKeywords[m[1]] {
			return m[1], "function", true
		}
	}
	if m := jsRustFnPat.FindStringSubmatch(trimmed); m != nil {
		return m[1], "function", true
	}
	if m := jsRustImplPat.FindStringSubmatch(trimmed); m != nil {
		return m[1], "impl", true
	}
	if m := jsRustTraitPat.FindStringSubmatch(trimmed); m != nil {
		return m[1], "trait", true
	}
	if m := jsRustStructPat.FindStringSubmatch(trimmed); m != nil {
		return m[1], "struct", true
	}
	if m := jsRustEnumPat.FindStringSubmatch(trimmed); m != nil {
		return m[1], "enum", true
	}
	return "", "", false
}

func findMatchingBrace(lines []string, start int) int {
	depth := 0
	started := false
	for i := start; i < len(lines); i++ {
		line := lines[i]
		for _, r := range line {
			switch r {
			case '{':
				depth++
				started = true
			case '}':
				if depth > 0 {
					depth--
					if started && depth == 0 {
						return i
					}
				}
			}
		}
	}
	return len(lines) - 1
}

func findJSDecls(lines []string) []jsDecl {
	var decls []jsDecl
	i := 0
	for i < len(lines) {
		line := lines[i]
		name, kind, ok := isJSDeclStart(line)
		if !ok {
			i++
			continue
		}

		endLine := i
		if strings.ContainsRune(line, '{') {
			endLine = findMatchingBrace(lines, i)
		} else {
			for j := i + 1; j < len(lines); j++ {
				if strings.ContainsRune(lines[j], '{') {
					endLine = findMatchingBrace(lines, j)
					break
				}
				if strings.TrimSpace(lines[j]) != "" {
					break
				}
			}
		}
		col := strings.Index(line, name)
		if col < 0 {
			col = 0
		}
		decls = append(decls, jsDecl{
			line:    i,
			endLine: endLine,
			name:    name,
			kind:    kind,
			col:     col,
		})
		i = endLine + 1
	}
	return decls
}

func jsDeclsToChunks(lines []string, decls []jsDecl) []Chunk {
	if len(decls) == 0 {
		return nil
	}
	var chunks []Chunk
	for idx, d := range decls {
		end := d.endLine
		if end >= len(lines) {
			end = len(lines) - 1
		}
		if d.line > end {
			end = d.line
		}
		content := strings.Join(lines[d.line:end+1], "\n")
		chunks = append(chunks, Chunk{
			Content:   content,
			StartLine: d.line,
			EndLine:   end,
			Index:     idx,
			Symbols:   []Symbol{{Name: d.name, Kind: d.kind, Line: d.line, Col: d.col}},
		})
	}
	return chunks
}

func JSChunker(content string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}
	decls := findJSDecls(lines)
	chunks := jsDeclsToChunks(lines, decls)
	if chunks == nil {
		return lineBasedChunk(content, 50, 0)
	}
	return chunks
}

func RustChunker(content string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}
	decls := findJSDecls(lines)
	chunks := jsDeclsToChunks(lines, decls)
	if chunks == nil {
		return lineBasedChunk(content, 50, 0)
	}
	return chunks
}
