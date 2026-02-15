// Package prompts provides MCP prompt registry implementation.
package prompts

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// PromptHandler is a function that generates prompt messages.
type PromptHandler func(args map[string]string) ([]shared.PromptMessage, error)

// PromptDefinition holds a prompt with its handler.
type PromptDefinition struct {
	Prompt  *shared.MCPPrompt
	Handler PromptHandler
}

func clonePrompt(prompt *shared.MCPPrompt) *shared.MCPPrompt {
	if prompt == nil {
		return nil
	}

	cloned := *prompt
	if prompt.Arguments != nil {
		cloned.Arguments = append([]shared.PromptArgument(nil), prompt.Arguments...)
	}
	return &cloned
}

// PromptRegistry manages MCP prompts.
type PromptRegistry struct {
	mu         sync.RWMutex
	prompts    map[string]*PromptDefinition
	maxPrompts int
}

// NewPromptRegistry creates a new PromptRegistry.
func NewPromptRegistry(maxPrompts int) *PromptRegistry {
	if maxPrompts <= 0 {
		maxPrompts = 1000
	}
	return &PromptRegistry{
		prompts:    make(map[string]*PromptDefinition),
		maxPrompts: maxPrompts,
	}
}

// NewPromptRegistryWithDefaults creates a PromptRegistry with default configuration.
func NewPromptRegistryWithDefaults() *PromptRegistry {
	return NewPromptRegistry(1000)
}

// Register registers a prompt with its handler.
func (pr *PromptRegistry) Register(prompt *shared.MCPPrompt, handler PromptHandler) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if len(pr.prompts) >= pr.maxPrompts {
		return shared.ErrMaxPromptsReached
	}

	pr.prompts[prompt.Name] = &PromptDefinition{
		Prompt:  clonePrompt(prompt),
		Handler: handler,
	}

	return nil
}

// Unregister removes a prompt.
func (pr *PromptRegistry) Unregister(name string) bool {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if _, exists := pr.prompts[name]; exists {
		delete(pr.prompts, name)
		return true
	}
	return false
}

// List returns a paginated list of prompts.
func (pr *PromptRegistry) List(cursor string, pageSize int) *shared.PromptListResult {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	if pageSize <= 0 {
		pageSize = 100
	}

	// Collect all prompts
	allPrompts := make([]*shared.MCPPrompt, 0, len(pr.prompts))
	for _, def := range pr.prompts {
		allPrompts = append(allPrompts, def.Prompt)
	}

	// Sort by name for consistent pagination
	sort.Slice(allPrompts, func(i, j int) bool {
		return allPrompts[i].Name < allPrompts[j].Name
	})

	// Find starting point
	startIdx := 0
	if cursor != "" {
		startIdx = len(allPrompts)
		for i, p := range allPrompts {
			if p.Name > cursor {
				startIdx = i
				break
			}
		}
	}

	// Get page
	endIdx := startIdx + pageSize
	if endIdx > len(allPrompts) {
		endIdx = len(allPrompts)
	}

	page := allPrompts[startIdx:endIdx]
	prompts := make([]shared.MCPPrompt, len(page))
	for i, p := range page {
		prompts[i] = *clonePrompt(p)
	}

	result := &shared.PromptListResult{
		Prompts: prompts,
	}

	if endIdx < len(allPrompts) {
		result.NextCursor = page[len(page)-1].Name
	}

	return result
}

// Get retrieves a prompt and generates its messages with the given arguments.
func (pr *PromptRegistry) Get(name string, args map[string]string) (*shared.PromptGetResult, error) {
	pr.mu.RLock()
	def, exists := pr.prompts[name]
	pr.mu.RUnlock()

	if !exists {
		return nil, shared.ErrPromptNotFound
	}

	// Validate required arguments
	if err := pr.validateArguments(def.Prompt, args); err != nil {
		return nil, err
	}

	// Generate messages
	messages, err := def.Handler(args)
	if err != nil {
		return nil, err
	}

	return &shared.PromptGetResult{
		Description: def.Prompt.Description,
		Messages:    messages,
	}, nil
}

// validateArguments validates that all required arguments are provided.
func (pr *PromptRegistry) validateArguments(prompt *shared.MCPPrompt, args map[string]string) error {
	for _, arg := range prompt.Arguments {
		if arg.Required {
			if _, exists := args[arg.Name]; !exists {
				return shared.ErrMissingRequiredArgument
			}
		}
	}
	return nil
}

// GetPrompt returns the prompt definition by name.
func (pr *PromptRegistry) GetPrompt(name string) *shared.MCPPrompt {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	def, exists := pr.prompts[name]
	if !exists {
		return nil
	}
	return def.Prompt
}

// HasPrompt checks if a prompt exists.
func (pr *PromptRegistry) HasPrompt(name string) bool {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	_, exists := pr.prompts[name]
	return exists
}

// GetArgumentNames returns the argument names for a prompt.
func (pr *PromptRegistry) GetArgumentNames(name string) []string {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	def, exists := pr.prompts[name]
	if !exists {
		return nil
	}

	names := make([]string, len(def.Prompt.Arguments))
	for i, arg := range def.Prompt.Arguments {
		names[i] = arg.Name
	}
	return names
}

// Count returns the number of registered prompts.
func (pr *PromptRegistry) Count() int {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return len(pr.prompts)
}

// Helper functions for creating prompts

// Interpolate replaces {{arg}} placeholders with argument values.
func Interpolate(template string, args map[string]string) string {
	result := template
	for name, value := range args {
		placeholder := "{{" + name + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// InterpolateRegex replaces argument placeholders using regex for more flexibility.
func InterpolateRegex(template string, args map[string]string) string {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	return re.ReplaceAllStringFunc(template, func(match string) string {
		name := match[2 : len(match)-2] // Extract name from {{name}}
		if value, exists := args[name]; exists {
			return value
		}
		return match // Leave placeholder if not found
	})
}

// DefinePrompt creates a prompt definition helper.
func DefinePrompt(name, title, description string, args []shared.PromptArgument) *shared.MCPPrompt {
	return &shared.MCPPrompt{
		Name:        name,
		Title:       title,
		Description: description,
		Arguments:   args,
	}
}

// TextMessage creates a text content message.
func TextMessage(role, text string) shared.PromptMessage {
	return shared.PromptMessage{
		Role: role,
		Content: []shared.PromptContent{
			{
				Type: shared.PromptContentTypeText,
				Text: text,
			},
		},
	}
}

// ResourceMessage creates a message with embedded resource.
func ResourceMessage(role, uri, mimeType string) shared.PromptMessage {
	return shared.PromptMessage{
		Role: role,
		Content: []shared.PromptContent{
			{
				Type:        shared.PromptContentTypeResource,
				ResourceURI: uri,
				MimeType:    mimeType,
			},
		},
	}
}

// RequiredArg creates a required argument definition.
func RequiredArg(name, description string) shared.PromptArgument {
	return shared.PromptArgument{
		Name:        name,
		Description: description,
		Required:    true,
	}
}

// OptionalArg creates an optional argument definition.
func OptionalArg(name, description string) shared.PromptArgument {
	return shared.PromptArgument{
		Name:        name,
		Description: description,
		Required:    false,
	}
}
