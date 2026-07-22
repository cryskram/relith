package mcp

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/cryskram/relith/internal/cli"
	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/reasoning"
	"github.com/cryskram/relith/internal/search"
)

type ToolHandler func(ctx context.Context, params map[string]any) CallToolResult

type ResourceHandler func(ctx context.Context, uri string) []ResourceContents

type Server struct {
	logger      zerolog.Logger
	db          *sql.DB
	queries     *db.Queries
	searcher    *search.Searcher
	reasoner    *reasoning.Engine
	reader      io.Reader
	writer      io.Writer
	tools       map[string]ToolHandler
	resources   map[string]ResourceHandler
	initialized bool
}

func NewServer(database *sql.DB, log zerolog.Logger) *Server {
	cfg := &config.Config{
		Indexer: config.IndexerConfig{
			Concurrency: 4,
			MaxFileSize: 10 * 1024 * 1024,
		},
		Search: config.SearchConfig{
			MaxResults:   100,
			PathBoosting: true,
		},
	}

	s := &Server{
		logger:    log,
		db:        database,
		queries:   db.New(database),
		searcher:  search.New(database, log, cfg.Search),
		reader:    os.Stdin,
		writer:    os.Stdout,
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
	}
	s.reasoner = reasoning.New(database, log, s.searcher)

	s.registerTools()
	s.registerResources()
	return s
}

func (s *Server) registerTools() {
	s.tools["search_code"] = s.handleSearchCode
	s.tools["get_file_content"] = s.handleGetFileContent
	s.tools["list_repositories"] = s.handleListRepos
	s.tools["get_repo_summary"] = s.handleGetRepoSummary
	s.tools["find_symbol"] = s.handleFindSymbol
	s.tools["find_references"] = s.handleFindReferences
	s.tools["trace_context"] = s.handleTraceContext
	s.tools["get_file_outline"] = s.handleGetFileOutline
	s.tools["find_callers"] = s.handleFindCallers
	s.tools["get_related_files"] = s.handleGetRelatedFiles
	s.tools["list_hub_files"] = s.handleListHubFiles
}

func (s *Server) registerResources() {
	s.resources["repo://"] = s.handleResourceRepo
}

func (s *Server) Run(ctx context.Context) error {
	s.logger.Info().Msg("MCP server starting (stdio)")
	scanner := bufio.NewScanner(s.reader)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.logger.Error().Err(err).Str("raw", line).Msg("parse error")
			continue
		}

		s.dispatch(ctx, req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdin: %w", err)
	}
	return nil
}

func (s *Server) dispatch(ctx context.Context, req JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(ctx, req)
	case "notifications/initialized":
		s.initialized = true
	case "notifications/cancelled":
	case "tools/list":
		s.handleToolsList(ctx, req)
	case "tools/call":
		s.handleToolsCall(ctx, req)
	case "resources/list":
		s.handleResourcesList(ctx, req)
	case "resources/read":
		s.handleResourcesRead(ctx, req)
	case "ping":
		s.writeJSON(req.ID, JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}})
	default:
		if req.ID != nil {
			s.writeJSON(req.ID, JSONRPCResponse{
				JSONRPC: "2.0", ID: req.ID,
				Error: &JSONRPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
			})
		}
	}
}

func (s *Server) handleInitialize(ctx context.Context, req JSONRPCRequest) {
	var initReq InitializeRequest
	if err := json.Unmarshal(req.Params, &initReq); err != nil {
		s.writeJSON(req.ID, JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &JSONRPCError{Code: -32602, Message: "invalid initialize params"},
		})
		return
	}

	s.logger.Info().Str("client", initReq.ClientInfo.Name).Str("version", initReq.ClientInfo.Version).Msg("client initialized")

	s.writeJSON(req.ID, JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: InitializeResult{
			ProtocolVersion: ProtocolVersion,
			Capabilities: ServerCapabilities{
				Tools:     &ToolCapabilities{ListChanged: false},
				Resources: &ResourceCapabilities{Subscribe: false, ListChanged: false},
			},
			ServerInfo: Implementation{
				Name:    "relith",
				Version: cli.Version,
			},
		},
	})
}

func (s *Server) handleToolsList(ctx context.Context, req JSONRPCRequest) {
	tools := []Tool{
		{
			Name:        "search_code",
			Description: "Full-text search across all indexed repositories. Returns matching code snippets with file paths, repo names, and relevance scores.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "Search query (supports FTS5 syntax: AND, OR, NOT, phrases)"},
					"repo_name": {"type": "string", "description": "Optional: filter by repository name"},
					"language": {"type": "string", "description": "Optional: filter by programming language (e.g. Go, Python, JavaScript)"},
					"max_results": {"type": "integer", "description": "Maximum results to return (default 20)", "default": 20}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "get_file_content",
			Description: "Retrieve the full content of a file from an indexed repository by repo name and file path.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"repo_name": {"type": "string", "description": "Repository name"},
					"path": {"type": "string", "description": "File path relative to repository root"}
				},
				"required": ["repo_name", "path"]
			}`),
		},
		{
			Name:        "list_repositories",
			Description: "List all tracked repositories with their status, file count, and last indexed time.",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		{
			Name:        "get_repo_summary",
			Description: "Get a summary of a repository including language breakdown, file count, chunk count, and indexing status.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"repo_name": {"type": "string", "description": "Repository name"}
				},
				"required": ["repo_name"]
			}`),
		},
		{
			Name:        "find_symbol",
			Description: "Search for symbols (functions, classes, structs, methods, interfaces) across indexed repositories by name prefix.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string", "description": "Symbol name prefix to search for (case-sensitive)"},
					"kind": {"type": "string", "description": "Optional: filter by symbol kind (function, class, struct, method, interface, trait, enum, type, impl)"},
					"repo_name": {"type": "string", "description": "Optional: filter by repository name"}
				},
				"required": ["name"]
			}`),
		},
		{
			Name:        "find_references",
			Description: "Find all references to a function or symbol name across all indexed repositories. Shows every call site with file, line, and surrounding context.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string", "description": "Symbol name prefix to search for references (case-sensitive)"},
					"repo_name": {"type": "string", "description": "Optional: filter by repository name"}
				},
				"required": ["name"]
			}`),
		},
		{
			Name:        "trace_context",
			Description: "Trace what leads to a behavior by combining full-text search, AST symbol matches, references, and graph-linked files into one reasoning bundle.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "Behavior, symbol, or keyword phrase to trace"},
					"repo_name": {"type": "string", "description": "Optional repository filter"},
					"max_results": {"type": "integer", "description": "Maximum files to return (default 8)", "default": 8}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "get_file_outline",
			Description: "Get a compact outline of a file: metadata, symbols, chunks, and nearby references.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"repo_name": {"type": "string", "description": "Repository name"},
					"path": {"type": "string", "description": "File path relative to repository root"}
				},
				"required": ["repo_name", "path"]
			}`),
		},
		{
			Name:        "find_callers",
			Description: "Find exact call sites for a symbol name across repositories or within one repository.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string", "description": "Exact symbol name"},
					"repo_name": {"type": "string", "description": "Optional repository filter"},
					"max_results": {"type": "integer", "description": "Maximum callers to return (default 20)", "default": 20}
				},
				"required": ["name"]
			}`),
		},
		{
			Name:        "get_related_files",
			Description: "Show the graph-neighbor files for a given repo/path, useful for following a feature across the codebase.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"repo_name": {"type": "string", "description": "Repository name"},
					"path": {"type": "string", "description": "File path relative to repository root"},
					"max_results": {"type": "integer", "description": "Maximum related files to return (default 12)", "default": 12}
				},
				"required": ["repo_name", "path"]
			}`),
		},
		{
			Name:        "list_hub_files",
			Description: "List the most connected files in a repository. Great for finding architecture hubs and hot spots.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"repo_name": {"type": "string", "description": "Optional repository filter"},
					"max_results": {"type": "integer", "description": "Maximum files to return (default 15)", "default": 15}
				}
			}`),
		},
	}

	s.writeJSON(req.ID, JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: ListToolsResult{Tools: tools},
	})
}

func (s *Server) handleToolsCall(ctx context.Context, req JSONRPCRequest) {
	var callReq struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &callReq); err != nil {
		s.writeJSON(req.ID, JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &JSONRPCError{Code: -32602, Message: "invalid tool call params"},
		})
		return
	}

	handler, ok := s.tools[callReq.Name]
	if !ok {
		s.writeJSON(req.ID, JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &JSONRPCError{Code: -32601, Message: fmt.Sprintf("unknown tool: %s", callReq.Name)},
		})
		return
	}

	result := handler(ctx, callReq.Arguments)
	s.writeJSON(req.ID, JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: result,
	})
}

func (s *Server) handleResourcesList(ctx context.Context, req JSONRPCRequest) {
	repos, err := s.queries.ListRepos(ctx)
	if err != nil {
		s.writeJSON(req.ID, JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &JSONRPCError{Code: -32603, Message: fmt.Sprintf("list repos: %v", err)},
		})
		return
	}

	resources := make([]Resource, 0, len(repos)+1)
	resources = append(resources, Resource{
		URI:         "relith://repos",
		Name:        "All Repositories",
		Description: "List of all indexed repositories",
		MimeType:    "application/json",
	})
	for _, r := range repos {
		resources = append(resources, Resource{
			URI:         fmt.Sprintf("relith://repos/%d", r.ID),
			Name:        r.Name,
			Description: fmt.Sprintf("Repository: %s (%s)", r.Name, r.Path),
			MimeType:    "application/json",
		})
	}

	s.writeJSON(req.ID, JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: ListResourcesResult{Resources: resources},
	})
}

func (s *Server) handleResourcesRead(ctx context.Context, req JSONRPCRequest) {
	var readReq ReadResourceRequest
	if err := json.Unmarshal(req.Params, &readReq); err != nil {
		s.writeJSON(req.ID, JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &JSONRPCError{Code: -32602, Message: "invalid resource read params"},
		})
		return
	}

	for prefix, handler := range s.resources {
		if len(readReq.URI) >= len(prefix) && readReq.URI[:len(prefix)] == prefix {
			contents := handler(ctx, readReq.URI)
			s.writeJSON(req.ID, JSONRPCResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: ReadResourceResult{Contents: contents},
			})
			return
		}
	}

	s.writeJSON(req.ID, JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Error: &JSONRPCError{Code: -32601, Message: fmt.Sprintf("unknown resource: %s", readReq.URI)},
	})
}

func (s *Server) writeJSON(id json.RawMessage, resp JSONRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to marshal response")
		return
	}
	data = append(data, '\n')
	if _, err := s.writer.Write(data); err != nil {
		s.logger.Error().Err(err).Msg("failed to write response")
	}
}

const noReposHelp = "No repositories tracked. Add one with:\n  relith repo add <path>\n  relith index"

func (s *Server) hasRepos(ctx context.Context) (bool, error) {
	repos, err := s.queries.ListRepos(ctx)
	if err != nil {
		return false, err
	}
	return len(repos) > 0, nil
}

func (s *Server) textContent(text string) CallToolResult {
	return CallToolResult{
		Content: []ToolContent{{Type: "text", Text: text}},
		IsError: false,
	}
}

func (s *Server) errorContent(text string) CallToolResult {
	return CallToolResult{
		Content: []ToolContent{{Type: "text", Text: text}},
		IsError: true,
	}
}

func strParam(params map[string]any, key string) string {
	v, _ := params[key].(string)
	return v
}

func intParam(params map[string]any, key string, defaultVal int) int {
	v, ok := params[key].(float64)
	if !ok {
		return defaultVal
	}
	return int(v)
}

func resolveRepoPath(repoPath, docPath string) string {
	if filepath.IsAbs(docPath) {
		return docPath
	}
	return filepath.Join(repoPath, docPath)
}
