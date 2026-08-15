package chunker

import (
	"regexp"
	"strings"
)

var (
	// C, C++
	cppClassPat     = regexp.MustCompile(`(?m)^\s*(?:typedef\s+)?(?:class|struct|union|enum\s+class|enum\s+struct)\s+([A-Za-z_]\w*)`)
	cppInterfacePat = regexp.MustCompile(`(?m)^\s*(?:public\s+)?interface\s+([A-Za-z_]\w*)`)
	cppNamespacePat = regexp.MustCompile(`(?m)^\s*namespace\s+([A-Za-z_]\w*(?:::\w+)*)`)
	cppFnPat        = regexp.MustCompile(`(?m)^\s*(?:virtual|static|inline|constexpr|extern|template\s*<[^>]*>\s*)?(?:\w+(?:<[^>]*>)?(?:\s*\*+|\s*&)?\s+)+(?:operator\s*)?(?:~)?([A-Za-z_]\w*)\s*\(`)
	cppIfDefPat     = regexp.MustCompile(`^\s*#\s*(?:include|define|if|ifdef|ifndef|endif|pragma|undef|error|warning|line|region|endregion)`)
	cppAccessPat    = regexp.MustCompile(`^\s*(public|private|protected)\s*:`)
	cppAnnotation   = regexp.MustCompile(`^\[.*\]`)
	// C#
	csharpRecordPat   = regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal|sealed|abstract|readonly|record)?\s*(?:record|record\s+class|record\s+struct)\s+([A-Za-z_]\w*)`)
	csharpEventPat    = regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal)\s+event\s+\w+\s+([A-Za-z_]\w*)`)
	csharpDelegatePat = regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal)\s+delegate\s+\w+\s+([A-Za-z_]\w*)\s*\(`)
	// Kotlin
	kotlinFunPat   = regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal|override|suspend|inline|operator|infix|tailrec|external)?\s*(?:fun\s+([A-Za-z_]\w*))\s*\(`)
	kotlinClassPat = regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal|abstract|open|sealed|data|value)?\s*(?:class|object|companion\s+object|enum\s+class|sealed\s+class|data\s+class|value\s+class|annotation\s+class)\s+([A-Za-z_]\w*)`)
	kotlinIfPat    = regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal)?\s*(?:interface|annotation)\s+([A-Za-z_]\w*)`)
	// Swift
	swiftFuncPat  = regexp.MustCompile(`(?m)^\s*(?:public|private|internal|fileprivate|open|static|class|override|mutating|nonmutating|discardableResult)?\s*(?:func\s+([A-Za-z_]\w*))\s*\(`)
	swiftClassPat = regexp.MustCompile(`(?m)^\s*(?:public|private|internal|fileprivate|open|final)?\s*(?:class|struct|enum|protocol|extension)\s+([A-Za-z_]\w*)`)
	// generic skip
	cppUsingPat = regexp.MustCompile(`(?i)^\s*(using\s|import\s)`)
)

func CppChunker(content string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}
	decls := findCppDecls(lines)
	chunks := jsDeclsToChunks(lines, toJSDeclsFromCpp(decls))
	if chunks == nil {
		return lineBasedChunk(content, 50, 0)
	}
	return chunks
}

type cppDecl struct {
	line    int
	endLine int
	name    string
	kind    string
	col     int
}

func findCppDecls(lines []string) []cppDecl {
	var decls []cppDecl
	inTemplate := false
	i := 0
	for i < len(lines) {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" {
			i++
			continue
		}

		if cppIfDefPat.MatchString(trimmed) {
			i++
			continue
		}

		if cppAccessPat.MatchString(trimmed) {
			i++
			continue
		}

		if cppAnnotation.MatchString(trimmed) {
			i++
			continue
		}

		if cppUsingPat.MatchString(trimmed) {
			i++
			continue
		}

		if strings.HasPrefix(trimmed, "template") {
			inTemplate = true
			i++
			continue
		}

		var name, kind string
		found := false
		col := 0

		if m := csharpRecordPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "record"
			found = true
			col = strings.Index(raw, name)
		} else if m := csharpDelegatePat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "delegate"
			found = true
			col = strings.Index(raw, name)
		} else if m := csharpEventPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "event"
			found = true
			col = strings.Index(raw, name)
		} else if m := kotlinClassPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "class"
			found = true
			col = strings.Index(raw, name)
		} else if m := kotlinFunPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "function"
			found = true
			col = strings.Index(raw, name)
		} else if m := kotlinIfPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "interface"
			found = true
			col = strings.Index(raw, name)
		} else if m := swiftFuncPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "function"
			found = true
			col = strings.Index(raw, name)
		} else if m := swiftClassPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "class"
			found = true
			col = strings.Index(raw, name)
		} else if m := cppClassPat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "class"
			found = true
			col = strings.Index(raw, name)
		} else if m := cppInterfacePat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "interface"
			found = true
			col = strings.Index(raw, name)
		} else if m := cppNamespacePat.FindStringSubmatch(trimmed); m != nil {
			name, kind = m[1], "module"
			found = true
			col = strings.Index(raw, name)
		} else if m := cppFnPat.FindStringSubmatch(trimmed); m != nil && !isCppControlFlow(trimmed) {
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
					if strings.TrimSpace(lines[j]) != "" &&
						!cppAccessPat.MatchString(lines[j]) &&
						!cppAnnotation.MatchString(strings.TrimSpace(lines[j])) {
						break
					}
				}
			}

			decls = append(decls, cppDecl{
				line:    inTemplateStripped(i, inTemplate),
				endLine: endLine,
				name:    name,
				kind:    kind,
				col:     col,
			})
			inTemplate = false
			i = endLine + 1
			continue
		}

		inTemplate = false
		i++
	}
	return decls
}

func isCppControlFlow(trimmed string) bool {
	keywords := []string{"if", "for", "while", "switch", "catch", "do"}
	for _, kw := range keywords {
		pat := regexp.MustCompile(`^\s*` + kw + `\s*\(`)
		if pat.MatchString(trimmed) {
			return true
		}
	}
	return false
}

func inTemplateStripped(i int, inTemplate bool) int {
	if inTemplate && i > 0 {
		return i - 1
	}
	return i
}
