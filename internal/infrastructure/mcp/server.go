// Package mcp provides the MCP (Model Context Protocol) server implementation.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/hooks"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/completion"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/logging"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/prompts"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/resources"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/sampling"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/sessions"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/tasks"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/tools"
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
	resources    *resources.ResourceRegistry
	prompts      *prompts.PromptRegistry
	sampling     *sampling.SamplingManager
	completion   *completion.CompletionHandler
	logging      *logging.LogManager
	capabilities *shared.MCPCapabilities

	// Task management
	tasks *tasks.TaskManager

	// Hooks system
	hooks *hooks.HooksManager

	// Session management
	sessions *sessions.SessionManager
}

// Options holds configuration options for the MCP server.
type Options struct {
	Tools               []shared.MCPToolProvider
	Port                int
	Host                string
	ResourceCacheConfig *shared.ResourceCacheConfig
	SamplingConfig      *shared.SamplingConfig
	TaskManagerConfig   *shared.TaskManagerConfig
	HooksConfig         *shared.HooksConfig
	SessionConfig       *shared.SessionConfig
}

func (s *Server) isConfigured() bool {
	if s == nil {
		return false
	}
	if s.toolRegistry == nil {
		return false
	}
	if s.resources == nil || s.prompts == nil || s.sampling == nil || s.completion == nil || s.logging == nil {
		return false
	}
	if s.capabilities == nil || s.tasks == nil || s.hooks == nil || s.sessions == nil {
		return false
	}
	return true
}

func (s *Server) configuredOrError() error {
	if !s.isConfigured() {
		return fmt.Errorf("mcp server is not initialized")
	}
	return nil
}

func cloneCapabilities(c *shared.MCPCapabilities) *shared.MCPCapabilities {
	if c == nil {
		return nil
	}

	cloned := *c
	if c.Experimental != nil {
		experimental := make(map[string]interface{}, len(c.Experimental))
		for key, value := range c.Experimental {
			experimental[key] = value
		}
		cloned.Experimental = experimental
	}
	if c.Logging != nil {
		logging := *c.Logging
		cloned.Logging = &logging
	}
	if c.Prompts != nil {
		prompts := *c.Prompts
		cloned.Prompts = &prompts
	}
	if c.Resources != nil {
		resources := *c.Resources
		cloned.Resources = &resources
	}
	if c.Tools != nil {
		tools := *c.Tools
		cloned.Tools = &tools
	}
	if c.Sampling != nil {
		sampling := *c.Sampling
		cloned.Sampling = &sampling
	}
	return &cloned
}

func normalizeToolName(name string) string {
	return strings.TrimSpace(name)
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

	// Create hooks manager
	var hooksManager *hooks.HooksManager
	if opts.HooksConfig != nil {
		hooksManager = hooks.NewHooksManager(*opts.HooksConfig)
	} else {
		hooksManager = hooks.NewHooksManagerWithDefaults()
	}

	// Create session manager
	var sessionManager *sessions.SessionManager
	if opts.SessionConfig != nil {
		sessionManager = sessions.NewSessionManager(*opts.SessionConfig)
	} else {
		sessionManager = sessions.NewSessionManagerWithDefaults()
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

	// Create HooksTools provider
	hooksTools := tools.NewHooksTools(hooksManager)

	// Combine user-provided tools with built-in tools
	allTools := make([]shared.MCPToolProvider, 0, len(opts.Tools)+1)
	for _, provider := range opts.Tools {
		if provider != nil {
			allTools = append(allTools, provider)
		}
	}
	if hooksTools != nil {
		allTools = append(allTools, hooksTools)
	}

	return &Server{
		tools:        allTools,
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
		hooks:        hooksManager,
		sessions:     sessionManager,
	}
}

// Start starts the MCP server.
func (s *Server) Start() error {
	if err := s.configuredOrError(); err != nil {
		return err
	}

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

	// Initialize hooks manager
	if s.hooks != nil {
		if err := s.hooks.Initialize(); err != nil {
			return err
		}
	}

	// Initialize session manager
	if s.sessions != nil {
		if err := s.sessions.Initialize(); err != nil {
			return err
		}
	}

	// Build tool registry
	for _, provider := range s.tools {
		if provider == nil {
			continue
		}
		for _, tool := range provider.GetTools() {
			toolName := normalizeToolName(tool.Name)
			if toolName == "" {
				continue
			}
			tool.Name = toolName
			s.toolRegistry[toolName] = tool
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
	if err := s.configuredOrError(); err != nil {
		return err
	}

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

	// Shutdown hooks manager
	if s.hooks != nil {
		s.hooks.Shutdown()
	}

	// Shutdown session manager
	if s.sessions != nil {
		s.sessions.Shutdown()
	}

	if s.httpServer != nil {
		return s.httpServer.Shutdown(context.Background())
	}

	return nil
}

// RegisterTool registers a tool.
func (s *Server) RegisterTool(tool shared.MCPTool) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.toolRegistry == nil {
		s.toolRegistry = make(map[string]shared.MCPTool)
	}
	toolName := normalizeToolName(tool.Name)
	if toolName == "" {
		return
	}
	tool.Name = toolName
	s.toolRegistry[toolName] = tool
}

// ListTools returns all available tools.
func (s *Server) ListTools() []shared.MCPTool {
	if s == nil {
		return []shared.MCPTool{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]bool)
	result := make([]shared.MCPTool, 0)

	// Add tools from registry
	for _, tool := range s.toolRegistry {
		toolName := normalizeToolName(tool.Name)
		if toolName == "" {
			continue
		}
		tool.Name = toolName
		if !seen[toolName] {
			result = append(result, tool)
			seen[toolName] = true
		}
	}

	// Add tools from providers
	for _, provider := range s.tools {
		if provider == nil {
			continue
		}
		for _, tool := range provider.GetTools() {
			toolName := normalizeToolName(tool.Name)
			if toolName == "" {
				continue
			}
			tool.Name = toolName
			if !seen[toolName] {
				result = append(result, tool)
				seen[toolName] = true
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// HandleRequest handles an MCP request.
func (s *Server) HandleRequest(ctx context.Context, request shared.MCPRequest) shared.MCPResponse {
	if !s.isConfigured() {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32603,
				Message: "mcp server is not initialized",
			},
		}
	}

	method := strings.TrimSpace(request.Method)

	// Handle MCP protocol methods first
	switch method {
	case "initialize":
		return s.handleInitialize(request)
	case "notifications/initialized":
		return shared.MCPResponse{ID: request.ID}
	case "tools/list":
		return s.handleToolsList(request)
	case "tools/call":
		return s.handleToolsCall(ctx, request)
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
		if provider == nil {
			continue // Ignore malformed provider entries
		}
		result, err := provider.Execute(ctx, method, request.Params)
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
			Message: fmt.Sprintf("Method not found: %s", method),
		},
	}
}

// ============================================================================
// MCP Protocol Method Handlers
// ============================================================================

func (s *Server) handleInitialize(request shared.MCPRequest) shared.MCPResponse {
	return shared.MCPResponse{
		ID: request.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2025-11-25",
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{},
				"resources": map[string]interface{}{},
				"prompts":   map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "claude-flow",
				"version": "3.0.0-alpha.1",
			},
		},
	}
}

func (s *Server) handleToolsList(request shared.MCPRequest) shared.MCPResponse {
	toolsList := s.ListTools()
	mcpTools := make([]map[string]interface{}, 0, len(toolsList))
	for _, tool := range toolsList {
		t := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
		}
		if tool.Parameters != nil {
			t["inputSchema"] = tool.Parameters
		} else {
			t["inputSchema"] = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}
		mcpTools = append(mcpTools, t)
	}
	return shared.MCPResponse{
		ID:     request.ID,
		Result: map[string]interface{}{"tools": mcpTools},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, request shared.MCPRequest) shared.MCPResponse {
	toolName, _ := request.Params["name"].(string)
	toolName = normalizeToolName(toolName)
	arguments, _ := request.Params["arguments"].(map[string]interface{})

	if toolName == "" {
		return shared.MCPResponse{
			ID: request.ID,
			Error: &shared.MCPError{
				Code:    -32602,
				Message: "Missing tool name",
			},
		}
	}

	s.mu.RLock()
	providers := s.tools
	s.mu.RUnlock()

	for _, provider := range providers {
		if provider == nil {
			continue
		}
		result, err := provider.Execute(ctx, toolName, arguments)
		if err != nil {
			continue
		}
		resultJSON, _ := json.Marshal(result)
		return shared.MCPResponse{
			ID: request.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": string(resultJSON),
					},
				},
			},
		}
	}

	return shared.MCPResponse{
		ID: request.ID,
		Error: &shared.MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Tool not found: %s", toolName),
		},
	}
}

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
	if !s.isConfigured() {
		return map[string]interface{}{
			"running": false,
			"error":   "mcp server is not initialized",
		}
	}

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
	if s == nil || provider == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.toolRegistry == nil {
		s.toolRegistry = make(map[string]shared.MCPTool)
	}
	s.tools = append(s.tools, provider)

	// Register tools from the provider
	for _, tool := range provider.GetTools() {
		toolName := normalizeToolName(tool.Name)
		if toolName == "" {
			continue
		}
		tool.Name = toolName
		s.toolRegistry[toolName] = tool
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
	if t == nil || t.server == nil {
		return fmt.Errorf("stdio transport server is not configured")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if t.reader == nil {
		return fmt.Errorf("reader is required")
	}
	if t.writer == nil {
		return fmt.Errorf("writer is required")
	}

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
	if s == nil {
		return nil
	}
	return s.resources
}

// GetPrompts returns the prompt registry.
func (s *Server) GetPrompts() *prompts.PromptRegistry {
	if s == nil {
		return nil
	}
	return s.prompts
}

// GetSampling returns the sampling manager.
func (s *Server) GetSampling() *sampling.SamplingManager {
	if s == nil {
		return nil
	}
	return s.sampling
}

// GetCompletion returns the completion handler.
func (s *Server) GetCompletion() *completion.CompletionHandler {
	if s == nil {
		return nil
	}
	return s.completion
}

// GetLogging returns the log manager.
func (s *Server) GetLogging() *logging.LogManager {
	if s == nil {
		return nil
	}
	return s.logging
}

// GetCapabilities returns the server capabilities.
func (s *Server) GetCapabilities() *shared.MCPCapabilities {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneCapabilities(s.capabilities)
}

// RegisterMockProvider registers a mock LLM provider for testing.
func (s *Server) RegisterMockProvider() {
	if s == nil || s.sampling == nil {
		return
	}
	mockProvider := sampling.NewMockProviderWithDefaults()
	s.sampling.RegisterProvider(mockProvider, true)
}

// GetTasks returns the task manager.
func (s *Server) GetTasks() *tasks.TaskManager {
	if s == nil {
		return nil
	}
	return s.tasks
}

// GetHooks returns the hooks manager.
func (s *Server) GetHooks() *hooks.HooksManager {
	if s == nil {
		return nil
	}
	return s.hooks
}

// GetSessions returns the session manager.
func (s *Server) GetSessions() *sessions.SessionManager {
	if s == nil {
		return nil
	}
	return s.sessions
}
