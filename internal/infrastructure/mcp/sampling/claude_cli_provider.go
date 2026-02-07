// Package sampling provides MCP sampling API implementation.
package sampling

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ClaudeCLIConfig holds configuration for the ClaudeCLIProvider.
type ClaudeCLIConfig struct {
	ClaudePath    string        // Path to claude binary (default: "claude")
	MaxConcurrent int           // Maximum concurrent processes (default: 5)
	Timeout       time.Duration // Per-request timeout (default: 120s)
}

// ClaudeCLIProvider implements LLMProvider using the claude CLI subprocess.
type ClaudeCLIProvider struct {
	name      string
	path      string
	timeout   time.Duration
	semaphore chan struct{}
}

// modelMap maps model hint names to full Claude model IDs.
var modelMap = map[string]string{
	"opus":   "claude-opus-4-6",
	"sonnet": "claude-sonnet-4-5-20250929",
	"haiku":  "claude-haiku-4-5-20251001",
}

// NewClaudeCLIProvider creates a new ClaudeCLIProvider.
func NewClaudeCLIProvider(cfg ClaudeCLIConfig) *ClaudeCLIProvider {
	path := cfg.ClaudePath
	if path == "" {
		path = "claude"
	}
	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &ClaudeCLIProvider{
		name:      "claude-cli",
		path:      path,
		timeout:   timeout,
		semaphore: make(chan struct{}, maxConcurrent),
	}
}

// Name returns the provider name.
func (p *ClaudeCLIProvider) Name() string {
	return p.name
}

// IsAvailable checks if the claude CLI is installed and reachable.
func (p *ClaudeCLIProvider) IsAvailable() bool {
	_, err := exec.LookPath(p.path)
	return err == nil
}

// CreateMessage calls the claude CLI to generate a response.
func (p *ClaudeCLIProvider) CreateMessage(ctx context.Context, req *shared.CreateMessageRequest) (*shared.CreateMessageResult, error) {
	// Acquire semaphore slot.
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	model := resolveModel(req.ModelPreferences)

	args := []string{
		"-p",
		"--model", model,
		"--output-format", "text",
	}

	// Pass system prompt via --system-prompt flag if provided.
	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	// Build input from messages.
	var input strings.Builder
	for _, msg := range req.Messages {
		for _, c := range msg.Content {
			if c.Text != "" {
				input.WriteString(c.Text)
				input.WriteString("\n")
			}
		}
	}

	// Apply timeout from config if ctx doesn't already have a deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, p.path, args...)
	cmd.Stdin = bytes.NewReader([]byte(input.String()))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, shared.ErrSamplingTimeout
		}
		return nil, fmt.Errorf("claude cli error: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return &shared.CreateMessageResult{
		Role:       "assistant",
		Content:    strings.TrimSpace(stdout.String()),
		Model:      model,
		StopReason: "endTurn",
	}, nil
}

// resolveModel picks a model ID from preferences, falling back to sonnet.
func resolveModel(prefs *shared.ModelPreferences) string {
	if prefs != nil {
		for _, hint := range prefs.Hints {
			if id, ok := modelMap[hint.Name]; ok {
				return id
			}
			// Allow full model IDs directly.
			if strings.HasPrefix(hint.Name, "claude-") {
				return hint.Name
			}
		}
	}
	return modelMap["sonnet"]
}
