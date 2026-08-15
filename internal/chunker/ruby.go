package chunker

import (
	"regexp"
	"strings"
)

var (
	rubyDefPat    = regexp.MustCompile(`(?m)^\s*(?:def\s+(?:self\.)?)([A-Za-z_]\w*)(?:\s*\(|\s|$)`)
	rubyClassPat  = regexp.MustCompile(`(?m)^\s*class\s+([A-Za-z_]\w*(?:::\w+)*(?:\s*<\s*[A-Za-z_]\w*)?)`)
	rubyModulePat = regexp.MustCompile(`(?m)^\s*module\s+([A-Za-z_]\w*(?:::\w+)*)`)
	rubyComment   = regexp.MustCompile(`^\s*#`)
)

func RubyChunker(content string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}
	decls := findRubyDecls(lines)
	chunks := jsDeclsToChunks(lines, toJSDeclsFromRuby(decls))
	if chunks == nil {
		return lineBasedChunk(content, 50, 0)
	}
	return chunks
}

type rubyDecl struct {
	line    int
	endLine int
	name    string
	kind    string
	col     int
}

func findRubyDecls(lines []string) []rubyDecl {
	var decls []rubyDecl
	keywordStack := make([]string, 0)
	i := 0
	for i < len(lines) {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" || rubyComment.MatchString(trimmed) {
			i++
			continue
		}

		if trimmed == "end" {
			if len(keywordStack) > 0 {
				keywordStack = keywordStack[:len(keywordStack)-1]
			}
			i++
			continue
		}

		if strings.HasPrefix(trimmed, "end ") {
			if len(keywordStack) > 0 {
				keywordStack = keywordStack[:len(keywordStack)-1]
			}
			i++
			continue
		}

		var name, kind string
		found := false
		col := 0

		if m := rubyDefPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "function"
			found = true
		} else if m := rubyClassPat.FindStringSubmatch(trimmed); m != nil {
			nameParts := strings.Fields(m[1])
			name = nameParts[0]
			kind = "class"
			found = true
		} else if m := rubyModulePat.FindStringSubmatch(trimmed); m != nil {
			name = m[1]
			kind = "module"
			found = true
		}

		if found {
			col = strings.Index(raw, name)
			if col < 0 {
				col = 0
			}

			keywordStack = append(keywordStack, kind)

			endLine := len(lines) - 1
			if len(keywordStack) > 0 {
				curDepth := len(keywordStack)
				for j := i + 1; j < len(lines); j++ {
					t := strings.TrimSpace(lines[j])
					if t == "end" {
						curDepth--
						if curDepth == 0 {
							endLine = j
							break
						}
					} else if strings.HasPrefix(t, "end ") {
						curDepth--
						if curDepth == 0 {
							endLine = j
							break
						}
					} else if isRubyStartKeyword(t) {
						curDepth++
					}
				}
			}

			decls = append(decls, rubyDecl{
				line:    i,
				endLine: endLine,
				name:    name,
				kind:    kind,
				col:     col,
			})
			i = endLine + 1
			keywordStack = keywordStack[:0]
			continue
		}

		i++
	}
	return decls
}

func isRubyStartKeyword(trimmed string) bool {
	if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "class ") ||
		strings.HasPrefix(trimmed, "module ") {
		return true
	}
	if strings.HasPrefix(trimmed, "if ") || trimmed == "if" ||
		strings.HasPrefix(trimmed, "unless ") || trimmed == "unless" ||
		strings.HasPrefix(trimmed, "case ") || trimmed == "case" ||
		strings.HasPrefix(trimmed, "while ") || trimmed == "while" ||
		strings.HasPrefix(trimmed, "until ") || trimmed == "until" ||
		strings.HasPrefix(trimmed, "for ") || trimmed == "for" ||
		strings.HasPrefix(trimmed, "begin ") || trimmed == "begin" ||
		strings.HasPrefix(trimmed, "do ") || trimmed == "do" ||
		strings.HasPrefix(trimmed, "unless ") {
		return true
	}
	return false
}
