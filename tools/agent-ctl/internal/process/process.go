package process

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"kicau.tools/agent-ctl/internal/config"
)

type ProcessRecord struct {
	Target    string    `json:"target"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Port      int       `json:"port"`
	LogPath   string    `json:"log_path"`
}

type State struct {
	Processes map[string]ProcessRecord `json:"processes"`
}

type Manager struct {
	cfg       *config.Config
	stateFile string
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:       cfg,
		stateFile: filepath.Join(cfg.StateDir, "state.json"),
	}
}

func (m *Manager) LoadState() (*State, error) {
	s := &State{Processes: make(map[string]ProcessRecord)}
	b, err := os.ReadFile(m.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	_ = json.Unmarshal(b, s)
	return s, nil
}

func (m *Manager) SaveState(s *State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.stateFile, b, 0644)
}

func (m *Manager) IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds; we verify via tasklist command or Signal 0 equivalent
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	out, err := cmd.Output()
	if err != nil {
		// Fallback to proc.Signal(nil)
		_ = proc
		return false
	}
	return len(out) > 0 && !containsString(string(out), "No tasks are running")
}

func (m *Manager) Start(targetName string, timeout time.Duration) (*ProcessRecord, error) {
	target, ok := m.cfg.Targets[targetName]
	if !ok {
		return nil, fmt.Errorf("unknown target: %s", targetName)
	}

	state, err := m.LoadState()
	if err != nil {
		return nil, err
	}

	if existing, found := state.Processes[targetName]; found {
		if m.IsRunning(existing.PID) {
			return &existing, fmt.Errorf("%s is already running with PID %d on port %d", target.DisplayName, existing.PID, target.Port)
		}
	}

	logDir := filepath.Dir(target.LogPath)
	_ = os.MkdirAll(logDir, 0755)
	logFile, err := os.OpenFile(target.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", target.LogPath, err)
	}

	if len(target.RunCmd) == 0 {
		return nil, fmt.Errorf("no run command configured for %s", targetName)
	}

	cmd := exec.Command(target.RunCmd[0], target.RunCmd[1:]...)
	cmd.Dir = target.Dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("failed to start %s: %w", targetName, err)
	}
	_ = logFile.Close()

	rec := ProcessRecord{
		Target:    targetName,
		PID:       cmd.Process.Pid,
		StartedAt: time.Now(),
		Port:      target.Port,
		LogPath:   target.LogPath,
	}

	state.Processes[targetName] = rec
	_ = m.SaveState(state)

	// Wait for health check with polling
	if target.HealthURL != "" {
		ready := m.WaitForHealth(target.HealthURL, timeout)
		if !ready {
			return &rec, fmt.Errorf("%s started (PID %d) but health check at %s did not pass within %v (inspect logs at %s)", target.DisplayName, rec.PID, target.HealthURL, timeout, target.LogPath)
		}
	}

	return &rec, nil
}

func (m *Manager) WaitForHealth(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return true
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func (m *Manager) Stop(targetName string) error {
	state, err := m.LoadState()
	if err != nil {
		return err
	}

	rec, ok := state.Processes[targetName]
	if !ok || !m.IsRunning(rec.PID) {
		delete(state.Processes, targetName)
		_ = m.SaveState(state)
		return fmt.Errorf("%s is not running", targetName)
	}

	// Kill entire process tree cleanly on Windows
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(rec.PID), "/T", "/F")
	_ = cmd.Run()

	delete(state.Processes, targetName)
	return m.SaveState(state)
}

func (m *Manager) Status(targetName string) (map[string]any, error) {
	target, ok := m.cfg.Targets[targetName]
	if !ok {
		return nil, fmt.Errorf("unknown target: %s", targetName)
	}

	state, _ := m.LoadState()
	rec, hasState := state.Processes[targetName]

	info := map[string]any{
		"target":       targetName,
		"display_name": target.DisplayName,
		"configured":   true,
		"dir":          target.Dir,
		"port":         target.Port,
		"health_url":   target.HealthURL,
	}

	if hasState && m.IsRunning(rec.PID) {
		info["running"] = true
		info["pid"] = rec.PID
		info["started_at"] = rec.StartedAt.Format(time.RFC3339)
		info["uptime_sec"] = int(time.Since(rec.StartedAt).Seconds())
	} else {
		// Check if port is already listening from an external launch
		if isPortOpen(target.Port) {
			info["running"] = true
			info["pid"] = "external"
			info["port_listening"] = true
		} else {
			info["running"] = false
		}
	}

	// Probe health endpoint
	client := &http.Client{Timeout: 1 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, target.HealthURL, nil)
	resp, err := client.Do(req)
	if err == nil {
		info["healthy"] = resp.StatusCode < 400
		info["http_status"] = resp.StatusCode
		_ = resp.Body.Close()
	} else {
		info["healthy"] = false
		info["health_error"] = err.Error()
	}

	return info, nil
}

func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true
	}
	return false
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || hasSubstr(s, substr))
}

func hasSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
