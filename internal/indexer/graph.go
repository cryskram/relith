package indexer

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cryskram/relith/internal/db"
)

func (idx *Indexer) BuildGraphForRepo(ctx context.Context, repoID int64, repoPath string) error {
	idx.logger.Info("building graph edges", "repo_id", repoID)

	q := idx.queries()

	if err := deleteGraphEdgesForRepo(ctx, idx.db, repoID); err != nil {
		return fmt.Errorf("clear graph edges: %w", err)
	}

	docs, err := q.ListDocuments(ctx, repoID)
	if err != nil {
		return fmt.Errorf("list documents: %w", err)
	}

	docByPath := make(map[string]db.Document, len(docs))
	for _, d := range docs {
		docByPath[d.Path] = d
	}

	batchSize := 100
	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}

		tx, err := idx.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}

		for _, doc := range docs[i:end] {
			if !importCapableLang(doc.Language.String) {
				continue
			}

			imports, err := idx.extractImportsForDoc(doc, repoPath, docByPath)
			if err != nil {
				idx.logger.Warn("extract imports", "err", err, "path", doc.Path)
				continue
			}
			if err := storeGraphEdges(ctx, tx, repoID, doc.ID, imports); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("store import edges for %s: %w", doc.Path, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit graph edges batch: %w", err)
		}
	}

	refEdges, err := q.GetGraphEdges(ctx, db.GetGraphEdgesParams{RepoID: repoID, RepoID_2: repoID, RepoID_3: repoID})
	if err != nil {
		return fmt.Errorf("get ref edges: %w", err)
	}

	if err := batchInsertRefEdges(ctx, idx.db, repoID, refEdges); err != nil {
		return fmt.Errorf("store ref edges: %w", err)
	}

	idx.logger.Info("graph build complete", "repo_id", repoID, "docs", len(docs), "ref_edges", len(refEdges))
	return nil
}

func (idx *Indexer) extractImportsForDoc(doc db.Document, repoPath string, docByPath map[string]db.Document) ([]GraphEdge, error) {
	lang := doc.Language.String
	if lang == "" {
		return nil, nil
	}

	switch lang {
	case "Go", "JavaScript", "TypeScript", "Python", "Rust", "Java", "Kotlin", "C#", "PHP":
	default:
		return nil, nil
	}

	fullPath := filepath.Join(repoPath, doc.Path)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", doc.Path, err)
	}
	defer f.Close()

	header := make([]byte, 0, 4096)
	scanner := bufio.NewScanner(io.LimitReader(f, 8192))
	for scanner.Scan() {
		header = append(header, scanner.Text()...)
		header = append(header, '\n')
	}
	content := string(header)

	var importPaths []string
	switch lang {
	case "Go":
		importPaths = scanGoImports(content)
	case "JavaScript", "TypeScript":
		importPaths = scanJSImports(content)
	case "Python":
		importPaths = scanPyImports(content)
	case "Rust":
		importPaths = scanRustImports(content)
	case "Java", "Kotlin":
		importPaths = scanJavaLikeImports(content, lang)
	case "C#":
		importPaths = scanCSharpImports(content)
	case "PHP":
		importPaths = scanPHPImports(content)
	}

	// For package/namespace languages, derive the source root from the file's
	// own package/namespace declaration so imports resolve within the repo.
	srcRoot := sourceRootFromPackage(doc.Path, ownPackageDecl(content, lang), lang)

	var edges []GraphEdge
	for _, p := range importPaths {
		resolved := resolveImportPath(p, doc.Path, lang, repoPath, srcRoot, docByPath)
		if resolved == "" {
			continue
		}
		target, ok := docByPath[resolved]
		if !ok || target.ID == doc.ID {
			continue
		}
		edges = append(edges, GraphEdge{
			SourceDocID: doc.ID,
			TargetDocID: target.ID,
			Kind:        "imports",
			Weight:      1,
		})
	}

	return edges, nil
}

func resolveImportPath(importPath, relPath, lang, repoPath, srcRoot string, docByPath map[string]db.Document) string {
	switch lang {
	case "Java", "Kotlin":
		ext := ".java"
		if lang == "Kotlin" {
			ext = ".kt"
		}
		pkgPath := strings.ReplaceAll(importPath, ".", "/")
		candidates := make([]string, 0, 2)
		if srcRoot != "" {
			candidates = append(candidates, filepath.Join(srcRoot, pkgPath+ext))
		}
		candidates = append(candidates, pkgPath+ext)
		for _, c := range candidates {
			if _, ok := docByPath[c]; ok {
				return c
			}
		}
	case "C#":
		nsPath := strings.ReplaceAll(importPath, ".", "/")
		candidates := make([]string, 0, 2)
		if srcRoot != "" {
			candidates = append(candidates, filepath.Join(srcRoot, nsPath+".cs"))
		}
		candidates = append(candidates, nsPath+".cs")
		for _, c := range candidates {
			if _, ok := docByPath[c]; ok {
				return c
			}
		}
	case "PHP":
		imp := strings.ReplaceAll(importPath, "\\", "/")
		candidates := make([]string, 0, 4)
		if srcRoot != "" {
			candidates = append(candidates, filepath.Join(srcRoot, imp+".php"))
		}
		candidates = append(candidates, imp+".php")
		// Legacy PHP layouts often lowercase directory names while keeping class case.
		lowerRel := filepath.ToSlash(filepath.Join(strings.ToLower(filepath.Dir(imp)), filepath.Base(imp)))
		if srcRoot != "" {
			candidates = append(candidates, filepath.Join(srcRoot, lowerRel+".php"))
		}
		candidates = append(candidates, lowerRel+".php")
		for _, c := range candidates {
			if _, ok := docByPath[c]; ok {
				return c
			}
		}
	case "JavaScript", "TypeScript":
		if strings.HasPrefix(importPath, ".") {
			dir := filepath.Dir(relPath)
			resolved := filepath.Clean(filepath.Join(dir, importPath))
			candidates := []string{resolved, resolved + ".js", resolved + ".ts", resolved + ".tsx", resolved + ".jsx", resolved + "/index.js", resolved + "/index.ts", resolved + "/index.tsx", resolved + "/index.jsx"}
			for _, c := range candidates {
				if _, ok := docByPath[c]; ok {
					return c
				}
			}
		}
	case "Go":
		goModPath := findGoModulePath(repoPath)
		if goModPath == "" {
			return ""
		}
		if strings.HasPrefix(importPath, goModPath) {
			p := strings.TrimPrefix(importPath, goModPath)
			p = strings.TrimPrefix(p, "/")
			dir := filepath.Dir(filepath.Join(p, "."))
			for path := range docByPath {
				if filepath.Dir(path) == dir && strings.HasSuffix(path, ".go") {
					return path
				}
			}
		}
	case "Python":
		if strings.HasPrefix(importPath, ".") {
			dir := filepath.Dir(relPath)
			depth := 0
			p := importPath
			for strings.HasPrefix(p, ".") {
				p = strings.TrimPrefix(p, ".")
				depth++
			}
			for i := 0; i < depth && dir != "."; i++ {
				dir = filepath.Dir(dir)
			}
			resolved := filepath.Clean(filepath.Join(dir, strings.ReplaceAll(p, ".", "/")))
			candidates := []string{resolved, resolved + ".py", resolved + "/__init__.py"}
			for _, c := range candidates {
				if _, ok := docByPath[c]; ok {
					return c
				}
			}
		} else {
			pkgPath := strings.ReplaceAll(importPath, ".", "/")
			candidates := []string{pkgPath, pkgPath + ".py", pkgPath + "/__init__.py", strings.TrimSuffix(relPath, filepath.Base(relPath)) + pkgPath + ".py", strings.TrimSuffix(relPath, filepath.Base(relPath)) + pkgPath + "/__init__.py"}
			for _, c := range candidates {
				if _, ok := docByPath[c]; ok {
					return c
				}
			}
		}
	case "Rust":
		if strings.HasPrefix(importPath, "crate::") {
			p := strings.TrimPrefix(importPath, "crate::")
			resolved := strings.ReplaceAll(p, "::", "/")
			candidates := []string{resolved, resolved + ".rs", resolved + "/mod.rs"}
			for _, c := range candidates {
				if _, ok := docByPath[c]; ok {
					return c
				}
			}
		}
	}
	return ""
}

func scanGoImports(content string) []string {
	re := regexp.MustCompile(`(?m)^\s*import\s+(?:\w+\s+)?["]([^"]+)["]`)
	reBlock := regexp.MustCompile(`(?m)^\s*import\s+\(([^)]+)\)`)
	var paths []string
	if m := reBlock.FindStringSubmatch(content); m != nil {
		for _, im := range re.FindAllStringSubmatch(m[1], -1) {
			paths = append(paths, im[1])
		}
	}
	for _, im := range re.FindAllStringSubmatch(content, -1) {
		paths = append(paths, im[1])
	}
	return paths
}

func scanJSImports(content string) []string {
	re := regexp.MustCompile(`(?:import|export)\s+(?:(?:\{[^}]*\}|\*\s+as\s+\w+|\w+)\s+from\s+)?['"]([^'"]+)['"]`)
	reReq := regexp.MustCompile(`(?:require|import)\s*\(['"]([^'"]+)['"]\)`)
	var paths []string
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		p := m[1]
		if !seen[p] && (strings.HasPrefix(p, ".") || strings.HasPrefix(p, "/")) {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, m := range reReq.FindAllStringSubmatch(content, -1) {
		p := m[1]
		if !seen[p] && (strings.HasPrefix(p, ".") || strings.HasPrefix(p, "/")) {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths
}

func scanPyImports(content string) []string {
	reImport := regexp.MustCompile(`(?m)^\s*import\s+(\S+)`)
	reFrom := regexp.MustCompile(`(?m)^\s*from\s+(\S+)\s+import`)
	var paths []string
	seen := map[string]bool{}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, m := range reFrom.FindAllStringSubmatch(content, -1) {
		add(m[1])
	}
	for _, m := range reImport.FindAllStringSubmatch(content, -1) {
		p := strings.Split(m[1], ",")[0]
		p = strings.TrimSpace(p)
		add(p)
	}
	return paths
}

func scanRustImports(content string) []string {
	re := regexp.MustCompile(`(?m)^\s*use\s+([^;]+);`)
	var paths []string
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		p := m[1]
		if seen[p] {
			continue
		}
		seen[p] = true
		if strings.HasPrefix(p, "crate::") || strings.HasPrefix(p, "super::") {
			paths = append(paths, p)
		}
	}
	return paths
}

func scanJavaLikeImports(content, lang string) []string {
	var re *regexp.Regexp
	if lang == "Java" {
		re = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([\w.]+)\s*;`)
	} else {
		re = regexp.MustCompile(`(?m)^\s*import\s+([\w.]+)`)
	}
	var paths []string
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		p := m[1]
		// Skip wildcard imports (a.b.c.*) — they reference whole packages, not a single file.
		if strings.HasSuffix(p, ".*") || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

func scanCSharpImports(content string) []string {
	re := regexp.MustCompile(`(?m)^\s*using\s+([\w.]+)\s*;`)
	var paths []string
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		p := m[1]
		if seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

func scanPHPImports(content string) []string {
	re := regexp.MustCompile(`(?m)^\s*use\s+(?:function\s+|const\s+)?([\w\\]+)(?:\s+as\s+\w+)?\s*;`)
	var paths []string
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		p := m[1]
		if seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

// ownPackageDecl returns the package/namespace declared by the file itself for
// languages where imports are package-relative, or "" when not applicable.
func ownPackageDecl(content, lang string) string {
	var re *regexp.Regexp
	switch lang {
	case "Java", "Kotlin":
		re = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)`)
	case "C#":
		re = regexp.MustCompile(`(?m)^\s*namespace\s+([\w.]+)`)
	case "PHP":
		re = regexp.MustCompile(`(?m)^\s*namespace\s+([\w\\]+)\s*;`)
	default:
		return ""
	}
	if m := re.FindStringSubmatch(content); m != nil {
		return m[1]
	}
	return ""
}

// sourceRootFromPackage reconstructs the source root directory of a file from its
// package/namespace declaration. E.g. src/main/java/com/example/Main.java with
// "package com.example;" yields src/main/java. Returns "" when it cannot be derived.
func sourceRootFromPackage(relPath, pkg, lang string) string {
	if pkg == "" || relPath == "." {
		return ""
	}
	sep := "."
	if lang == "PHP" {
		sep = "\\"
	}
	dir := filepath.Dir(relPath)
	parts := strings.Split(pkg, sep)
	for i := len(parts) - 1; i >= 0; i-- {
		if filepath.Base(dir) != parts[i] {
			return ""
		}
		dir = filepath.Dir(dir)
	}
	return dir
}

type GraphEdge struct {
	SourceDocID int64
	TargetDocID int64
	Kind        string
	Weight      int
}

func storeGraphEdges(ctx context.Context, tx *sql.Tx, repoID int64, docID int64, edges []GraphEdge) error {
	if len(edges) == 0 {
		return nil
	}
	return batchExec(ctx, tx, "INSERT OR IGNORE INTO graph_edges (repo_id, source_doc_id, target_doc_id, kind, weight) VALUES ", 5,
		func(i int) []interface{} {
			return []interface{}{repoID, docID, edges[i].TargetDocID, edges[i].Kind, edges[i].Weight}
		},
		len(edges),
	)
}

func deleteGraphEdgesForRepo(ctx context.Context, conn *sql.DB, repoID int64) error {
	_, err := conn.ExecContext(ctx, `DELETE FROM graph_edges WHERE repo_id = ?`, repoID)
	return err
}

func (idx *Indexer) updateGraphForFile(ctx context.Context, repoID int64, repoPath, relPath string, docID int64) error {
	docs, err := idx.queries().ListDocuments(ctx, repoID)
	if err != nil {
		return err
	}
	docByPath := make(map[string]db.Document, len(docs))
	for _, d := range docs {
		docByPath[d.Path] = d
	}

	doc, ok := docByPath[relPath]
	if !ok {
		return nil
	}

	imports, err := idx.extractImportsForDoc(doc, repoPath, docByPath)
	if err != nil {
		return err
	}

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := idx.queries().WithTx(tx)

	if _, err := tx.ExecContext(ctx, `DELETE FROM graph_edges WHERE source_doc_id = ? OR target_doc_id = ?`, docID, docID); err != nil {
		return err
	}
	if err := storeGraphEdges(ctx, tx, repoID, docID, imports); err != nil {
		return err
	}

	refEdges, err := qtx.GetGraphEdges(ctx, db.GetGraphEdgesParams{RepoID: repoID, RepoID_2: repoID, RepoID_3: repoID})
	if err != nil {
		return err
	}
	var filtered []db.GetGraphEdgesRow
	for _, e := range refEdges {
		if e.SourceID == docID || e.TargetID == docID {
			filtered = append(filtered, e)
		}
	}
	if err := batchInsertRefEdgesTx(ctx, tx, repoID, filtered); err != nil {
		return err
	}

	return tx.Commit()
}

func batchInsertRefEdgesTx(ctx context.Context, tx *sql.Tx, repoID int64, edges []db.GetGraphEdgesRow) error {
	if len(edges) == 0 {
		return nil
	}
	return batchExec(ctx, tx, "INSERT OR IGNORE INTO graph_edges (repo_id, source_doc_id, target_doc_id, kind, weight) VALUES ", 5,
		func(i int) []interface{} {
			return []interface{}{repoID, edges[i].SourceID, edges[i].TargetID, "references", edges[i].Weight}
		},
		len(edges),
	)
}

type batchExecer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// importCapableLang reports whether the graph builder extracts explicit import
// edges for this language. All other languages still get reference
// co-occurrence edges (refs JOIN symbols).
func importCapableLang(lang string) bool {
	switch lang {
	case "Go", "JavaScript", "TypeScript", "Python", "Rust", "Java", "Kotlin", "C#", "PHP":
		return true
	}
	return false
}

func batchInsertRefEdges(ctx context.Context, conn batchExecer, repoID int64, edges []db.GetGraphEdgesRow) error {
	if len(edges) == 0 {
		return nil
	}
	return batchExec(ctx, conn, "INSERT OR IGNORE INTO graph_edges (repo_id, source_doc_id, target_doc_id, kind, weight) VALUES ", 5,
		func(i int) []interface{} {
			return []interface{}{repoID, edges[i].SourceID, edges[i].TargetID, "references", edges[i].Weight}
		},
		len(edges),
	)
}

func findGoModulePath(repoRoot string) string {
	content, err := ReadFileContent(filepath.Join(repoRoot, "go.mod"), 1024*1024)
	if err != nil || content == "" {
		return ""
	}
	re := regexp.MustCompile(`(?m)^module\s+(\S+)`)
	if m := re.FindStringSubmatch(content); m != nil {
		return m[1]
	}
	return ""
}
