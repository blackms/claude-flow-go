// Package mcp provides the MCP (Model Context Protocol) server implementation.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/completion"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/logging"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/prompts"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/resources"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/sampling"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/tasks"
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

	// MCP 2025-11-25 compliance components
	resources   *resources.ResourceRegistry
	prompts     *prompts.PromptRegistry
	sampling    *sampling.SamplingManager
	completion  *completion.CompletionHandler
	logging     *logging.LogManager
	capabilities *shared.MCPCapabilities

	// Task management
	tasks       *tasks.TaskManager
}

// Options holds configuration options for the MCP server.
type Options struct {
	Tools               []shared.MCPToolProvider
	Port                int
	Host                string
	ResourceCacheConfig *shared.ResourceCacheConfig
	SamplingConfig      *shared.SamplingConfig
	TaskManagerConfig   *shared.TaskManagerConfig
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

	// Create resource registry
	var resourceRegistry *resources.ResourceRegistry
	if opts.ResourceCacheConfig != nil {
		resourceRegistry = resources.NewResourceRegistry(*opts.ResourceCacheConfig)
	} else {
		resourceRegistry = resources.NewResourceRegistryWithDefaults()
	}

	// Create prompt registry
	promptRegistry := prompts.NewPromptRegistryWithDefaults()

	// Create sampling manager
	var samplingManager *sampling.SamplingManager
	if opts.SamplingConfig != nil {
		samplingManager = sampling.NewSamplingManager(*opts.SamplingConfig)
	} else {
		samplingManager = sampling.NewSamplingManagerWithDefaults()
	}

	// Create completion handler
	completionHandler := completion.NewCompletionHandler(resourceRegistry, promptRegistry)

	// Create log manager
	logManager := logging.NewLogManagerWithDefaults()

	// Create task manager
	var taskManager *tasks.TaskManager
	if opts.TaskManagerConfig != nil {
		taskManager = tasks.NewTaskManager(*opts.TaskManagerConfig, nil)
	} else {
		taskManager = tasks.NewTaskManagerWithDefaults(nil)
	}

	// Build capabilities
	capabilities := &shared.MCPCapabilities{
		Resources: &shared.ResourcesCapability{
			Subscribe:   true,
			ListChanged: true,
		},
		Prompts: &shared.PromptsCapability{
			ListChanged: true,
		},
		Tools: &shared.ToolsCapability{
			ListChanged: true,
		},
		Sampling: &shared.SamplingCapability{},
		Logging: &shared.LoggingCapability{
			Level: shared.MCPLogLevelInfo,
		},
	}

	return &Server{
		tools:        opts.Tools,
		port:         port,
		host:         host,
		toolRegistry: make(map[string]shared.MCPTool),
		resources:    resourceRegistry,
		prompts:      promptRegistry,
		sampling:     samplingManager,
		completion:   completionHandler,
		logging:      logManager,
		capabilities: capabilities,
		tasks:        taskManager,
	}
}

// Start starts the MCP server.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// Initialize task manager
	if s.tasks != nil {
		if err := s.tasks.Initialize(); err != nil {
			return err
		}
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

	// Shutdown task manager
	if s.tasks != nil {
		s.tasks.Shutdown()
	}

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
	// Handle MCP protocol methods first
	switch request.Method {
	case "resources/list":
		return s.handleResourcesList(request)
	case "resources/read":
		return s.handleResourcesRead(request)
	case "resources/subscribe":
		return s.handleResourcesSubscribe(request)
	case "resources/unsubscribe":
		return s.handleResourcesUnsubscribe(request)
	case "prompts/list":
		return s.handlePromptsList(request)
	case "prompts/get":
		return s.handlePromptsGet(request)
	case "sampling/createMessage":
		return s.handleSamplingCreateMessage(ctx, request)
	case "completion/complete":
		return s.handleCompletionComplete(request)
	case "logging/setLevel":
		return s.handleLoggingSetLevel(request)
	}

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

// ============================================================================
// MCP Protocol Method Handlers
// ============================================================================

func (s *Server) handleResourcesList(request shared.MCPRequest) shared.MCPResponse {
	cursor := ""
	pageSize := 100

	if request.Params != nil {
		if c, ok := request.Params["cursor"].(string); ok {
			cursor = c
		}
		if ps, ok := request.Params["pageSize"].(float64); ok {
			pageSize = int(ps)
		}
	}

	result := s.resources.List(cursor, pageSize)

	return shared.MCPResponse{
		ID:     request.ID,
		Result: result,
	}
}

func (s *Server) handleResourcesRead(request shared.MCPRequest) shared.MCPResponse {
	uri, ok := request.Params["uri"].(string)
	if !ok || uri == "" {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Invalid params: uri is required",
			},
		}
	}

	result, err := s.resources.Read(uri)
	if err != nil {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: err.Error(),
			},
		}
	}

	return shared.MCPResponse{
		ID:     request.ID,
		Result: result,
	}
}

func (s *Server) handleResourcesSubscribe(request shared.MCPRequest) shared.MCPResponse {
	uri, ok := request.Params["uri"].(string)
	if !ok || uri == "" {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Invalid params: uri is required",
			},
		}
	}

	// Subscribe with a no-op callback (real implementation would send notifications)
	subID := s.resources.Subscribe(uri, func(uri string, content *shared.ResourceContent) {
		// In a real implementation, this would send a notification to the client
		s.logging.Debug("Resource updated", map[string]interface{}{"uri": uri})
	})

	return shared.MCPResponse{
		ID: request.ID,
		Result: map[string]interface{}{
			"subscriptionId": subID,
		},
	}
}

func (s *Server) handleResourcesUnsubscribe(request shared.MCPRequest) shared.MCPResponse {
	subID, ok := request.Params["subscriptionId"].(string)
	if !ok || subID == "" {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Invalid params: subscriptionId is required",
			},
		}
	}

	success := s.resources.Unsubscribe(subID)

	return shared.MCPResponse{
		ID: request.ID,
		Result: map[string]interface{}{
			"success": success,
		},
	}
}

func (s *Server) handlePromptsList(request shared.MCPRequest) shared.MCPResponse {
	cursor := ""
	pageSize := 100

	if request.Params != nil {
		if c, ok := request.Params["cursor"].(string); ok {
			cursor = c
		}
		if ps, ok := request.Params["pageSize"].(float64); ok {
			pageSize = int(ps)
		}
	}

	result := s.prompts.List(cursor, pageSize)

	return shared.MCPResponse{
		ID:     request.ID,
		Result: result,
	}
}

func (s *Server) handlePromptsGet(request shared.MCPRequest) shared.MCPResponse {
	name, ok := request.Params["name"].(string)
	if !ok || name == "" {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Invalid params: name is required",
			},
		}
	}

	// Extract arguments
	args := make(map[string]string)
	if argsParam, ok := request.Params["arguments"].(map[string]interface{}); ok {
		for k, v := range argsParam {
			if str, ok := v.(string); ok {
				args[k] = str
			}
		}
	}

	result, err := s.prompts.Get(name, args)
	if err != nil {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: err.Error(),
			},
		}
	}

	return shared.MCPResponse{
		ID:     request.ID,
		Result: result,
	}
}

func (s *Server) handleSamplingCreateMessage(ctx context.Context, request shared.MCPRequest) shared.MCPResponse {
	// Parse request
	reqBytes, err := json.Marshal(request.Params)
	if err != nil {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	var createReq shared.CreateMessageRequest
	if err := json.Unmarshal(reqBytes, &createReq); err != nil {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Invalid params: " + err.Error(),
			},
		}
	}

	result, err := s.sampling.CreateMessageWithContext(ctx, &createReq)
	if err != nil {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32603,
				Message: err.Error(),
			},
		}
	}

	return shared.MCPResponse{
		ID:     request.ID,
		Result: result,
	}
}

func (s *Server) handleCompletionComplete(request shared.MCPRequest) shared.MCPResponse {
	// Parse reference
	refParam, ok := request.Params["ref"].(map[string]interface{})
	if !ok {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Invalid params: ref is required",
			},
		}
	}

	ref := &shared.CompletionReference{}
	if t, ok := refParam["type"].(string); ok {
		ref.Type = shared.CompletionReferenceType(t)
	}
	if n, ok := refParam["name"].(string); ok {
		ref.Name = n
	}
	if u, ok := refParam["uri"].(string); ok {
		ref.URI = u
	}

	// Parse argument
	argParam, ok := request.Params["argument"].(map[string]interface{})
	if !ok {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Invalid params: argument is required",
			},
		}
	}

	arg := &shared.CompletionArgument{}
	if n, ok := argParam["name"].(string); ok {
		arg.Name = n
	}
	if v, ok := argParam["value"].(string); ok {
		arg.Value = v
	}

	result := s.completion.Complete(ref, arg)

	return shared.MCPResponse{
		ID: request.ID,
		Result: map[string]interface{}{
			"completion": result,
		},
	}
}

func (s *Server) handleLoggingSetLevel(request shared.MCPRequest) shared.MCPResponse {
	level, ok := request.Params["level"].(string)
	if !ok || level == "" {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Invalid params: level is required",
			},
		}
	}

	if err := s.logging.SetLevel(level); err != nil {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: err.Error(),
			},
		}
	}

	// Update capabilities
	s.mu.Lock()
	s.capabilities.Logging.Level = shared.MCPLogLevel(level)
	s.mu.Unlock()

	return shared.MCPResponse{
		ID: request.ID,
		Result: map[string]interface{}{
			"success": true,
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

// ============================================================================
// Component Accessors
// ============================================================================

// GetResources returns the resource registry.
func (s *Server) GetResources() *resources.ResourceRegistry {
	return s.resources
}

// GetPrompts returns the prompt registry.
func (s *Server) GetPrompts() *prompts.PromptRegistry {
	return s.prompts
}

// GetSampling returns the sampling manager.
func (s *Server) GetSampling() *sampling.SamplingManager {
	return s.sampling
}

// GetCompletion returns the completion handler.
func (s *Server) GetCompletion() *completion.CompletionHandler {
	return s.completion
}

// GetLogging returns the log manager.
func (s *Server) GetLogging() *logging.LogManager {
	return s.logging
}

// GetCapabilities returns the server capabilities.
func (s *Server) GetCapabilities() *shared.MCPCapabilities {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.capabilities
}

// RegisterMockProvider registers a mock LLM provider for testing.
func (s *Server) RegisterMockProvider() {
	mockProvider := sampling.NewMockProviderWithDefaults()
	s.sampling.RegisterProvider(mockProvider, true)
}

// GetTasks returns the task manager.
func (s *Server) GetTasks() *tasks.TaskManager {
	return s.tasks
}
