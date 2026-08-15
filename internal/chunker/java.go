package chunker

import (
	"regexp"
	"strings"
)

var (
	javaClassPat      = regexp.MustCompile(`(?m)^\s*(?:public|private|protected|static|abstract|final|sealed|non-sealed|strictfp)\s+(?:class|@interface|interface|enum|record)\s+([A-Za-z_]\w*)`)
	javaInnerClassPat = regexp.MustCompile(`(?m)^\s*(class|@interface|interface|enum|record)\s+([A-Za-z_]\w*)`)
	javaMethodPrefix  = regexp.MustCompile(`(?m)^\s*(?:public|private|protected|static|abstract|final|synchronized|native|strictfp)\s`)
	javaAnnotation    = regexp.MustCompile(`^\s*@\w+`)
	javaSkipPat       = regexp.MustCompile(`(?i)^\s*(package|import)\s`)
	javaKeywords      = map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"new": true, "return": true, "throw": true, "assert": true,
		"class": true, "interface": true, "enum": true, "record": true,
	}
)

func JavaChunker(content string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	decls := findJavaDecls(lines)
	chunks := jsDeclsToChunks(lines, toJSDeclsFromJava(decls))
	if chunks == nil {
		return lineBasedChunk(content, 50, 0)
	}
	return chunks
}

type javaDecl struct {
	line    int
	endLine int
	name    string
	kind    string
	col     int
}

func findJavaDecls(lines []string) []javaDecl {
	var decls []javaDecl
	annotationLine := -1
	i := 0
	for i < len(lines) {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" {
			i++
			continue
		}

		if javaAnnotation.MatchString(trimmed) {
			if annotationLine < 0 {
				annotationLine = i
			}
			i++
			continue
		}

		if javaSkipPat.MatchString(trimmed) {
			annotationLine = -1
			i++
			continue
		}

		var name, kind string
		found := false
		col := 0

		if m := javaClassPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "class"
			found = true
		} else if m := javaInnerClassPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[2], strings.ToLower(m[1])
			if strings.HasPrefix(kind, "@") {
				kind = "annotation"
			}
			found = true
		} else if n, ok := extractJavaMethod(trimmed, raw); ok {
			name, kind = n, "function"
			found = true
		}

		if !found && javaMethodPrefix.MatchString(trimmed) {
			// Constructor: ClassName(args) { ... } with no return type
			if n, ok := extractJavaConstructor(trimmed); ok {
				name, kind = n, "function"
				found = true
			}
		}

		if found {
			declLine := i
			if annotationLine >= 0 {
				declLine = annotationLine
			}

			col = strings.Index(raw, name)
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
					if strings.TrimSpace(lines[j]) != "" && !javaAnnotation.MatchString(strings.TrimSpace(lines[j])) {
						break
					}
				}
			}

			decls = append(decls, javaDecl{
				line:    declLine,
				endLine: endLine,
				name:    name,
				kind:    kind,
				col:     col,
			})

			i = endLine + 1
			annotationLine = -1
			continue
		}

		annotationLine = -1
		i++
	}
	return decls
}

func extractJavaMethod(trimmed, raw string) (string, bool) {
	if !javaMethodPrefix.MatchString(trimmed) {
		return "", false
	}

	parenIdx := strings.Index(trimmed, "(")
	if parenIdx < 0 {
		return "", false
	}

	if strings.Contains(trimmed, "=") && parenIdx > strings.Index(trimmed, "=") {
		return "", false
	}

	beforeParen := strings.TrimSpace(trimmed[:parenIdx])
	parts := strings.Fields(beforeParen)
	if len(parts) < 2 {
		return "", false
	}

	name := parts[len(parts)-1]

	if !regexp.MustCompile(`^[A-Za-z_]\w*$`).MatchString(name) {
		return "", false
	}

	if javaKeywords[name] {
		return "", false
	}

	if name == strings.ToUpper(name[:1])+name[1:] {
		return "", false
	}

	return name, true
}

func extractJavaConstructor(trimmed string) (string, bool) {
	parenIdx := strings.Index(trimmed, "(")
	if parenIdx < 0 {
		return "", false
	}

	beforeParen := strings.TrimSpace(trimmed[:parenIdx])
	parts := strings.Fields(beforeParen)
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return "", false
	}

	name := parts[len(parts)-1]
	if name[0] < 'A' || name[0] > 'Z' {
		return "", false
	}

	if strings.Contains(trimmed, "void") {
		return "", false
	}

	return name, true
}
