package trace

import (
	"context"
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

type Tracer struct {
	client *http.Client
}

func NewTracer() *Tracer {
	return &Tracer{
		client: &http.Client{Timeout: 35 * time.Second},
	}
}

func (t *Tracer) Metrics(target config.TargetInfo, pid int) (map[string]any, error) {
	metrics := map[string]any{
		"target":    target.Name,
		"timestamp": time.Now().Format(time.RFC3339),
		"port":      target.Port,
	}

	// Try probing process stats on Windows if PID is known
	if pid > 0 {
		metrics["pid"] = pid
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
		if out, err := cmd.Output(); err == nil {
			line := strings.TrimSpace(string(out))
			parts := strings.Split(line, "\",\"")
			if len(parts) >= 5 {
				memStr := strings.Trim(parts[4], "\" ")
				metrics["memory_working_set"] = memStr
			}
		}
	}

	// Try scraping Prometheus/Go metrics endpoint if exposed
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("http://localhost:%d/metrics", target.Port), nil)
	resp, err := t.client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		metrics["metrics_available"] = true
		metrics["raw_metrics_sample"] = truncateString(string(body), 500)
	} else {
		metrics["metrics_available"] = false
	}

	return metrics, nil
}

func (t *Tracer) Profile(target config.TargetInfo, profileType string, duration time.Duration, outputDir string) (string, error) {
	if profileType == "" {
		profileType = "heap"
	}
	if duration <= 0 {
		duration = 5 * time.Second
	}

	url := fmt.Sprintf("http://localhost:%d/debug/pprof/%s", target.Port, profileType)
	if profileType == "profile" {
		url += fmt.Sprintf("?seconds=%d", int(duration.Seconds()))
	}

	if outputDir == "" {
		outputDir = filepath.Join(os.TempDir(), "agent-ctl-profiles")
	}
	_ = os.MkdirAll(outputDir, 0755)

	filename := fmt.Sprintf("%s_%s_%d.pprof", target.Name, profileType, time.Now().Unix())
	targetPath := filepath.Join(outputDir, filename)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch pprof from %s: %w (ensure pprof is enabled on target)", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pprof endpoint %s returned HTTP %d", url, resp.StatusCode)
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s (%d bytes saved)", targetPath, n), nil
}

func truncateString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
