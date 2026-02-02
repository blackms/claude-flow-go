// Package utility provides utility services for diagnostics, daemon management, and benchmarking.
package utility

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// CheckStatus represents the status of a diagnostic check.
type CheckStatus string

const (
	CheckStatusPass CheckStatus = "pass"
	CheckStatusWarn CheckStatus = "warn"
	CheckStatusFail CheckStatus = "fail"
)

// CheckResult represents the result of a single diagnostic check.
type CheckResult struct {
	Name     string        `json:"name"`
	Status   CheckStatus   `json:"status"`
	Message  string        `json:"message"`
	Fix      string        `json:"fix,omitempty"`
	Duration time.Duration `json:"duration"`
}

// Summary holds the summary of all checks.
type Summary struct {
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Failed   int `json:"failed"`
	Total    int `json:"total"`
}

// DiagnosticReport holds the complete diagnostic report.
type DiagnosticReport struct {
	Version   string        `json:"version"`
	Timestamp time.Time     `json:"timestamp"`
	Platform  string        `json:"platform"`
	Checks    []CheckResult `json:"checks"`
	Summary   Summary       `json:"summary"`
}

// DoctorService provides diagnostic functionality.
type DoctorService struct {
	version  string
	basePath string
}

// NewDoctorService creates a new doctor service.
func NewDoctorService(version string) *DoctorService {
	home, _ := os.UserHomeDir()
	return &DoctorService{
		version:  version,
		basePath: filepath.Join(home, ".claude-flow"),
	}
}

// RunAllChecks runs all diagnostic checks.
func (d *DoctorService) RunAllChecks(verbose bool) *DiagnosticReport {
	report := &DiagnosticReport{
		Version:   d.version,
		Timestamp: time.Now(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Checks:    make([]CheckResult, 0),
	}

	// Run checks in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	checks := []func() CheckResult{
		d.checkVersion,
		d.checkGoRuntime,
		d.checkConfig,
		d.checkDaemon,
		d.checkMemoryDB,
		d.checkDiskSpace,
		d.checkEnvVars,
		d.checkGit,
	}

	for _, check := range checks {
		wg.Add(1)
		go func(checkFn func() CheckResult) {
			defer wg.Done()
			result := checkFn()
			mu.Lock()
			report.Checks = append(report.Checks, result)
			mu.Unlock()
		}(check)
	}

	wg.Wait()

	// Calculate summary
	for _, check := range report.Checks {
		switch check.Status {
		case CheckStatusPass:
			report.Summary.Passed++
		case CheckStatusWarn:
			report.Summary.Warnings++
		case CheckStatusFail:
			report.Summary.Failed++
		}
		report.Summary.Total++
	}

	return report
}

// RunCheck runs a specific check by name.
func (d *DoctorService) RunCheck(name string) (*CheckResult, error) {
	checks := map[string]func() CheckResult{
		"version": d.checkVersion,
		"go":      d.checkGoRuntime,
		"config":  d.checkConfig,
		"daemon":  d.checkDaemon,
		"memory":  d.checkMemoryDB,
		"disk":    d.checkDiskSpace,
		"env":     d.checkEnvVars,
		"git":     d.checkGit,
	}

	checkFn, exists := checks[name]
	if !exists {
		return nil, fmt.Errorf("unknown check: %s", name)
	}

	result := checkFn()
	return &result, nil
}

// checkVersion checks the current version.
func (d *DoctorService) checkVersion() CheckResult {
	start := time.Now()

	result := CheckResult{
		Name:   "Version",
		Status: CheckStatusPass,
	}

	if d.version == "" {
		result.Status = CheckStatusWarn
		result.Message = "Version not set"
	} else {
		result.Message = fmt.Sprintf("claude-flow-go %s", d.version)
	}

	result.Duration = time.Since(start)
	return result
}

// checkGoRuntime checks the Go runtime version.
func (d *DoctorService) checkGoRuntime() CheckResult {
	start := time.Now()

	result := CheckResult{
		Name: "Go Runtime",
	}

	goVersion := runtime.Version()
	result.Message = goVersion

	// Extract version number (e.g., "go1.21.0" -> "1.21")
	versionStr := strings.TrimPrefix(goVersion, "go")
	parts := strings.Split(versionStr, ".")
	if len(parts) >= 2 {
		major := parts[0]
		minor := parts[1]
		if major == "1" {
			minorNum := 0
			fmt.Sscanf(minor, "%d", &minorNum)
			if minorNum >= 21 {
				result.Status = CheckStatusPass
			} else if minorNum >= 18 {
				result.Status = CheckStatusWarn
				result.Message = fmt.Sprintf("%s (Go 1.21+ recommended)", goVersion)
			} else {
				result.Status = CheckStatusFail
				result.Message = fmt.Sprintf("%s (Go 1.18+ required)", goVersion)
				result.Fix = "Install Go 1.21 or later from https://go.dev/dl/"
			}
		}
	}

	if result.Status == "" {
		result.Status = CheckStatusPass
	}

	result.Duration = time.Since(start)
	return result
}

// checkConfig checks the configuration file.
func (d *DoctorService) checkConfig() CheckResult {
	start := time.Now()

	result := CheckResult{
		Name: "Configuration",
	}

	configPath := filepath.Join(d.basePath, "config.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		result.Status = CheckStatusWarn
		result.Message = "No config file found"
		result.Fix = fmt.Sprintf("Create %s with your configuration", configPath)
	} else {
		// Try to parse the config
		data, err := os.ReadFile(configPath)
		if err != nil {
			result.Status = CheckStatusFail
			result.Message = fmt.Sprintf("Cannot read config: %v", err)
		} else {
			var config map[string]interface{}
			if err := json.Unmarshal(data, &config); err != nil {
				result.Status = CheckStatusFail
				result.Message = fmt.Sprintf("Invalid JSON: %v", err)
				result.Fix = "Fix the JSON syntax in your config file"
			} else {
				result.Status = CheckStatusPass
				result.Message = "Valid configuration"
			}
		}
	}

	result.Duration = time.Since(start)
	return result
}

// checkDaemon checks if the daemon is running.
func (d *DoctorService) checkDaemon() CheckResult {
	start := time.Now()

	result := CheckResult{
		Name: "Daemon",
	}

	pidPath := filepath.Join(d.basePath, "daemon.pid")

	pidData, err := os.ReadFile(pidPath)
	if os.IsNotExist(err) {
		result.Status = CheckStatusWarn
		result.Message = "Daemon not running"
		result.Fix = "Run 'claude-flow daemon start' to start the daemon"
	} else if err != nil {
		result.Status = CheckStatusFail
		result.Message = fmt.Sprintf("Cannot read PID file: %v", err)
	} else {
		var pid int
		fmt.Sscanf(string(pidData), "%d", &pid)

		// Check if process is running
		process, err := os.FindProcess(pid)
		if err != nil {
			result.Status = CheckStatusWarn
			result.Message = "Daemon PID file exists but process not found"
			result.Fix = "Run 'claude-flow daemon start' to restart the daemon"
		} else {
			// Try to signal the process (signal 0 just checks if it exists)
			err = process.Signal(syscall.Signal(0))
			if err != nil {
				result.Status = CheckStatusWarn
				result.Message = fmt.Sprintf("Daemon process %d not responding", pid)
				result.Fix = "Run 'claude-flow daemon restart' to restart the daemon"
			} else {
				result.Status = CheckStatusPass
				result.Message = fmt.Sprintf("Running (PID %d)", pid)
			}
		}
	}

	result.Duration = time.Since(start)
	return result
}

// checkMemoryDB checks the memory database.
func (d *DoctorService) checkMemoryDB() CheckResult {
	start := time.Now()

	result := CheckResult{
		Name: "Memory Database",
	}

	dbPath := filepath.Join(d.basePath, "memory.db")

	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		result.Status = CheckStatusWarn
		result.Message = "No memory database found"
		result.Fix = "Memory database will be created on first use"
	} else if err != nil {
		result.Status = CheckStatusFail
		result.Message = fmt.Sprintf("Cannot access database: %v", err)
	} else {
		size := info.Size()
		sizeStr := formatBytes(size)
		result.Status = CheckStatusPass
		result.Message = fmt.Sprintf("Found (%s)", sizeStr)
	}

	result.Duration = time.Since(start)
	return result
}

// checkDiskSpace checks available disk space.
func (d *DoctorService) checkDiskSpace() CheckResult {
	start := time.Now()

	result := CheckResult{
		Name: "Disk Space",
	}

	// Get disk usage for the base path
	var stat syscall.Statfs_t
	if err := syscall.Statfs(d.basePath, &stat); err != nil {
		// Fallback to home directory
		home, _ := os.UserHomeDir()
		if err := syscall.Statfs(home, &stat); err != nil {
			result.Status = CheckStatusWarn
			result.Message = "Cannot determine disk space"
			result.Duration = time.Since(start)
			return result
		}
	}

	// Calculate available space in bytes
	available := stat.Bavail * uint64(stat.Bsize)
	total := stat.Blocks * uint64(stat.Bsize)
	usedPercent := float64(total-available) / float64(total) * 100

	availableStr := formatBytes(int64(available))

	if available < 1024*1024*100 { // Less than 100MB
		result.Status = CheckStatusFail
		result.Message = fmt.Sprintf("Low disk space: %s available", availableStr)
		result.Fix = "Free up disk space"
	} else if available < 1024*1024*1024 { // Less than 1GB
		result.Status = CheckStatusWarn
		result.Message = fmt.Sprintf("%s available (%.1f%% used)", availableStr, usedPercent)
	} else {
		result.Status = CheckStatusPass
		result.Message = fmt.Sprintf("%s available (%.1f%% used)", availableStr, usedPercent)
	}

	result.Duration = time.Since(start)
	return result
}

// checkEnvVars checks required environment variables.
func (d *DoctorService) checkEnvVars() CheckResult {
	start := time.Now()

	result := CheckResult{
		Name: "API Keys",
	}

	apiKeys := []string{
		"ANTHROPIC_API_KEY",
		"CLAUDE_API_KEY",
		"OPENAI_API_KEY",
	}

	found := make([]string, 0)
	for _, key := range apiKeys {
		if val := os.Getenv(key); val != "" {
			found = append(found, key)
		}
	}

	if len(found) == 0 {
		result.Status = CheckStatusWarn
		result.Message = "No API keys configured"
		result.Fix = "Set ANTHROPIC_API_KEY or CLAUDE_API_KEY environment variable"
	} else {
		result.Status = CheckStatusPass
		result.Message = fmt.Sprintf("Found: %s", strings.Join(found, ", "))
	}

	result.Duration = time.Since(start)
	return result
}

// checkGit checks git installation and repository status.
func (d *DoctorService) checkGit() CheckResult {
	start := time.Now()

	result := CheckResult{
		Name: "Git",
	}

	// Check if git is installed
	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()
	if err != nil {
		result.Status = CheckStatusWarn
		result.Message = "Git not installed"
		result.Fix = "Install git from https://git-scm.com/"
		result.Duration = time.Since(start)
		return result
	}

	version := strings.TrimSpace(string(output))
	result.Status = CheckStatusPass
	result.Message = version

	result.Duration = time.Since(start)
	return result
}

// GetAvailableChecks returns the list of available check names.
func (d *DoctorService) GetAvailableChecks() []string {
	return []string{"version", "go", "config", "daemon", "memory", "disk", "env", "git"}
}

// formatBytes formats bytes as human-readable string.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatReport formats a diagnostic report for display.
func FormatReport(report *DiagnosticReport, showFix bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Claude Flow Diagnostics (v%s)\n", report.Version))
	sb.WriteString(fmt.Sprintf("Platform: %s\n", report.Platform))
	sb.WriteString(fmt.Sprintf("Time: %s\n\n", report.Timestamp.Format(time.RFC3339)))

	for _, check := range report.Checks {
		icon := getStatusIcon(check.Status)
		sb.WriteString(fmt.Sprintf("%s %s: %s", icon, check.Name, check.Message))
		if check.Duration > 0 {
			sb.WriteString(fmt.Sprintf(" (%.0fms)", float64(check.Duration.Microseconds())/1000))
		}
		sb.WriteString("\n")

		if showFix && check.Fix != "" && check.Status != CheckStatusPass {
			sb.WriteString(fmt.Sprintf("  Fix: %s\n", check.Fix))
		}
	}

	sb.WriteString(fmt.Sprintf("\nSummary: %d passed, %d warnings, %d failed\n",
		report.Summary.Passed, report.Summary.Warnings, report.Summary.Failed))

	return sb.String()
}

// getStatusIcon returns the icon for a check status.
func getStatusIcon(status CheckStatus) string {
	switch status {
	case CheckStatusPass:
		return "[OK]"
	case CheckStatusWarn:
		return "[WARN]"
	case CheckStatusFail:
		return "[FAIL]"
	default:
		return "[?]"
	}
}
