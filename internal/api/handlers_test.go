package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/testutil"
)

func newTestServer(t *testing.T) (*httptest.Server, *testutil.DB) {
	t.Helper()

	fdb := testutil.NewDB(t)
	srv := New(fdb.SQL, testutil.DiscardLogger(), fdb.Config())
	ts := httptest.NewServer(srv.http.Handler)
	t.Cleanup(ts.Close)
	return ts, fdb
}

func get(t *testing.T, ts *httptest.Server, path string) (int, string) {
	t.Helper()

	resp, err := ts.Client().Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(body)
}

func TestHealth(t *testing.T) {
	ts, _ := newTestServer(t)

	status, body := get(t, ts, "/v1/health")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("body = %s, want status ok", body)
	}
}

func TestListReposEmpty(t *testing.T) {
	ts, _ := newTestServer(t)

	status, body := get(t, ts, "/v1/repos")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if strings.TrimSpace(body) != "[]" && strings.TrimSpace(body) != "null" {
		t.Errorf("expected empty repos, got %s", body)
	}
}

func TestCreateAndGetRepo(t *testing.T) {
	ts, _ := newTestServer(t)

	body := `{"path":"/tmp/repo","name":"my-repo","remote_url":"https://example.com/x.git"}`
	resp, err := ts.Client().Post(ts.URL+"/v1/repos", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /v1/repos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var repo db.Repository
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		t.Fatalf("decode repo: %v", err)
	}
	if repo.Name != "my-repo" {
		t.Errorf("repo name = %q, want my-repo", repo.Name)
	}

	status, bodyStr := get(t, ts, "/v1/repos/"+strconv.FormatInt(repo.ID, 10))
	if status != http.StatusOK {
		t.Fatalf("get repo status = %d, want %d", status, http.StatusOK)
	}
	if !strings.Contains(bodyStr, `"name":"my-repo"`) {
		t.Errorf("get repo body = %s", bodyStr)
	}
}

func TestCreateRepoValidation(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, err := ts.Client().Post(ts.URL+"/v1/repos", "application/json", strings.NewReader(`{"path":"/x"}`))
	if err != nil {
		t.Fatalf("POST /v1/repos: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing name status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateRepoDuplicate(t *testing.T) {
	ts, _ := newTestServer(t)

	for i := 0; i < 2; i++ {
		body := `{"path":"/tmp/repo","name":"my-repo"}`
		resp, err := ts.Client().Post(ts.URL+"/v1/repos", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST /v1/repos: %v", err)
		}
		resp.Body.Close()
		if i == 0 && resp.StatusCode != http.StatusCreated {
			t.Fatalf("first create status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}
		if i == 1 && resp.StatusCode != http.StatusConflict {
			t.Errorf("duplicate create status = %d, want %d", resp.StatusCode, http.StatusConflict)
		}
	}
}

func TestSearch(t *testing.T) {
	ts, fdb := newTestServer(t)

	repo := fdb.CreateRepo("/tmp/repo", "my-repo")
	doc := fdb.CreateDocument(repo.ID, "main.go", "Go")
	fdb.CreateChunk(doc.ID, 0, "package main\nfunc main() { fmt.Println(\"hello world\") }")

	status, body := get(t, ts, "/v1/search?q=main")
	if status != http.StatusOK {
		t.Fatalf("search status = %d, want %d (body: %s)", status, http.StatusOK, body)
	}
	if !strings.Contains(body, "main.go") {
		t.Errorf("search results should include main.go, got %s", body)
	}
}

func TestSearchMissingQuery(t *testing.T) {
	ts, _ := newTestServer(t)

	status, _ := get(t, ts, "/v1/search")
	if status != http.StatusBadRequest {
		t.Errorf("missing q status = %d, want %d", status, http.StatusBadRequest)
	}
}

func TestGetRepoNotFound(t *testing.T) {
	ts, _ := newTestServer(t)

	status, _ := get(t, ts, "/v1/repos/999")
	if status != http.StatusNotFound {
		t.Errorf("missing repo status = %d, want %d", status, http.StatusNotFound)
	}
}

func TestGetRepoInvalidID(t *testing.T) {
	ts, _ := newTestServer(t)

	status, _ := get(t, ts, "/v1/repos/abc")
	if status != http.StatusBadRequest {
		t.Errorf("invalid id status = %d, want %d", status, http.StatusBadRequest)
	}
}

func TestGraph(t *testing.T) {
	ts, fdb := newTestServer(t)

	repo := fdb.CreateRepo("/tmp/repo", "my-repo")
	docA := fdb.CreateDocument(repo.ID, "a.go", "Go")
	docB := fdb.CreateDocument(repo.ID, "b.go", "Go")
	fdb.CreateGraphEdge(repo.ID, docA.ID, docB.ID, "references", 2)

	status, body := get(t, ts, "/v1/graph")
	if status != http.StatusOK {
		t.Fatalf("graph status = %d, want %d (body: %s)", status, http.StatusOK, body)
	}
	if !strings.Contains(body, `"nodes"`) || !strings.Contains(body, `"edges"`) {
		t.Errorf("graph body should have nodes and edges, got %s", body)
	}
	if !strings.Contains(body, "a.go") || !strings.Contains(body, "b.go") {
		t.Errorf("graph should include both docs, got %s", body)
	}
}

func TestGraphRepoNotFound(t *testing.T) {
	ts, fdb := newTestServer(t)

	fdb.CreateRepo("/tmp/repo", "my-repo")

	status, _ := get(t, ts, "/v1/graph?repo=nope")
	if status != http.StatusNotFound {
		t.Errorf("missing graph repo status = %d, want %d", status, http.StatusNotFound)
	}
}

func TestStats(t *testing.T) {
	ts, fdb := newTestServer(t)

	repo := fdb.CreateRepo("/tmp/repo", "my-repo")
	doc := fdb.CreateDocument(repo.ID, "main.go", "Go")
	fdb.CreateChunk(doc.ID, 0, "package main\nfunc main() {}")

	status, body := get(t, ts, "/v1/stats")
	if status != http.StatusOK {
		t.Fatalf("stats status = %d, want %d (body: %s)", status, http.StatusOK, body)
	}
	if !strings.Contains(body, `"repo_count":1`) {
		t.Errorf("stats should report 1 repo, got %s", body)
	}
}

func TestContentMissingRepo(t *testing.T) {
	ts, _ := newTestServer(t)

	status, _ := get(t, ts, "/v1/content?repo=nope&path=main.go")
	if status != http.StatusNotFound {
		t.Errorf("content missing repo status = %d, want %d", status, http.StatusNotFound)
	}
}

func TestContentMissingParams(t *testing.T) {
	ts, _ := newTestServer(t)

	status, _ := get(t, ts, "/v1/content")
	if status != http.StatusBadRequest {
		t.Errorf("content missing params status = %d, want %d", status, http.StatusBadRequest)
	}
}

func TestReason(t *testing.T) {
	ts, fdb := newTestServer(t)

	repo := fdb.CreateRepo("/tmp/repo", "my-repo")
	doc := fdb.CreateDocument(repo.ID, "main.go", "Go")
	fdb.CreateChunk(doc.ID, 0, "package main\nfunc NewParser() *Parser { return &Parser{} }")
	fdb.CreateSymbol(doc.ID, "NewParser", "function", 2, 6)

	status, body := get(t, ts, "/v1/reason?q=NewParser")
	if status != http.StatusOK {
		t.Fatalf("reason status = %d, want %d (body: %s)", status, http.StatusOK, body)
	}
	if !strings.Contains(body, "NewParser") {
		t.Errorf("reason body should mention NewParser, got %s", body)
	}
}

func TestDeleteRepo(t *testing.T) {
	ts, fdb := newTestServer(t)

	repo := fdb.CreateRepo("/tmp/repo", "my-repo")

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/v1/repos/"+strconv.FormatInt(repo.ID, 10), http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}
