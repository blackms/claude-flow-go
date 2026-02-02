// Package utility provides utility services for diagnostics, daemon management, and benchmarking.
package utility

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DaemonStatus represents the current status of the daemon.
type DaemonStatus string

const (
	DaemonStatusRunning DaemonStatus = "running"
	DaemonStatusStopped DaemonStatus = "stopped"
	DaemonStatusUnknown DaemonStatus = "unknown"
)

// DaemonState represents the state of the daemon.
type DaemonState struct {
	PID       int          `json:"pid"`
	Status    DaemonStatus `json:"status"`
	StartTime time.Time    `json:"startTime,omitempty"`
	Uptime    string       `json:"uptime,omitempty"`
	LogFile   string       `json:"logFile"`
	PIDFile   string       `json:"pidFile"`
}

// DaemonService provides daemon management functionality.
type DaemonService struct {
	basePath string
	pidFile  string
	logFile  string
}

// NewDaemonService creates a new daemon service.
func NewDaemonService() (*DaemonService, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	basePath := filepath.Join(home, ".claude-flow")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &DaemonService{
		basePath: basePath,
		pidFile:  filepath.Join(basePath, "daemon.pid"),
		logFile:  filepath.Join(basePath, "daemon.log"),
	}, nil
}

// Start starts the daemon.
func (d *DaemonService) Start(foreground bool, logFile string) error {
	// Check if already running
	if d.IsRunning() {
		state, _ := d.Status()
		return fmt.Errorf("daemon already running (PID %d)", state.PID)
	}

	if logFile != "" {
		d.logFile = logFile
	}

	if foreground {
		return d.runForeground()
	}

	return d.runBackground()
}

// runForeground runs the daemon in the foreground.
func (d *DaemonService) runForeground() error {
	// Write PID file
	if err := d.writePID(os.Getpid()); err != nil {
		return err
	}

	// Open log file
	logFile, err := os.OpenFile(d.logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	// Log startup
	d.log(logFile, "Daemon started in foreground mode (PID %d)", os.Getpid())

	// Run the daemon loop
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	fmt.Println("Daemon running in foreground. Press Ctrl+C to stop.")

	for {
		select {
		case <-ticker.C:
			d.log(logFile, "Daemon heartbeat")
		}
	}
}

// runBackground runs the daemon in the background.
func (d *DaemonService) runBackground() error {
	// Get the executable path
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Prepare the command
	cmd := exec.Command(executable, "daemon", "start", "--foreground")

	// Open log file for output
	logFile, err := os.OpenFile(d.logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Detach the process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Write PID file
	if err := d.writePID(cmd.Process.Pid); err != nil {
		logFile.Close()
		return err
	}

	logFile.Close()

	fmt.Printf("Daemon started (PID %d)\n", cmd.Process.Pid)
	fmt.Printf("Log file: %s\n", d.logFile)

	return nil
}

// Stop stops the daemon.
func (d *DaemonService) Stop() error {
	pid, err := d.readPID()
	if err != nil {
		return fmt.Errorf("daemon not running: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		d.removePID()
		return fmt.Errorf("daemon process not found")
	}

	// Try graceful shutdown with SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Try SIGKILL if SIGTERM fails
		if err := process.Signal(syscall.SIGKILL); err != nil {
			d.removePID()
			return fmt.Errorf("failed to stop daemon: %w", err)
		}
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case <-done:
		// Process exited
	case <-time.After(5 * time.Second):
		// Force kill
		process.Signal(syscall.SIGKILL)
	}

	d.removePID()

	// Log the stop
	if logFile, err := os.OpenFile(d.logFile, os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		d.log(logFile, "Daemon stopped")
		logFile.Close()
	}

	return nil
}

// Restart restarts the daemon.
func (d *DaemonService) Restart() error {
	if d.IsRunning() {
		if err := d.Stop(); err != nil {
			return fmt.Errorf("failed to stop daemon: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	return d.Start(false, "")
}

// Status returns the current daemon status.
func (d *DaemonService) Status() (*DaemonState, error) {
	state := &DaemonState{
		Status:  DaemonStatusStopped,
		LogFile: d.logFile,
		PIDFile: d.pidFile,
	}

	pid, err := d.readPID()
	if err != nil {
		return state, nil
	}

	state.PID = pid

	// Check if process is running
	process, err := os.FindProcess(pid)
	if err != nil {
		return state, nil
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		state.Status = DaemonStatusUnknown
		return state, nil
	}

	state.Status = DaemonStatusRunning

	// Try to get process start time from PID file modification time
	if info, err := os.Stat(d.pidFile); err == nil {
		state.StartTime = info.ModTime()
		uptime := time.Since(state.StartTime)
		state.Uptime = formatDuration(uptime)
	}

	return state, nil
}

// IsRunning returns true if the daemon is running.
func (d *DaemonService) IsRunning() bool {
	state, _ := d.Status()
	return state.Status == DaemonStatusRunning
}

// GetLogs returns recent log entries.
func (d *DaemonService) GetLogs(lines int) ([]string, error) {
	file, err := os.Open(d.logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	// Read all lines (for simplicity; could be optimized for large files)
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return last N lines
	if len(allLines) <= lines {
		return allLines, nil
	}

	return allLines[len(allLines)-lines:], nil
}

// TailLogs tails the log file, writing new lines to the writer.
func (d *DaemonService) TailLogs(w io.Writer, follow bool) error {
	file, err := os.Open(d.logFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Seek to end minus some bytes
	info, _ := file.Stat()
	offset := info.Size() - 4096
	if offset < 0 {
		offset = 0
	}
	file.Seek(offset, 0)

	// Read and print
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fmt.Fprintln(w, scanner.Text())
	}

	if !follow {
		return nil
	}

	// Follow mode - keep reading new content
	for {
		line := make([]byte, 4096)
		n, err := file.Read(line)
		if n > 0 {
			w.Write(line[:n])
		}
		if err == io.EOF {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if err != nil {
			return err
		}
	}
}

// writePID writes the PID to the PID file.
func (d *DaemonService) writePID(pid int) error {
	return os.WriteFile(d.pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// readPID reads the PID from the PID file.
func (d *DaemonService) readPID() (int, error) {
	data, err := os.ReadFile(d.pidFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID file: %w", err)
	}

	return pid, nil
}

// removePID removes the PID file.
func (d *DaemonService) removePID() error {
	return os.Remove(d.pidFile)
}

// log writes a log message to the log file.
func (d *DaemonService) log(w io.Writer, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(w, "[%s] %s\n", timestamp, msg)
}

// formatDuration formats a duration as a human-readable string.
func formatDuration(dur time.Duration) string {
	if dur < time.Minute {
		return fmt.Sprintf("%ds", int(dur.Seconds()))
	}
	if dur < time.Hour {
		return fmt.Sprintf("%dm %ds", int(dur.Minutes()), int(dur.Seconds())%60)
	}
	if dur < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(dur.Hours()), int(dur.Minutes())%60)
	}
	days := int(dur.Hours() / 24)
	hours := int(dur.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// FormatStatus formats daemon status for display.
func FormatStatus(state *DaemonState, verbose bool) string {
	var sb strings.Builder

	sb.WriteString("Daemon Status\n")
	sb.WriteString(strings.Repeat("=", 40) + "\n\n")

	statusIcon := "[STOPPED]"
	if state.Status == DaemonStatusRunning {
		statusIcon = "[RUNNING]"
	} else if state.Status == DaemonStatusUnknown {
		statusIcon = "[UNKNOWN]"
	}

	sb.WriteString(fmt.Sprintf("Status:    %s\n", statusIcon))

	if state.PID > 0 {
		sb.WriteString(fmt.Sprintf("PID:       %d\n", state.PID))
	}

	if !state.StartTime.IsZero() {
		sb.WriteString(fmt.Sprintf("Started:   %s\n", state.StartTime.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("Uptime:    %s\n", state.Uptime))
	}

	if verbose {
		sb.WriteString(fmt.Sprintf("\nPID File:  %s\n", state.PIDFile))
		sb.WriteString(fmt.Sprintf("Log File:  %s\n", state.LogFile))
	}

	return sb.String()
}
