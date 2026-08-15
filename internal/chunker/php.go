package chunker

import (
	"regexp"
	"strings"
)

var (
	phpFnPat    = regexp.MustCompile(`(?m)^\s*(?:public|private|protected|static|abstract|final)?\s*(?:static\s+)?function\s+&?\s*([A-Za-z_]\w*)\s*\(`)
	phpClassPat = regexp.MustCompile(`(?m)^\s*(?:abstract|final|readonly)?\s*(class|interface|trait|enum)\s+([A-Za-z_]\w*)`)
	phpSkipPat  = regexp.MustCompile(`(?i)^\s*(namespace|use|require|include|declare|strict_types)\s`)
	phpComment  = regexp.MustCompile(`^\s*(//|#|/\*)`)
)

func PHPChunker(content string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}
	decls := findPHPDecls(lines)
	chunks := jsDeclsToChunks(lines, toJSDeclsFromPhp(decls))
	if chunks == nil {
		return lineBasedChunk(content, 50, 0)
	}
	return chunks
}

type phpDecl struct {
	line    int
	endLine int
	name    string
	kind    string
	col     int
}

func findPHPDecls(lines []string) []phpDecl {
	var decls []phpDecl
	i := 0
	for i < len(lines) {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" || phpComment.MatchString(trimmed) {
			i++
			continue
		}

		if phpSkipPat.MatchString(trimmed) {
			i++
			continue
		}

		if strings.HasPrefix(trimmed, "<?") || strings.HasPrefix(trimmed, "?>") {
			i++
			continue
		}

		var name, kind string
		found := false
		col := 0

		if m := phpClassPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[2], strings.ToLower(m[1])
			found = true
			col = strings.Index(raw, name)
		} else if m := phpFnPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "function"
			found = true
			col = strings.Index(raw, name)
		}

		if found {
			if col < 0 {
				col = 0
			}

			endLine := i
			if strings.ContainsRune(raw, '{') {
				endLine = findMatchingBrace(lines, i)
			} else if strings.ContainsRune(raw, ';') {
				endLine = i
			} else {
				for j := i + 1; j < len(lines); j++ {
					if strings.ContainsRune(lines[j], '{') {
						endLine = findMatchingBrace(lines, j)
						break
					}
					if strings.TrimSpace(lines[j]) != "" && !phpComment.MatchString(strings.TrimSpace(lines[j])) {
						break
					}
				}
			}

			decls = append(decls, phpDecl{
				line:    i,
				endLine: endLine,
				name:    name,
				kind:    kind,
				col:     col,
			})
			i = endLine + 1
			continue
		}
		i++
	}
	return decls
}
