package chunker

import (
	"regexp"
	"strings"
)

var (
	braceClassPat   = regexp.MustCompile(`(?i)\b(class|struct|interface|trait|enum)\s+([A-Za-z_]\w*)`)
	braceFnPat      = regexp.MustCompile(`(?i)(?:async\s+)?(?:function\s*\*?\s*)([A-Za-z_]\w*)\s*\(`)
	braceMethodPat  = regexp.MustCompile(`(?i)^\s*(?:get|set)\s+([A-Za-z_]\w*)\s*\(`)
	braceArrowPat   = regexp.MustCompile(`(?i)(?:const|let|var)\s+([A-Za-z_]\w*)\s*=\s*(?:async\s*)?(?:\(|$)`)
	braceRustImplPat = regexp.MustCompile(`(?i)impl\s+([A-Za-z_]\w*)`)
	braceRustFnPat   = regexp.MustCompile(`(?i)(?:pub\s+)?(?:unsafe\s+)?fn\s+([A-Za-z_]\w*)`)
	braceAnnotation  = regexp.MustCompile(`^\s*@\w+`)

	braceSkipPats = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^\s*(import|package|using|namespace|#include|#define|#if|#endif|#pragma|#region|#endregion)\s`),
		regexp.MustCompile(`^\s*@\w+`),
	}
)

func BraceChunker(content string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	var decls []declInfo
	braceDepth := 0
	inDecl := -1

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" {
			updateBraceDecl(&braceDepth, raw)
			if inDecl >= 0 && braceDepth == 0 {
				decls[len(decls)-1].endLine = i + 1
				inDecl = -1
			}
			continue
		}

		isSkip := false
		for _, p := range braceSkipPats {
			if p.MatchString(trimmed) {
				isSkip = true
				break
			}
		}
		if isSkip {
			updateBraceDecl(&braceDepth, raw)
			if inDecl >= 0 && braceDepth == 0 {
				decls[len(decls)-1].endLine = i + 1
				inDecl = -1
			}
			continue
		}

		var name, kind string
		col := 0
		isDecl := false

		if m := braceClassPat.FindStringSubmatch(trimmed); m != nil && !insideMethodDecl(trimmed) {
			name = m[2]
			kind = strings.ToLower(m[1])
			col = strings.Index(raw, name)
			isDecl = true
		} else if m := braceFnPat.FindStringSubmatch(trimmed); m != nil {
			name = m[1]
			kind = "function"
			col = strings.Index(raw, name)
			isDecl = true
		} else if m := braceRustFnPat.FindStringSubmatch(trimmed); m != nil {
			name = m[1]
			kind = "function"
			col = strings.Index(raw, name)
			isDecl = true
		} else if m := braceRustImplPat.FindStringSubmatch(trimmed); m != nil {
			name = m[1]
			kind = "impl"
			col = strings.Index(raw, name)
			isDecl = true
		} else if m := braceArrowPat.FindStringSubmatch(trimmed); m != nil {
			name = m[1]
			kind = "function"
			col = strings.Index(raw, name)
			isDecl = true
		} else if m := braceMethodPat.FindStringSubmatch(trimmed); m != nil {
			name = m[1]
			kind = "function"
			col = strings.Index(raw, name)
			isDecl = true
		}

		if isDecl {
			if inDecl >= 0 {
				decls[len(decls)-1].endLine = i
			}

			rawBrace := strings.Count(raw, "{") - strings.Count(raw, "}")
			trimBrace := strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

			if rawBrace > 0 || (rawBrace == 0 && strings.Contains(trimmed, "=>") && strings.Contains(raw, "{")) {
				cb := rawBrace
				if cb < 0 {
					cb = 0
				}
				if cb > 0 {
					braceDepth = cb - 1
				} else {
					braceDepth = 0
				}
				inDecl = i
				decls = append(decls, declInfo{
					line:    i,
					endLine: -1,
					symbols: []Symbol{{Name: name, Kind: kind, Line: i, Col: col}},
				})
				if braceDepth == 0 && rawBrace == 0 {
					decls[len(decls)-1].endLine = i + 1
					inDecl = -1
				}
			} else if trimBrace > 0 {
				braceDepth = trimBrace - 1
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
				braceDepth = 0
				inDecl = -1
				decls = append(decls, declInfo{
					line:    i,
					endLine: i + 1,
					symbols: []Symbol{{Name: name, Kind: kind, Line: i, Col: col}},
				})
			}
			continue
		}

		updateBraceDecl(&braceDepth, raw)
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

func updateBraceDecl(depth *int, line string) {
	bb := strings.Count(line, "{") - strings.Count(line, "}")
	*depth += bb
	if *depth < 0 {
		*depth = 0
	}
}

func insideMethodDecl(trimmed string) bool {
	return strings.HasPrefix(trimmed, "\t") || strings.HasPrefix(trimmed, "  ")
}
