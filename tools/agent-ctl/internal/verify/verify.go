package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"kicau.tools/agent-ctl/internal/config"
)

type TestResult struct {
	Passed   bool   `json:"passed"`
	Duration string `json:"duration"`
	Output   string `json:"output"`
}

type SmokeResult struct {
	Endpoint   string `json:"endpoint"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
	Passed     bool   `json:"passed"`
	Sample     string `json:"sample"`
}

type EvidenceReport struct {
	Timestamp   string                 `json:"timestamp"`
	Target      string                 `json:"target"`
	GitCommit   string                 `json:"git_commit"`
	GitClean    bool                   `json:"git_clean"`
	Tests       *TestResult            `json:"tests,omitempty"`
	SmokeTests  []SmokeResult          `json:"smoke_tests,omitempty"`
	OverallPass bool                   `json:"overall_pass"`
	Summary     string                 `json:"summary"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type Verifier struct {
	client *http.Client
}

func NewVerifier() *Verifier {
	return &Verifier{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (v *Verifier) RunTests(target config.TargetInfo) (*TestResult, error) {
	if len(target.TestCmd) == 0 {
		return nil, fmt.Errorf("no test command configured for %s", target.Name)
	}

	start := time.Now()
	cmd := exec.Command(target.TestCmd[0], target.TestCmd[1:]...)
	cmd.Dir = target.Dir

	out, err := cmd.CombinedOutput()
	duration := time.Since(start)

	res := &TestResult{
		Passed:   err == nil,
		Duration: duration.Round(time.Millisecond).String(),
		Output:   string(out),
	}

	return res, nil
}

func (v *Verifier) RunSmoke(target config.TargetInfo) []SmokeResult {
	var results []SmokeResult

	endpoints := []string{target.HealthURL}
	if target.ReadyURL != "" && target.ReadyURL != target.HealthURL {
		endpoints = append(endpoints, target.ReadyURL)
	}

	for _, ep := range endpoints {
		if ep == "" {
			continue
		}
		start := time.Now()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ep, nil)
		resp, err := v.client.Do(req)
		latency := time.Since(start).Milliseconds()

		if err != nil {
			results = append(results, SmokeResult{
				Endpoint:   ep,
				StatusCode: 0,
				LatencyMs:  latency,
				Passed:     false,
				Sample:     err.Error(),
			})
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		sample := string(body)
		if len(sample) > 200 {
			sample = sample[:200] + "..."
		}

		results = append(results, SmokeResult{
			Endpoint:   ep,
			StatusCode: resp.StatusCode,
			LatencyMs:  latency,
			Passed:     resp.StatusCode >= 200 && resp.StatusCode < 400,
			Sample:     sample,
		})
	}

	return results
}

func (v *Verifier) CollectEvidence(target config.TargetInfo, runUnitTests, runSmokeTests bool) (*EvidenceReport, error) {
	rep := &EvidenceReport{
		Timestamp:   time.Now().Format(time.RFC3339),
		Target:      target.Name,
		OverallPass: true,
		Metadata:    make(map[string]interface{}),
	}

	// Git inspection
	cmd := exec.Command("git", "-C", target.Dir, "rev-parse", "--short", "HEAD")
	if out, err := cmd.Output(); err == nil {
		rep.GitCommit = strings.TrimSpace(string(out))
	}
	cmd = exec.Command("git", "-C", target.Dir, "status", "--porcelain")
	if out, err := cmd.Output(); err == nil {
		rep.GitClean = len(strings.TrimSpace(string(out))) == 0
	}

	// 1. Unit & Architecture Tests
	if runUnitTests {
		tr, err := v.RunTests(target)
		if err != nil {
			rep.OverallPass = false
			rep.Metadata["test_error"] = err.Error()
		} else {
			rep.Tests = tr
			if !tr.Passed {
				rep.OverallPass = false
			}
		}
	}

	// 2. HTTP Smoke Tests
	if runSmokeTests {
		smoke := v.RunSmoke(target)
		rep.SmokeTests = smoke
		for _, s := range smoke {
			if !s.Passed {
				rep.OverallPass = false
				break
			}
		}
	}

	if rep.OverallPass {
		rep.Summary = fmt.Sprintf("All verification checks PASSED for %s at commit %s", target.DisplayName, rep.GitCommit)
	} else {
		rep.Summary = fmt.Sprintf("Verification FAILED for %s (check test output and smoke endpoints)", target.DisplayName)
	}

	return rep, nil
}

func (r *EvidenceReport) ToMarkdown() string {
	var sb strings.Builder
	statusEmoji := "PASS"
	if !r.OverallPass {
		statusEmoji = "FAIL"
	}

	sb.WriteString(fmt.Sprintf("## Verification Evidence Report: [%s] %s\n\n", statusEmoji, r.Target))
	sb.WriteString(fmt.Sprintf("- **Timestamp:** `%s`\n", r.Timestamp))
	sb.WriteString(fmt.Sprintf("- **Git Commit:** `%s` (Clean: `%t`)\n", r.GitCommit, r.GitClean))
	sb.WriteString(fmt.Sprintf("- **Verdict:** **%s**\n\n", r.Summary))

	if r.Tests != nil {
		sb.WriteString("### Automated Test Suite\n")
		testStatus := "PASSED"
		if !r.Tests.Passed {
			testStatus = "FAILED"
		}
		sb.WriteString(fmt.Sprintf("- **Result:** `%s` (Duration: %s)\n", testStatus, r.Tests.Duration))
		sb.WriteString("```text\n")
		// Show tail of test output
		lines := strings.Split(strings.TrimSpace(r.Tests.Output), "\n")
		if len(lines) > 20 {
			lines = lines[len(lines)-20:]
		}
		sb.WriteString(strings.Join(lines, "\n"))
		sb.WriteString("\n```\n\n")
	}

	if len(r.SmokeTests) > 0 {
		sb.WriteString("### HTTP Smoke Checks\n\n")
		sb.WriteString("| Endpoint | Status | Latency | Sample |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- |\n")
		for _, s := range r.SmokeTests {
			resStr := fmt.Sprintf("%d OK", s.StatusCode)
			if !s.Passed {
				resStr = fmt.Sprintf("%d FAIL", s.StatusCode)
			}
			cleanSample := strings.ReplaceAll(s.Sample, "\n", " ")
			sb.WriteString(fmt.Sprintf("| `%s` | `%s` | `%d ms` | `%s` |\n", s.Endpoint, resStr, s.LatencyMs, cleanSample))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (r *EvidenceReport) ToJSON() (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return "", err
	}
	return buf.String(), nil
}
