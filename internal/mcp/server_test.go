package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cryskram/relith/internal/testutil"
)

func runServer(t *testing.T, fdb *testutil.DB, requests string) []JSONRPCResponse {
	t.Helper()

	s := NewServer(fdb.SQL, testutil.DiscardLogger())
	var out bytes.Buffer
	s.reader = strings.NewReader(requests)
	s.writer = &out

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	responses := make([]JSONRPCResponse, 0, len(lines))
	for _, line := range lines {
		var resp JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("invalid response line %q: %v", line, err)
		}
		responses = append(responses, resp)
	}
	return responses
}

func byID(t *testing.T, responses []JSONRPCResponse, id string) JSONRPCResponse {
	t.Helper()

	for _, resp := range responses {
		if string(resp.ID) == id {
			return resp
		}
	}
	t.Fatalf("no response with id %s in %d responses", id, len(responses))
	return JSONRPCResponse{}
}

func TestServerInitialize(t *testing.T) {
	fdb := testutil.NewDB(t)

	requests := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
`
	responses := runServer(t, fdb, requests)
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected object result, got %T", resp.Result)
	}
	if result["protocolVersion"] != ProtocolVersion {
		t.Errorf("protocolVersion = %v, want %s", result["protocolVersion"], ProtocolVersion)
	}
}

func TestServerToolsList(t *testing.T) {
	fdb := testutil.NewDB(t)

	requests := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}
`
	responses := runServer(t, fdb, requests)

	resp := byID(t, responses, "1")
	if resp.Error != nil {
		t.Fatalf("tools/list error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected object result, got %T", resp.Result)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got %T", result["tools"])
	}
	if len(tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	found := false
	for _, tool := range tools {
		if m, ok := tool.(map[string]any); ok && m["name"] == "search_code" {
			found = true
		}
	}
	if !found {
		t.Error("tools/list should include search_code")
	}
}

func TestServerCallSearchCode(t *testing.T) {
	fdb := testutil.NewDB(t)

	repo := fdb.CreateRepo("/tmp/repo", "my-repo")
	doc := fdb.CreateDocument(repo.ID, "main.go", "Go")
	fdb.CreateChunk(doc.ID, 0, "package main\nfunc main() { fmt.Println(\"hello world\") }")

	requests := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_code","arguments":{"query":"main"}}}
`
	responses := runServer(t, fdb, requests)

	resp := byID(t, responses, "1")
	if resp.Error != nil {
		t.Fatalf("search_code error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected object result, got %T", resp.Result)
	}
	if isErr, _ := result["isError"].(bool); isErr {
		t.Fatalf("search_code returned isError: %+v", result)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "main.go") {
		t.Errorf("search_code result should include main.go, got: %s", text)
	}
}

func TestServerCallSearchCodeNoRepos(t *testing.T) {
	fdb := testutil.NewDB(t)

	requests := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_code","arguments":{"query":"main"}}}
`
	responses := runServer(t, fdb, requests)

	resp := byID(t, responses, "1")
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected object result, got %T", resp.Result)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "No results found") {
		t.Errorf("expected no-repos hint, got: %s", text)
	}
}

func TestServerCallListRepositories(t *testing.T) {
	fdb := testutil.NewDB(t)

	fdb.CreateRepo("/tmp/repo", "my-repo")

	requests := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_repositories","arguments":{}}}
`
	responses := runServer(t, fdb, requests)

	resp := byID(t, responses, "1")
	if resp.Error != nil {
		t.Fatalf("list_repositories error: %+v", resp.Error)
	}
	text := resultText(t, resp.Result.(map[string]any))
	if !strings.Contains(text, "my-repo") {
		t.Errorf("list_repositories should include my-repo, got: %s", text)
	}
}

func TestServerCallUnknownTool(t *testing.T) {
	fdb := testutil.NewDB(t)

	requests := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}
`
	responses := runServer(t, fdb, requests)

	resp := byID(t, responses, "1")
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "nope") {
		t.Errorf("error message should name the tool, got %q", resp.Error.Message)
	}
}

func TestServerPing(t *testing.T) {
	fdb := testutil.NewDB(t)

	requests := `{"jsonrpc":"2.0","id":1,"method":"ping"}
`
	responses := runServer(t, fdb, requests)

	resp := byID(t, responses, "1")
	if resp.Error != nil {
		t.Fatalf("ping error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected object result, got %T", resp.Result)
	}
	if len(result) != 0 {
		t.Errorf("ping result should be empty, got %v", result)
	}
}

func TestServerMethodNotFound(t *testing.T) {
	fdb := testutil.NewDB(t)

	requests := `{"jsonrpc":"2.0","id":1,"method":"bogus"}
`
	responses := runServer(t, fdb, requests)

	resp := byID(t, responses, "1")
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
}

func TestServerFindSymbol(t *testing.T) {
	fdb := testutil.NewDB(t)

	repo := fdb.CreateRepo("/tmp/repo", "my-repo")
	doc := fdb.CreateDocument(repo.ID, "main.go", "Go")
	fdb.CreateSymbol(doc.ID, "NewParser", "function", 2, 6)

	requests := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"find_symbol","arguments":{"name":"NewPar"}}}
`
	responses := runServer(t, fdb, requests)

	resp := byID(t, responses, "1")
	if resp.Error != nil {
		t.Fatalf("find_symbol error: %+v", resp.Error)
	}
	text := resultText(t, resp.Result.(map[string]any))
	if !strings.Contains(text, "NewParser") {
		t.Errorf("find_symbol should include NewParser, got: %s", text)
	}
}

func resultText(t *testing.T, result map[string]any) string {
	t.Helper()

	content, ok := result["content"].([]any)
	if !ok {
		t.Fatalf("expected content array, got %T", result["content"])
	}
	var sb strings.Builder
	for _, item := range content {
		if m, ok := item.(map[string]any); ok {
			sb.WriteString(m["text"].(string))
		}
	}
	return sb.String()
}
