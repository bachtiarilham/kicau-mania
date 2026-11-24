package diagnostic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kicau.tools/agent-ctl/internal/config"
)

type Diagnostic struct {
	client *http.Client
}

func NewDiagnostic() *Diagnostic {
	return &Diagnostic{
		client: &http.Client{Timeout: 3 * time.Second},
	}
}

func (d *Diagnostic) Logs(target config.TargetInfo, tailCount int, errorOnly bool) ([]string, error) {
	if target.LogPath == "" {
		return nil, fmt.Errorf("no log path configured for %s", target.Name)
	}

	file, err := os.Open(target.LogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{fmt.Sprintf("Log file does not exist yet at %s", target.LogPath)}, nil
		}
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		if errorOnly {
			lower := strings.ToLower(text)
			if !strings.Contains(lower, "error") && !strings.Contains(lower, "warn") && !strings.Contains(lower, "panic") {
				continue
			}
		}
		lines = append(lines, text)
	}

	if tailCount > 0 && len(lines) > tailCount {
		lines = lines[len(lines)-tailCount:]
	}

	return lines, scanner.Err()
}

func (d *Diagnostic) Health(target config.TargetInfo) (map[string]any, error) {
	res := map[string]any{
		"target":       target.Name,
		"display_name": target.DisplayName,
		"timestamp":    time.Now().Format(time.RFC3339),
		"endpoints":    map[string]any{},
	}

	endpoints := map[string]string{
		"healthz": target.HealthURL,
	}
	if target.ReadyURL != "" && target.ReadyURL != target.HealthURL {
		endpoints["readyz"] = target.ReadyURL
	}

	epMap := res["endpoints"].(map[string]any)
	allHealthy := true

	for name, url := range endpoints {
		if url == "" {
			continue
		}
		start := time.Now()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		resp, err := d.client.Do(req)
		latency := time.Since(start)

		if err != nil {
			allHealthy = false
			epMap[name] = map[string]any{
				"url":     url,
				"healthy": false,
				"error":   err.Error(),
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		var jsonBody any
		if err := json.Unmarshal(body, &jsonBody); err != nil {
			jsonBody = string(body)
		}

		healthy := resp.StatusCode >= 200 && resp.StatusCode < 400
		if !healthy {
			allHealthy = false
		}

		epMap[name] = map[string]any{
			"url":         url,
			"healthy":     healthy,
			"status_code": resp.StatusCode,
			"latency_ms":  latency.Milliseconds(),
			"body":        jsonBody,
		}
	}

	res["overall_healthy"] = allHealthy
	return res, nil
}

func (d *Diagnostic) Inspect(target config.TargetInfo) (map[string]any, error) {
	info := map[string]any{
		"target":    target.Name,
		"dir":       target.Dir,
		"port":      target.Port,
		"log_path":  target.LogPath,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// Git commit hash & status
	cmd := exec.Command("git", "-C", target.Dir, "rev-parse", "--short", "HEAD")
	if out, err := cmd.Output(); err == nil {
		info["git_commit"] = strings.TrimSpace(string(out))
	}

	cmd = exec.Command("git", "-C", target.Dir, "status", "--porcelain")
	if out, err := cmd.Output(); err == nil {
		clean := len(strings.TrimSpace(string(out))) == 0
		info["git_clean"] = clean
	}

	// Read environment file variables (safely redacted)
	envPath := filepath.Join(target.Dir, ".env")
	if envBytes, err := os.ReadFile(envPath); err == nil {
		redactedEnv := make(map[string]string)
		for _, line := range strings.Split(string(envBytes), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				lowerK := strings.ToLower(k)
				if strings.Contains(lowerK, "pass") || strings.Contains(lowerK, "secret") || strings.Contains(lowerK, "key") || strings.Contains(lowerK, "token") {
					redactedEnv[k] = "[Redacted]"
				} else {
					redactedEnv[k] = v
				}
			}
		}
		info["env_vars"] = redactedEnv
	}

	return info, nil
}
