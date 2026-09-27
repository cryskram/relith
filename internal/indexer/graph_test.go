package indexer

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/cryskram/relith/internal/db"
)

func writeGraphTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractImportsPackageLangs(t *testing.T) {
	tests := []struct {
		name       string
		lang       string
		mainPath   string
		mainBody   string
		depPath    string
		depBody    string
		wantEdges  int
		wantTarget string // resolved doc path
	}{
		{
			name:       "Java",
			lang:       "Java",
			mainPath:   filepath.Join("src", "main", "java", "com", "example", "Main.java"),
			mainBody:   "package com.example;\n\nimport com.example.util.StringUtils;\n\npublic class Main {\n    StringUtils utils;\n}\n",
			depPath:    filepath.Join("src", "main", "java", "com", "example", "util", "StringUtils.java"),
			depBody:    "package com.example.util;\n\npublic class StringUtils {\n}\n",
			wantEdges:  1,
			wantTarget: filepath.Join("src", "main", "java", "com", "example", "util", "StringUtils.java"),
		},
		{
			name:       "Kotlin",
			lang:       "Kotlin",
			mainPath:   filepath.Join("kotlin", "com", "example", "App.kt"),
			mainBody:   "package com.example\n\nimport com.example.util.Helper\n\nfun main() { val h = Helper() }\n",
			depPath:    filepath.Join("kotlin", "com", "example", "util", "Helper.kt"),
			depBody:    "package com.example.util\n\nclass Helper\n",
			wantEdges:  1,
			wantTarget: filepath.Join("kotlin", "com", "example", "util", "Helper.kt"),
		},
		{
			name:       "CSharp",
			lang:       "C#",
			mainPath:   filepath.Join("src", "App", "Services", "UserService.cs"),
			mainBody:   "namespace App.Services;\n\nusing App.Utils.NameHelper;\n\nclass UserService {\n    NameHelper helper;\n}\n",
			depPath:    filepath.Join("src", "App", "Utils", "NameHelper.cs"),
			depBody:    "namespace App.Utils;\n\nclass NameHelper {\n}\n",
			wantEdges:  1,
			wantTarget: filepath.Join("src", "App", "Utils", "NameHelper.cs"),
		},
		{
			name:       "PHP",
			lang:       "PHP",
			mainPath:   filepath.Join("src", "App", "Controller", "HomeController.php"),
			mainBody:   "<?php\n\nnamespace App\\Controller;\n\nuse App\\Service\\UserService;\n\nclass HomeController {\n    private UserService $service;\n}\n",
			depPath:    filepath.Join("src", "App", "Service", "UserService.php"),
			depBody:    "<?php\n\nnamespace App\\Service;\n\nclass UserService {\n}\n",
			wantEdges:  1,
			wantTarget: filepath.Join("src", "App", "Service", "UserService.php"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			writeGraphTestFile(t, filepath.Join(repoRoot, tc.mainPath), tc.mainBody)
			writeGraphTestFile(t, filepath.Join(repoRoot, tc.depPath), tc.depBody)

			docByPath := map[string]db.Document{
				tc.mainPath: {ID: 1, Path: tc.mainPath, Language: sql.NullString{String: tc.lang, Valid: true}},
				tc.depPath:  {ID: 2, Path: tc.depPath, Language: sql.NullString{String: tc.lang, Valid: true}},
			}

			edges, err := (&Indexer{}).extractImportsForDoc(docByPath[tc.mainPath], repoRoot, docByPath)
			if err != nil {
				t.Fatalf("extractImportsForDoc: %v", err)
			}
			if len(edges) != tc.wantEdges {
				t.Fatalf("expected %d import edge(s), got %d: %+v", tc.wantEdges, len(edges), edges)
			}
			if tc.wantTarget != "" && edges[0].TargetDocID != 2 {
				t.Errorf("edge target = %d, want doc ID 2 (%s)", edges[0].TargetDocID, tc.wantTarget)
			}
			if edges[0].Kind != "imports" {
				t.Errorf("edge kind = %q, want %q", edges[0].Kind, "imports")
			}
		})
	}
}

func TestImportCapableLangExpanded(t *testing.T) {
	for _, lang := range []string{"Go", "JavaScript", "TypeScript", "Python", "Rust", "Java", "Kotlin", "C#", "PHP"} {
		if !importCapableLang(lang) {
			t.Errorf("importCapableLang(%q) = false, want true", lang)
		}
	}
	for _, lang := range []string{"C", "C++", "Ruby", "Swift", "SQL"} {
		if importCapableLang(lang) {
			t.Errorf("importCapableLang(%q) = true, want false", lang)
		}
	}
}
