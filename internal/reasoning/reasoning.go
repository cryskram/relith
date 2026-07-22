package reasoning

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog"

	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/search"
)

var identRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

var stopWords = map[string]bool{
	"the": true, "and": true, "or": true, "for": true, "with": true,
	"this": true, "that": true, "from": true, "into": true, "when": true,
	"what": true, "where": true, "how": true, "why": true, "show": true,
	"me": true, "everything": true, "leads": true, "behavior": true,
	"behaviour": true, "to": true, "of": true, "in": true, "on": true,
	"a": true, "an": true, "is": true, "it": true, "we": true, "you": true,
	"can": true, "make": true, "please": true, "context": true, "thingy": true,
}

type Engine struct {
	db      *sql.DB
	queries *db.Queries
	search  *search.Searcher
	log     zerolog.Logger
}

type TraceRequest struct {
	Query      string `json:"query"`
	RepoName   string `json:"repo_name,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type TraceBundle struct {
	Query         string          `json:"query"`
	RepoName      string          `json:"repo_name,omitempty"`
	FocusTerms    []string        `json:"focus_terms,omitempty"`
	SearchHits    []search.Result `json:"search_hits,omitempty"`
	Symbols       []SymbolHit     `json:"symbols,omitempty"`
	References    []RefHit        `json:"references,omitempty"`
	RelatedFiles  []FileHit       `json:"related_files,omitempty"`
	RelatedEdges  []EdgeHit       `json:"related_edges,omitempty"`
	GeneratedNote string          `json:"generated_note,omitempty"`
}

type SymbolHit struct {
	ID       int64  `json:"id"`
	DocID    int64  `json:"doc_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	RepoName string `json:"repo_name"`
	Line     int64  `json:"line"`
	Col      int64  `json:"col"`
	Reason   string `json:"reason"`
}

type RefHit struct {
	ID       int64  `json:"id"`
	DocID    int64  `json:"doc_id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	RepoName string `json:"repo_name"`
	Line     int64  `json:"line"`
	Col      int64  `json:"col"`
	Context  string `json:"context"`
	Reason   string `json:"reason"`
}

type FileHit struct {
	DocID    int64   `json:"doc_id"`
	RepoName string  `json:"repo_name"`
	Path     string  `json:"path"`
	Language string  `json:"language,omitempty"`
	Score    float64 `json:"score"`
	Reason   string  `json:"reason"`
	Snippet  string  `json:"snippet,omitempty"`
}

type EdgeHit struct {
	SourceDocID int64  `json:"source_doc_id"`
	TargetDocID int64  `json:"target_doc_id"`
	SourcePath  string `json:"source_path"`
	TargetPath  string `json:"target_path"`
	Weight      int64  `json:"weight"`
}

type docCandidate struct {
	DocID   int64
	Score   float64
	Reasons []string
}

func New(database *sql.DB, logger zerolog.Logger, searcher *search.Searcher) *Engine {
	return &Engine{db: database, queries: db.New(database), search: searcher, log: logger}
}

func (e *Engine) Trace(ctx context.Context, req TraceRequest) (TraceBundle, error) {
	q := strings.TrimSpace(req.Query)
	if q == "" {
		return TraceBundle{}, fmt.Errorf("query is required")
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 8
	}
	if maxResults > 20 {
		maxResults = 20
	}

	focusTerms := extractTerms(q)
	if len(focusTerms) == 0 {
		focusTerms = []string{q}
	}
	bundle := TraceBundle{Query: q, RepoName: req.RepoName, FocusTerms: focusTerms}

	repos, err := e.queries.ListRepos(ctx)
	if err != nil {
		return bundle, fmt.Errorf("list repos: %w", err)
	}
	repoByID := make(map[int64]db.Repository, len(repos))
	repoByName := make(map[string]db.Repository, len(repos))
	for _, r := range repos {
		repoByID[r.ID] = r
		repoByName[r.Name] = r
	}

	searchHits, err := e.collectSearchHits(ctx, q, focusTerms, maxResults, req.RepoName)
	if err != nil {
		return bundle, fmt.Errorf("search: %w", err)
	}
	bundle.SearchHits = searchHits

	candidates := map[int64]*docCandidate{}
	seedRepos := map[int64]bool{}
	markCandidate := func(docID int64, score float64, reason string) {
		cand := candidates[docID]
		if cand == nil {
			cand = &docCandidate{DocID: docID}
			candidates[docID] = cand
		}
		cand.Score += score
		cand.Reasons = appendUniqueReason(cand.Reasons, reason)
	}

	for _, h := range searchHits {
		markCandidate(h.DocumentID, 6.0+h.Score/10.0, "fts match")
	}

	var symbolHits []SymbolHit
	var refHits []RefHit
	for _, term := range focusTerms {
		var symRowsName []db.FindSymbolsByNameRow
		var symRowsRepo []db.FindSymbolsByRepoRow
		var refRowsName []db.FindRefsByNameRow
		var refRowsRepo []db.FindRefsByNameAndRepoRow
		if req.RepoName != "" {
			symRowsRepo, err = e.queries.FindSymbolsByRepo(ctx, db.FindSymbolsByRepoParams{
				Name:    req.RepoName,
				Column2: sql.NullString{String: term, Valid: true},
			})
			if err != nil {
				return bundle, fmt.Errorf("find symbols: %w", err)
			}
			refRowsRepo, err = e.queries.FindRefsByNameAndRepo(ctx, db.FindRefsByNameAndRepoParams{
				Name:    req.RepoName,
				Column2: sql.NullString{String: term, Valid: true},
			})
			if err != nil {
				return bundle, fmt.Errorf("find refs: %w", err)
			}
		} else {
			symRowsName, err = e.queries.FindSymbolsByName(ctx, sql.NullString{String: term, Valid: true})
			if err != nil {
				return bundle, fmt.Errorf("find symbols: %w", err)
			}
			refRowsName, err = e.queries.FindRefsByName(ctx, sql.NullString{String: term, Valid: true})
			if err != nil {
				return bundle, fmt.Errorf("find refs: %w", err)
			}
		}

		for _, row := range symRowsName {
			symbolHits = append(symbolHits, SymbolHit{
				ID: row.ID, DocID: row.DocID, Name: row.Name, Kind: row.Kind,
				Path: row.Path, RepoName: row.RepoName, Line: row.Line, Col: row.Col,
				Reason: fmt.Sprintf("symbol prefix %q", term),
			})
			markCandidate(row.DocID, 4.5, "symbol match")
			if repo, ok := repoByName[row.RepoName]; ok {
				seedRepos[repo.ID] = true
			}
		}

		for _, row := range refRowsName {
			refHits = append(refHits, RefHit{
				ID: row.ID, DocID: row.DocID, Name: row.Name, Path: row.Path,
				RepoName: row.RepoName, Line: row.Line, Col: row.Col, Context: row.Context,
				Reason: fmt.Sprintf("reference prefix %q", term),
			})
			markCandidate(row.DocID, 2.8, "reference match")
			if repo, ok := repoByName[row.RepoName]; ok {
				seedRepos[repo.ID] = true
			}
		}
		for _, row := range symRowsRepo {
			symbolHits = append(symbolHits, SymbolHit{
				ID: row.ID, DocID: row.DocID, Name: row.Name, Kind: row.Kind,
				Path: row.Path, RepoName: row.RepoName, Line: row.Line, Col: row.Col,
				Reason: fmt.Sprintf("symbol prefix %q", term),
			})
			markCandidate(row.DocID, 4.5, "symbol match")
			if repo, ok := repoByName[row.RepoName]; ok {
				seedRepos[repo.ID] = true
			}
		}
		for _, row := range refRowsRepo {
			refHits = append(refHits, RefHit{
				ID: row.ID, DocID: row.DocID, Name: row.Name, Path: row.Path,
				RepoName: row.RepoName, Line: row.Line, Col: row.Col, Context: row.Context,
				Reason: fmt.Sprintf("reference prefix %q", term),
			})
			markCandidate(row.DocID, 2.8, "reference match")
			if repo, ok := repoByName[row.RepoName]; ok {
				seedRepos[repo.ID] = true
			}
		}
	}

	if req.RepoName != "" {
		if repo, ok := repoByName[req.RepoName]; ok {
			seedRepos[repo.ID] = true
		}
	}
	for docID := range candidates {
		doc, err := e.queries.GetDocument(ctx, docID)
		if err != nil {
			continue
		}
		seedRepos[doc.RepoID] = true
	}

	seedDocs := map[int64]bool{}
	for docID := range candidates {
		seedDocs[docID] = true
	}

	relatedScores := map[int64]*docCandidate{}
	for docID, cand := range candidates {
		relatedScores[docID] = &docCandidate{DocID: docID, Score: cand.Score, Reasons: append([]string{}, cand.Reasons...)}
	}

	relatedEdges := map[string]EdgeHit{}
	for repoID := range seedRepos {
		edges, err := e.queries.GetGraphEdges(ctx, repoID)
		if err != nil {
			continue
		}
		for _, edge := range edges {
			if !seedDocs[edge.SourceID] && !seedDocs[edge.TargetID] {
				continue
			}
			if relatedScores[edge.SourceID] == nil {
				relatedScores[edge.SourceID] = &docCandidate{DocID: edge.SourceID}
			}
			if relatedScores[edge.TargetID] == nil {
				relatedScores[edge.TargetID] = &docCandidate{DocID: edge.TargetID}
			}
			relatedScores[edge.SourceID].Score += float64(edge.Weight) * 0.8
			relatedScores[edge.TargetID].Score += float64(edge.Weight) * 0.8
			relatedScores[edge.SourceID].Reasons = appendUniqueReason(relatedScores[edge.SourceID].Reasons, "graph neighbor")
			relatedScores[edge.TargetID].Reasons = appendUniqueReason(relatedScores[edge.TargetID].Reasons, "graph neighbor")
			key := fmt.Sprintf("%d:%d", edge.SourceID, edge.TargetID)
			relatedEdges[key] = EdgeHit{
				SourceDocID: edge.SourceID,
				TargetDocID: edge.TargetID,
				SourcePath:  edge.SourcePath,
				TargetPath:  edge.TargetPath,
				Weight:      edge.Weight,
			}
		}
	}

	type scoredDoc struct {
		docID int64
		score float64
	}
	var docOrder []scoredDoc
	for docID, cand := range relatedScores {
		docOrder = append(docOrder, scoredDoc{docID: docID, score: cand.Score})
	}
	sort.Slice(docOrder, func(i, j int) bool {
		if docOrder[i].score == docOrder[j].score {
			return docOrder[i].docID < docOrder[j].docID
		}
		return docOrder[i].score > docOrder[j].score
	})
	if len(docOrder) > maxResults {
		docOrder = docOrder[:maxResults]
	}

	for _, item := range docOrder {
		doc, err := e.queries.GetDocument(ctx, item.docID)
		if err != nil {
			continue
		}
		repo := repoByID[doc.RepoID]
		chunks, err := e.queries.ListChunks(ctx, doc.ID)
		if err != nil {
			continue
		}
		cand := relatedScores[doc.ID]
		bundle.RelatedFiles = append(bundle.RelatedFiles, FileHit{
			DocID: doc.ID, RepoName: repo.Name, Path: doc.Path, Language: doc.Language.String,
			Score: round2(item.score), Reason: strings.Join(cand.Reasons, ", "),
			Snippet: pickSnippet(chunks, bundle.FocusTerms),
		})
	}

	for edge := range relatedEdges {
		bundle.RelatedEdges = append(bundle.RelatedEdges, relatedEdges[edge])
	}
	sort.Slice(bundle.RelatedEdges, func(i, j int) bool {
		if bundle.RelatedEdges[i].Weight == bundle.RelatedEdges[j].Weight {
			return bundle.RelatedEdges[i].SourceDocID < bundle.RelatedEdges[j].SourceDocID
		}
		return bundle.RelatedEdges[i].Weight > bundle.RelatedEdges[j].Weight
	})

	if len(symbolHits) > maxResults {
		symbolHits = symbolHits[:maxResults]
	}
	if len(refHits) > maxResults {
		refHits = refHits[:maxResults]
	}
	bundle.Symbols = dedupeSymbols(symbolHits)
	bundle.References = dedupeRefs(refHits)
	bundle.GeneratedNote = "relith hybrid retrieval: FTS seeds + AST symbols + references + graph rerank"
	return bundle, nil
}

func (e *Engine) collectSearchHits(ctx context.Context, query string, terms []string, limit int, repoName string) ([]search.Result, error) {
	seen := map[int64]search.Result{}
	add := func(result search.Result, reasonBoost float64) {
		if repoName != "" && result.RepoName != repoName {
			return
		}
		existing, ok := seen[result.DocumentID]
		if !ok || result.Score+reasonBoost > existing.Score {
			result.Score += reasonBoost
			seen[result.DocumentID] = result
		}
	}

	queries := []struct {
		q     string
		boost float64
	}{
		{q: query, boost: 0},
	}
	for _, term := range terms {
		if term == query {
			continue
		}
		queries = append(queries, struct {
			q     string
			boost float64
		}{q: term, boost: 0.8})
	}

	for _, item := range queries {
		hits, err := e.search.Search(ctx, item.q, limit)
		if err != nil {
			return nil, err
		}
		for _, hit := range hits {
			add(hit, item.boost)
		}
	}

	var out []search.Result
	for _, h := range seen {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].DocumentID < out[j].DocumentID
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (b TraceBundle) Text() string {
	data, _ := json.MarshalIndent(b, "", "  ")
	return string(data)
}

func extractTerms(q string) []string {
	matches := identRe.FindAllString(q, -1)
	seen := map[string]bool{}
	var terms []string
	for _, m := range matches {
		lower := strings.ToLower(m)
		if stopWords[lower] || seen[lower] {
			continue
		}
		seen[lower] = true
		terms = append(terms, m)
	}
	sort.Slice(terms, func(i, j int) bool {
		if len(terms[i]) == len(terms[j]) {
			return terms[i] < terms[j]
		}
		return len(terms[i]) > len(terms[j])
	})
	if len(terms) > 4 {
		terms = terms[:4]
	}
	return terms
}

func appendUniqueReason(existing []string, reasons ...string) []string {
	seen := map[string]bool{}
	for _, r := range existing {
		seen[r] = true
	}
	for _, r := range reasons {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		existing = append(existing, r)
	}
	return existing
}

func dedupeSymbols(in []SymbolHit) []SymbolHit {
	seen := map[string]bool{}
	out := make([]SymbolHit, 0, len(in))
	for _, v := range in {
		key := fmt.Sprintf("%d:%s:%d", v.DocID, v.Name, v.Line)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}

func dedupeRefs(in []RefHit) []RefHit {
	seen := map[string]bool{}
	out := make([]RefHit, 0, len(in))
	for _, v := range in {
		key := fmt.Sprintf("%d:%s:%d:%d", v.DocID, v.Name, v.Line, v.Col)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}

func pickSnippet(chunks []db.Chunk, terms []string) string {
	if len(chunks) == 0 {
		return ""
	}
	for _, term := range terms {
		for _, chunk := range chunks {
			if strings.Contains(strings.ToLower(chunk.Content), strings.ToLower(term)) {
				return truncate(chunk.Content, 800)
			}
		}
	}
	return truncate(chunks[0].Content, 800)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func round2(v float64) float64 {
	if v < 0 {
		return -round2(-v)
	}
	return float64(int(v*100+0.5)) / 100
}
