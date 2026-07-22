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
	idx.logger.Info().Int64("repo_id", repoID).Msg("building graph edges")

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
			if err := deleteGraphEdgesForDoc(ctx, tx, doc.ID); err != nil {
				tx.Rollback()
				return fmt.Errorf("delete edges for doc %d: %w", doc.ID, err)
			}

			imports, err := idx.extractImportsForDoc(doc, repoPath, docByPath)
			if err != nil {
				idx.logger.Warn().Err(err).Str("path", doc.Path).Msg("extract imports")
				continue
			}
			if err := storeGraphEdges(ctx, tx, repoID, doc.ID, imports); err != nil {
				tx.Rollback()
				return fmt.Errorf("store import edges for %s: %w", doc.Path, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit graph edges batch: %w", err)
		}
	}

	refEdges, err := q.GetGraphEdges(ctx, repoID)
	if err != nil {
		return fmt.Errorf("get ref edges: %w", err)
	}

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for ref edges: %w", err)
	}
	for _, e := range refEdges {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO graph_edges (repo_id, source_doc_id, target_doc_id, kind, weight) VALUES (?, ?, ?, 'references', ?)`, repoID, e.SourceID, e.TargetID, e.Weight); err != nil {
			tx.Rollback()
			return fmt.Errorf("store ref edge: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ref edges: %w", err)
	}

	idx.logger.Info().Int64("repo_id", repoID).Int("docs", len(docs)).Int("ref_edges", len(refEdges)).Msg("graph build complete")
	return nil
}

func (idx *Indexer) extractImportsForDoc(doc db.Document, repoPath string, docByPath map[string]db.Document) ([]GraphEdge, error) {
	lang := doc.Language.String
	if lang == "" {
		return nil, nil
	}

	switch lang {
	case "Go", "JavaScript", "TypeScript", "Python", "Rust":
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
	}

	var edges []GraphEdge
	for _, p := range importPaths {
		resolved := resolveImportPath(p, doc.Path, lang, repoPath, docByPath)
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

func resolveImportPath(importPath, relPath, lang, repoPath string, docByPath map[string]db.Document) string {
	switch lang {
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
	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO graph_edges (repo_id, source_doc_id, target_doc_id, kind, weight) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()
	for _, e := range edges {
		if _, err := stmt.ExecContext(ctx, repoID, docID, e.TargetDocID, e.Kind, e.Weight); err != nil {
			return fmt.Errorf("insert: %w", err)
		}
	}
	return nil
}

func deleteGraphEdgesForDoc(ctx context.Context, tx *sql.Tx, docID int64) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM graph_edges WHERE source_doc_id = ? OR target_doc_id = ?`, docID, docID)
	return err
}

func deleteGraphEdgesForRepo(ctx context.Context, db *sql.DB, repoID int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM graph_edges WHERE repo_id = ?`, repoID)
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
	defer tx.Rollback()

	if err := deleteGraphEdgesForDoc(ctx, tx, docID); err != nil {
		return err
	}
	if err := storeGraphEdges(ctx, tx, repoID, docID, imports); err != nil {
		return err
	}

	refEdges, err := idx.queries().GetGraphEdges(ctx, repoID)
	if err != nil {
		return err
	}
	for _, e := range refEdges {
		if e.SourceID == docID || e.TargetID == docID {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO graph_edges (repo_id, source_doc_id, target_doc_id, kind, weight) VALUES (?, ?, ?, 'references', ?)`, repoID, e.SourceID, e.TargetID, e.Weight); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
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
