// Package mcp provides the MCP (Model Context Protocol) server implementation.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Server implements the MCP server.
type Server struct {
	mu           sync.RWMutex
	tools        []shared.MCPToolProvider
	port         int
	host         string
	running      bool
	toolRegistry map[string]shared.MCPTool
	httpServer   *http.Server
}

// Options holds configuration options for the MCP server.
type Options struct {
	Tools []shared.MCPToolProvider
	Port  int
	Host  string
}

// NewServer creates a new MCP server.
func NewServer(opts Options) *Server {
	port := opts.Port
	if port == 0 {
		port = 3000
	}
	host := opts.Host
	if host == "" {
		host = "localhost"
	}

	return &Server{
		tools:        opts.Tools,
		port:         port,
		host:         host,
		toolRegistry: make(map[string]shared.MCPTool),
	}
}

// Start starts the MCP server.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// Build tool registry
	for _, provider := range s.tools {
		for _, tool := range provider.GetTools() {
			s.toolRegistry[tool.Name] = tool
		}
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	mux.HandleFunc("/tools", s.handleListTools)
	mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	s.running = true

	// Start server in goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}
	}()

	return nil
}

// Stop stops the MCP server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	s.toolRegistry = make(map[string]shared.MCPTool)

	if s.httpServer != nil {
		return s.httpServer.Shutdown(context.Background())
	}

	return nil
}

// RegisterTool registers a tool.
func (s *Server) RegisterTool(tool shared.MCPTool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.toolRegistry[tool.Name] = tool
}

// ListTools returns all available tools.
func (s *Server) ListTools() []shared.MCPTool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]shared.MCPTool, 0, len(s.toolRegistry))
	for _, tool := range s.toolRegistry {
		tools = append(tools, tool)
	}
	return tools
}

// HandleRequest handles an MCP request.
func (s *Server) HandleRequest(ctx context.Context, request shared.MCPRequest) shared.MCPResponse {
	s.mu.RLock()
	providers := s.tools
	s.mu.RUnlock()

	// Find the tool provider that can handle this request
	for _, provider := range providers {
		result, err := provider.Execute(ctx, request.Method, request.Params)
		if err != nil {
			continue // Try next provider
		}

		return shared.MCPResponse{
			ID:     request.ID,
			Result: result,
		}
	}

	return shared.MCPResponse{
		ID: request.ID,
		Error: &shared.MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", request.Method),
		},
	}
}

// GetStatus returns the server status.
func (s *Server) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running":   s.running,
		"port":      s.port,
		"host":      s.host,
		"toolCount": len(s.toolRegistry),
	}
}

// AddToolProvider adds a tool provider.
func (s *Server) AddToolProvider(provider shared.MCPToolProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tools = append(s.tools, provider)

	// Register tools from the provider
	for _, tool := range provider.GetTools() {
		s.toolRegistry[tool.Name] = tool
	}
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, "", -32700, "Parse error")
		return
	}
	defer r.Body.Close()

	var request shared.MCPRequest
	if err := json.Unmarshal(body, &request); err != nil {
		s.writeError(w, "", -32700, "Parse error")
		return
	}

	response := s.HandleRequest(r.Context(), request)
	s.writeResponse(w, response)
}

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tools := s.ListTools()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": tools,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := s.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) writeResponse(w http.ResponseWriter, response shared.MCPResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) writeError(w http.ResponseWriter, id string, code int, message string) {
	response := shared.MCPResponse{
		ID: id,
		Error: &shared.MCPError{
			Code:    code,
			Message: message,
		},
	}
	s.writeResponse(w, response)
}

// ============================================================================
// STDIO Transport (for CLI usage)
// ============================================================================

// StdioTransport handles MCP communication over stdio.
type StdioTransport struct {
	server *Server
	reader io.Reader
	writer io.Writer
}

// NewStdioTransport creates a new stdio transport.
func NewStdioTransport(server *Server, reader io.Reader, writer io.Writer) *StdioTransport {
	return &StdioTransport{
		server: server,
		reader: reader,
		writer: writer,
	}
}

// Run runs the stdio transport.
func (t *StdioTransport) Run(ctx context.Context) error {
	decoder := json.NewDecoder(t.reader)
	encoder := json.NewEncoder(t.writer)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var request shared.MCPRequest
		if err := decoder.Decode(&request); err != nil {
			if err == io.EOF {
				return nil
			}
			continue
		}

		response := t.server.HandleRequest(ctx, request)
		if err := encoder.Encode(response); err != nil {
			continue
		}
	}
}
