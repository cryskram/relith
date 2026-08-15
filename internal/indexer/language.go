package indexer

import "strings"

var extLangMap = map[string]string{
	".go":         "Go",
	".py":         "Python",
	".js":         "JavaScript",
	".ts":         "TypeScript",
	".tsx":        "TypeScript",
	".jsx":        "JavaScript",
	".rs":         "Rust",
	".java":       "Java",
	".kt":         "Kotlin",
	".kts":        "Kotlin",
	".scala":      "Scala",
	".swift":      "Swift",
	".c":          "C",
	".h":          "C",
	".cpp":        "C++",
	".hpp":        "C++",
	".cc":         "C++",
	".cxx":        "C++",
	".hh":         "C++",
	".cs":         "C#",
	".fs":         "F#",
	".rb":         "Ruby",
	".php":        "PHP",
	".pl":         "Perl",
	".pm":         "Perl",
	".r":          "R",
	".m":          "Objective-C",
	".mm":         "Objective-C",
	".zig":        "Zig",
	".nim":        "Nim",
	".ex":         "Elixir",
	".exs":        "Elixir",
	".erl":        "Erlang",
	".hrl":        "Erlang",
	".clj":        "Clojure",
	".cljs":       "Clojure",
	".cljc":       "Clojure",
	".lisp":       "Lisp",
	".lsp":        "Lisp",
	".el":         "Emacs Lisp",
	".lua":        "Lua",
	".hs":         "Haskell",
	".ml":         "OCaml",
	".mli":        "OCaml",
	".caml":       "OCaml",
	".sml":        "Standard ML",
	".sql":        "SQL",
	".sh":         "Shell",
	".bash":       "Shell",
	".zsh":        "Shell",
	".fish":       "Shell",
	".ps1":        "PowerShell",
	".psm1":       "PowerShell",
	".bat":        "Batch",
	".cmd":        "Batch",
	".makefile":   "Makefile",
	".mk":         "Makefile",
	".dockerfile": "Dockerfile",
	".yaml":       "YAML",
	".yml":        "YAML",
	".json":       "JSON",
	".toml":       "TOML",
	".ini":        "INI",
	".cfg":        "INI",
	".conf":       "INI",
	".md":         "Markdown",
	".rst":        "reStructuredText",
	".adoc":       "AsciiDoc",
	".tex":        "LaTeX",
	".bib":        "BibTeX",
	".html":       "HTML",
	".htm":        "HTML",
	".css":        "CSS",
	".scss":       "SCSS",
	".less":       "Less",
	".sass":       "Sass",
	".xml":        "XML",
	".svg":        "SVG",
	".graphql":    "GraphQL",
	".gql":        "GraphQL",
	".proto":      "Protocol Buffers",
	".thrift":     "Thrift",
	".dart":       "Dart",
	".groovy":     "Groovy",
	".gradle":     "Groovy",
	".vue":        "Vue",
	".svelte":     "Svelte",
	".astro":      "Astro",
	".sqlite":     "SQL",
	".db":         "SQL",
}

var codeLangs = map[string]bool{
	"Go": true, "Python": true, "JavaScript": true, "TypeScript": true,
	"Rust": true, "Java": true, "Kotlin": true, "Scala": true,
	"Swift": true, "C": true, "C++": true, "C#": true, "F#": true,
	"Ruby": true, "PHP": true, "Perl": true, "R": true,
	"Objective-C": true, "Zig": true, "Nim": true,
	"Elixir": true, "Erlang": true, "Clojure": true,
	"Lisp": true, "Emacs Lisp": true, "Lua": true,
	"Haskell": true, "OCaml": true, "Standard ML": true,
	"Shell": true, "PowerShell": true, "Batch": true,
	"Makefile": true, "Dart": true, "Groovy": true,
	"Vue": true, "Svelte": true, "Astro": true,
	"GraphQL": true, "Protocol Buffers": true, "Thrift": true,
	"SQL": true,
}

func isCodeLang(lang string) bool {
	return codeLangs[lang]
}

func DetectLanguage(path string) string {
	lower := strings.ToLower(path)
	for ext, lang := range extLangMap {
		if strings.HasSuffix(lower, ext) {
			return lang
		}
	}
	if strings.HasSuffix(lower, "makefile") {
		return "Makefile"
	}
	if strings.HasSuffix(lower, "dockerfile") {
		return "Dockerfile"
	}
	return ""
}
