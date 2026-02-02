// Package security provides security infrastructure implementations.
package security

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Executor errors.
var (
	ErrEmptyAllowlist     = errors.New("at least one allowed command must be specified")
	ErrDangerousCommand   = errors.New("dangerous command not allowed")
	ErrCommandNotAllowed  = errors.New("command not in allowlist")
	ErrSudoNotAllowed     = errors.New("sudo commands are not allowed")
	ErrNullByteInArg      = errors.New("null byte detected in argument")
	ErrDangerousPattern   = errors.New("dangerous pattern detected in argument")
	ErrCommandChaining    = errors.New("potential command chaining detected")
	ErrExecutionTimeout   = errors.New("command execution timed out")
	ErrCommandNotFound    = errors.New("command not found")
	ErrExecutionFailed    = errors.New("command execution failed")
)

// Default blocked argument patterns.
var defaultBlockedPatterns = []string{
	";",
	"&&",
	"||",
	"|",
	"`",
	"$(",
	"${",
	">",
	"<",
	">>",
	"&",
	"\n",
	"\r",
	"\x00",
	"$()",
}

// Dangerous commands that should never be allowed.
var dangerousCommands = []string{
	"rm",
	"rmdir",
	"del",
	"format",
	"mkfs",
	"dd",
	"chmod",
	"chown",
	"kill",
	"killall",
	"pkill",
	"reboot",
	"shutdown",
	"init",
	"poweroff",
	"halt",
}

// SafeExecutorConfig configures the safe executor.
type SafeExecutorConfig struct {
	// AllowedCommands is the allowlist of commands.
	AllowedCommands []string

	// BlockedPatterns are patterns that are rejected in arguments.
	BlockedPatterns []string

	// Timeout is the maximum execution timeout (default: 30s).
	Timeout time.Duration

	// MaxBuffer is the maximum buffer size for stdout/stderr (default: 10MB).
	MaxBuffer int

	// WorkingDir is the working directory for command execution.
	WorkingDir string

	// Environment variables to include.
	Env []string

	// AllowSudo allows sudo commands (default: false).
	AllowSudo bool
}

// DefaultSafeExecutorConfig returns the default configuration.
func DefaultSafeExecutorConfig(allowedCommands []string) SafeExecutorConfig {
	return SafeExecutorConfig{
		AllowedCommands: allowedCommands,
		BlockedPatterns: defaultBlockedPatterns,
		Timeout:         30 * time.Second,
		MaxBuffer:       10 * 1024 * 1024, // 10MB
		WorkingDir:      "",
		Env:             nil,
		AllowSudo:       false,
	}
}

// ExecutionResult contains the result of command execution.
type ExecutionResult struct {
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	ExitCode int           `json:"exitCode"`
	Command  string        `json:"command"`
	Args     []string      `json:"args"`
	Duration time.Duration `json:"duration"`
}

// SafeExecutor provides safe command execution.
type SafeExecutor struct {
	config          SafeExecutorConfig
	blockedPatterns []*regexp.Regexp
}

// NewSafeExecutor creates a new safe executor.
func NewSafeExecutor(config SafeExecutorConfig) (*SafeExecutor, error) {
	if len(config.AllowedCommands) == 0 {
		return nil, ErrEmptyAllowlist
	}

	// Check for dangerous commands in allowlist
	for _, cmd := range config.AllowedCommands {
		basename := filepath.Base(cmd)
		for _, dangerous := range dangerousCommands {
			if basename == dangerous {
				return nil, fmt.Errorf("%w: %s", ErrDangerousCommand, cmd)
			}
		}
	}

	// Compile blocked patterns
	blockedPatterns := make([]*regexp.Regexp, 0, len(config.BlockedPatterns))
	for _, pattern := range config.BlockedPatterns {
		escaped := regexp.QuoteMeta(pattern)
		re, err := regexp.Compile("(?i)" + escaped)
		if err != nil {
			continue
		}
		blockedPatterns = append(blockedPatterns, re)
	}

	return &SafeExecutor{
		config:          config,
		blockedPatterns: blockedPatterns,
	}, nil
}

// validateCommand validates a command against the allowlist.
func (e *SafeExecutor) validateCommand(command string) error {
	basename := filepath.Base(command)

	// Check for sudo
	if !e.config.AllowSudo && (command == "sudo" || basename == "sudo") {
		return ErrSudoNotAllowed
	}

	// Check if command is allowed
	for _, allowed := range e.config.AllowedCommands {
		allowedBasename := filepath.Base(allowed)
		if command == allowed || basename == allowedBasename {
			return nil
		}
	}

	return fmt.Errorf("%w: %s", ErrCommandNotAllowed, command)
}

// validateArguments validates command arguments for injection patterns.
func (e *SafeExecutor) validateArguments(args []string) error {
	for _, arg := range args {
		// Check for null bytes
		if strings.Contains(arg, "\x00") {
			return ErrNullByteInArg
		}

		// Check against blocked patterns
		for _, pattern := range e.blockedPatterns {
			if pattern.MatchString(arg) {
				return fmt.Errorf("%w: %s", ErrDangerousPattern, arg)
			}
		}

		// Check for command chaining attempts
		if strings.HasPrefix(arg, "-") && regexp.MustCompile(`[;&|]`).MatchString(arg) {
			return fmt.Errorf("%w: %s", ErrCommandChaining, arg)
		}
	}

	return nil
}

// SanitizeArgument sanitizes a single argument.
func (e *SafeExecutor) SanitizeArgument(arg string) string {
	result := strings.ReplaceAll(arg, "\x00", "")
	result = regexp.MustCompile(`[;&|` + "`" + `$(){}><\n\r]`).ReplaceAllString(result, "")
	return result
}

// Execute executes a command safely.
func (e *SafeExecutor) Execute(command string, args ...string) (*ExecutionResult, error) {
	return e.ExecuteWithContext(context.Background(), command, args...)
}

// ExecuteWithContext executes a command with context.
func (e *SafeExecutor) ExecuteWithContext(ctx context.Context, command string, args ...string) (*ExecutionResult, error) {
	startTime := time.Now()

	// Validate command
	if err := e.validateCommand(command); err != nil {
		return nil, err
	}

	// Validate arguments
	if err := e.validateArguments(args); err != nil {
		return nil, err
	}

	// Create context with timeout
	if e.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.config.Timeout)
		defer cancel()
	}

	// Create command - NEVER use shell
	cmd := exec.CommandContext(ctx, command, args...)

	// Set working directory
	if e.config.WorkingDir != "" {
		cmd.Dir = e.config.WorkingDir
	}

	// Set environment
	if e.config.Env != nil {
		cmd.Env = e.config.Env
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	err := cmd.Run()

	result := &ExecutionResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
		Command:  command,
		Args:     args,
		Duration: time.Since(startTime),
	}

	if err != nil {
		// Check for timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrExecutionTimeout
		}

		// Check for command not found
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			// Still return the result with non-zero exit code
			return result, nil
		}

		if execErr, ok := err.(*exec.Error); ok {
			if execErr.Err == exec.ErrNotFound {
				return nil, fmt.Errorf("%w: %s", ErrCommandNotFound, command)
			}
		}

		return nil, fmt.Errorf("%w: %v", ErrExecutionFailed, err)
	}

	return result, nil
}

// AllowCommand adds a command to the allowlist at runtime.
func (e *SafeExecutor) AllowCommand(command string) error {
	basename := filepath.Base(command)

	// Check for dangerous commands
	for _, dangerous := range dangerousCommands {
		if basename == dangerous {
			return fmt.Errorf("%w: %s", ErrDangerousCommand, command)
		}
	}

	// Check if already in allowlist
	for _, allowed := range e.config.AllowedCommands {
		if allowed == command {
			return nil
		}
	}

	e.config.AllowedCommands = append(e.config.AllowedCommands, command)
	return nil
}

// IsCommandAllowed checks if a command is allowed.
func (e *SafeExecutor) IsCommandAllowed(command string) bool {
	return e.validateCommand(command) == nil
}

// GetAllowedCommands returns the current allowlist.
func (e *SafeExecutor) GetAllowedCommands() []string {
	result := make([]string, len(e.config.AllowedCommands))
	copy(result, e.config.AllowedCommands)
	return result
}

// SetWorkingDir sets the working directory.
func (e *SafeExecutor) SetWorkingDir(dir string) {
	e.config.WorkingDir = dir
}

// SetTimeout sets the execution timeout.
func (e *SafeExecutor) SetTimeout(timeout time.Duration) {
	e.config.Timeout = timeout
}

// CreateDevelopmentExecutor creates an executor for common development tasks.
func CreateDevelopmentExecutor() (*SafeExecutor, error) {
	config := SafeExecutorConfig{
		AllowedCommands: []string{
			"git",
			"npm",
			"npx",
			"node",
			"go",
			"tsc",
			"vitest",
			"jest",
			"eslint",
			"prettier",
			"cargo",
			"python",
			"python3",
			"pip",
			"pip3",
		},
		BlockedPatterns: defaultBlockedPatterns,
		Timeout:         30 * time.Second,
		MaxBuffer:       10 * 1024 * 1024,
		AllowSudo:       false,
	}

	return NewSafeExecutor(config)
}

// CreateReadOnlyExecutor creates an executor for read-only operations.
func CreateReadOnlyExecutor() (*SafeExecutor, error) {
	config := SafeExecutorConfig{
		AllowedCommands: []string{
			"git",
			"cat",
			"head",
			"tail",
			"ls",
			"find",
			"grep",
			"which",
			"echo",
			"pwd",
			"env",
			"whoami",
			"hostname",
			"uname",
		},
		BlockedPatterns: defaultBlockedPatterns,
		Timeout:         10 * time.Second,
		MaxBuffer:       10 * 1024 * 1024,
		AllowSudo:       false,
	}

	return NewSafeExecutor(config)
}

// CreateGitExecutor creates an executor for git operations only.
func CreateGitExecutor() (*SafeExecutor, error) {
	config := SafeExecutorConfig{
		AllowedCommands: []string{"git"},
		BlockedPatterns: defaultBlockedPatterns,
		Timeout:         60 * time.Second,
		MaxBuffer:       50 * 1024 * 1024, // 50MB for large diffs
		AllowSudo:       false,
	}

	return NewSafeExecutor(config)
}

// IsDangerousCommand checks if a command is in the dangerous commands list.
func IsDangerousCommand(command string) bool {
	basename := filepath.Base(command)
	for _, dangerous := range dangerousCommands {
		if basename == dangerous {
			return true
		}
	}
	return false
}

// GetDangerousCommands returns the list of dangerous commands.
func GetDangerousCommands() []string {
	result := make([]string, len(dangerousCommands))
	copy(result, dangerousCommands)
	return result
}
