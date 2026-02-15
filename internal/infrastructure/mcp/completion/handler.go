// Package completion provides MCP completion handler implementation.
package completion

import (
	"sort"
	"strings"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/prompts"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/resources"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// MaxCompletionResults is the maximum number of completion results to return.
const MaxCompletionResults = 10

// CompletionHandler handles completion requests.
type CompletionHandler struct {
	resources *resources.ResourceRegistry
	prompts   *prompts.PromptRegistry
}

// NewCompletionHandler creates a new CompletionHandler.
func NewCompletionHandler(res *resources.ResourceRegistry, pr *prompts.PromptRegistry) *CompletionHandler {
	return &CompletionHandler{
		resources: res,
		prompts:   pr,
	}
}

// Complete handles a completion request.
func (ch *CompletionHandler) Complete(ref *shared.CompletionReference, arg *shared.CompletionArgument) *shared.CompletionResult {
	if ch == nil || ref == nil {
		return emptyResult()
	}
	if arg == nil {
		arg = &shared.CompletionArgument{}
	}

	refType := shared.CompletionReferenceType(strings.TrimSpace(string(ref.Type)))
	switch refType {
	case shared.CompletionRefPrompt:
		return ch.completePrompt(ref.Name, arg)
	case shared.CompletionRefResource:
		return ch.completeResource(ref.URI, arg)
	default:
		return &shared.CompletionResult{
			Values:  []string{},
			Total:   0,
			HasMore: false,
		}
	}
}

// completePrompt completes prompt arguments.
func (ch *CompletionHandler) completePrompt(name string, arg *shared.CompletionArgument) *shared.CompletionResult {
	if ch.prompts == nil {
		return emptyResult()
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return emptyResult()
	}

	// Get prompt argument names
	argNames := ch.prompts.GetArgumentNames(name)
	if argNames == nil {
		return emptyResult()
	}

	// Filter by argument name prefix
	var matches []string
	prefix := strings.ToLower(strings.TrimSpace(arg.Value))

	for _, argName := range argNames {
		if strings.HasPrefix(strings.ToLower(argName), prefix) {
			matches = append(matches, argName)
		}
	}
	sort.Strings(matches)

	return buildResult(matches)
}

// completeResource completes resource URIs.
func (ch *CompletionHandler) completeResource(uriPrefix string, arg *shared.CompletionArgument) *shared.CompletionResult {
	if ch.resources == nil {
		return emptyResult()
	}

	// Use the value as prefix if provided, otherwise use uriPrefix
	prefix := strings.TrimSpace(arg.Value)
	if prefix == "" {
		prefix = strings.TrimSpace(uriPrefix)
	}

	// Find matching resources
	resources := ch.resources.ListResourcesByPrefix(prefix)
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].URI == resources[j].URI {
			return resources[i].Name < resources[j].Name
		}
		return resources[i].URI < resources[j].URI
	})

	// Extract URIs
	uris := make([]string, len(resources))
	for i, res := range resources {
		uris[i] = res.URI
	}

	return buildResult(uris)
}

// emptyResult returns an empty completion result.
func emptyResult() *shared.CompletionResult {
	return &shared.CompletionResult{
		Values:  []string{},
		Total:   0,
		HasMore: false,
	}
}

// buildResult builds a completion result with pagination.
func buildResult(matches []string) *shared.CompletionResult {
	if matches == nil {
		matches = []string{}
	}

	total := len(matches)
	hasMore := total > MaxCompletionResults

	if hasMore {
		matches = matches[:MaxCompletionResults]
	}

	return &shared.CompletionResult{
		Values:  matches,
		Total:   total,
		HasMore: hasMore,
	}
}
