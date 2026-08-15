package reasoning

import (
	"context"
	"reflect"
	"testing"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/search"
	"github.com/cryskram/relith/internal/testutil"
)

func newEngine(t *testing.T, fdb *testutil.DB) *Engine {
	t.Helper()

	cfg := config.SearchConfig{MaxResults: 100, PathBoosting: true}
	searcher := search.New(fdb.SQL, testutil.DiscardLogger(), cfg)
	return New(fdb.SQL, testutil.DiscardLogger(), searcher)
}

func TestTraceEmptyQuery(t *testing.T) {
	fdb := testutil.NewDB(t)
	e := newEngine(t, fdb)

	_, err := e.Trace(context.Background(), TraceRequest{Query: "  "})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestTraceEmptyDB(t *testing.T) {
	fdb := testutil.NewDB(t)
	e := newEngine(t, fdb)

	bundle, err := e.Trace(context.Background(), TraceRequest{Query: "nothing here"})
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	if len(bundle.SearchHits) != 0 {
		t.Errorf("expected no search hits, got %d", len(bundle.SearchHits))
	}
	if len(bundle.RelatedFiles) != 0 {
		t.Errorf("expected no related files, got %d", len(bundle.RelatedFiles))
	}
}

func TestTraceSymbolAndReferenceHits(t *testing.T) {
	fdb := testutil.NewDB(t)

	repo := fdb.CreateRepo("/tmp/repo", "my-repo")
	docA := fdb.CreateDocument(repo.ID, "parser.go", "Go")
	docB := fdb.CreateDocument(repo.ID, "main.go", "Go")
	fdb.CreateChunk(docA.ID, 0, "package main\nfunc NewParser() *Parser { return &Parser{} }")
	fdb.CreateChunk(docB.ID, 0, "package main\nfunc main() { p := NewParser() }")
	fdb.CreateSymbol(docA.ID, "NewParser", "function", 2, 6)
	fdb.CreateRef(docB.ID, "NewParser", 2, 12, "p := NewParser()")
	fdb.CreateGraphEdge(repo.ID, docA.ID, docB.ID, "references", 1)

	e := newEngine(t, fdb)
	bundle, err := e.Trace(context.Background(), TraceRequest{Query: "NewParser", MaxResults: 8})
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}

	if len(bundle.SearchHits) == 0 {
		t.Error("expected FTS search hits")
	}
	if len(bundle.Symbols) == 0 {
		t.Error("expected symbol hits")
	} else {
		if bundle.Symbols[0].Name != "NewParser" {
			t.Errorf("symbol name = %s, want NewParser", bundle.Symbols[0].Name)
		}
		if bundle.Symbols[0].Path != "parser.go" {
			t.Errorf("symbol path = %s, want parser.go", bundle.Symbols[0].Path)
		}
	}
	if len(bundle.References) == 0 {
		t.Error("expected reference hits")
	}
	if len(bundle.RelatedFiles) == 0 {
		t.Error("expected related files")
	}
	if bundle.GeneratedNote == "" {
		t.Error("expected a generated note")
	}
}

func TestTraceRepoFilter(t *testing.T) {
	fdb := testutil.NewDB(t)

	repoA := fdb.CreateRepo("/tmp/a", "repo-a")
	repoB := fdb.CreateRepo("/tmp/b", "repo-b")
	docA := fdb.CreateDocument(repoA.ID, "parser.go", "Go")
	docB := fdb.CreateDocument(repoB.ID, "other.go", "Go")
	fdb.CreateChunk(docA.ID, 0, "func NewParser() *Parser { return &Parser{} }")
	fdb.CreateChunk(docB.ID, 0, "func NewParser() *Parser { return &Parser{} }")
	fdb.CreateSymbol(docA.ID, "NewParser", "function", 1, 6)
	fdb.CreateSymbol(docB.ID, "NewParser", "function", 1, 6)

	e := newEngine(t, fdb)
	bundle, err := e.Trace(context.Background(), TraceRequest{Query: "NewParser", RepoName: "repo-a", MaxResults: 8})
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}

	for _, hit := range bundle.SearchHits {
		if hit.RepoName != "repo-a" {
			t.Errorf("search hit repo = %s, want repo-a", hit.RepoName)
		}
	}
	for _, sym := range bundle.Symbols {
		if sym.RepoName != "repo-a" {
			t.Errorf("symbol repo = %s, want repo-a", sym.RepoName)
		}
	}
}

func TestExtractTerms(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{input: "NewParser", want: []string{"NewParser"}},
		{input: "trace the NewParser behavior", want: []string{"NewParser", "trace"}},
		{input: "show me the context", want: nil},
		{input: "index build speed", want: []string{"build", "index", "speed"}},
		{input: "   ", want: nil},
	}

	for _, tt := range tests {
		got := extractTerms(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("extractTerms(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExtractTermsDedupesAndLimits(t *testing.T) {
	got := extractTerms("foo bar baz qux foo")
	if len(got) > 4 {
		t.Errorf("expected at most 4 terms, got %d", len(got))
	}
	count := 0
	for _, term := range got {
		if term == "foo" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected foo deduped, got %d occurrences", count)
	}
}

func TestAppendUniqueReason(t *testing.T) {
	got := appendUniqueReason(nil, "a", "b", "a", "")
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("appendUniqueReason = %v, want %v", got, want)
	}
}

func TestDedupeSymbols(t *testing.T) {
	in := []SymbolHit{
		{DocID: 1, Name: "Foo", Line: 3},
		{DocID: 1, Name: "Foo", Line: 3},
		{DocID: 1, Name: "Foo", Line: 4},
	}
	got := dedupeSymbols(in)
	if len(got) != 2 {
		t.Errorf("expected 2 unique symbols, got %d", len(got))
	}
}

func TestRound2(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{input: 1.234, want: 1.23},
		{input: 1.235, want: 1.24},
		{input: -1.234, want: -1.23},
		{input: 0, want: 0},
	}
	for _, tt := range tests {
		if got := round2(tt.input); got != tt.want {
			t.Errorf("round2(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
