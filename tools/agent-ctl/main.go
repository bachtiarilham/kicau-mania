package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"kicau.tools/agent-ctl/internal/config"
	"kicau.tools/agent-ctl/internal/diagnostic"
	"kicau.tools/agent-ctl/internal/process"
	"kicau.tools/agent-ctl/internal/trace"
	"kicau.tools/agent-ctl/internal/verify"
)

const version = "1.0.0"

func printHelp() {
	help := `agent-ctl - Reusable Agent Automation & Control CLI v%s

USAGE:
  agent-ctl <command> [target] [flags]

TARGETS:
  cloud       Cloud Backend API (cloud-app, default port 8080)
  edge        Local Edge Media Engine (local-app, default port 8090)
  desktop     Desktop Operator HUD (kicau-mania-desktop, default port 5173)
  all         Run command across all configured targets

COMMANDS:
  start       Start application processes in background with health polling
  stop        Gracefully stop running target processes
  restart     Stop and start target processes
  status      Show current status, PID, port listening, and health
  logs        View target logs with optional tail count and error-only filter
  health      Deep health and readiness probe with HTTP latency measurement
  inspect     Inspect target directory, git commit, and redacted env vars
  trace       Collect runtime performance metrics or grab a pprof profile
  verify      Execute automated tests and smoke tests
  evidence    Generate auditable verification report (JSON or Markdown)

FLAGS:
  --help      Show this help message
  --version   Show version information

EXAMPLES:
  agent-ctl start cloud
  agent-ctl status
  agent-ctl logs cloud --tail 30 --error-only
  agent-ctl health all
  agent-ctl trace metrics cloud
  agent-ctl verify cloud
  agent-ctl evidence all --format markdown
`
	fmt.Printf(help, version)
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	cmd := os.Args[1]
	if cmd == "--help" || cmd == "-h" || cmd == "help" {
		printHelp()
		os.Exit(0)
	}
	if cmd == "--version" || cmd == "-v" || cmd == "version" {
		fmt.Printf("agent-ctl version %s\n", version)
		os.Exit(0)
	}

	cfg, err := config.Detect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting project configuration: %v\n", err)
		os.Exit(1)
	}

	mgr := process.NewManager(cfg)
	diag := diagnostic.NewDiagnostic()
	tracer := trace.NewTracer()
	verifier := verify.NewVerifier()

	target := "all"
	args := os.Args[2:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		target = args[0]
		args = args[1:]
	}

	targets := resolveTargets(cfg, target)
	if len(targets) == 0 {
		fmt.Fprintf(os.Stderr, "Unknown target '%s'. Valid targets: cloud, edge, desktop, all\n", target)
		os.Exit(1)
	}

	switch cmd {
	case "start", "run":
		handleStart(mgr, targets, args)
	case "stop":
		handleStop(mgr, targets)
	case "restart":
		handleStop(mgr, targets)
		time.Sleep(1 * time.Second)
		handleStart(mgr, targets, args)
	case "status":
		handleStatus(mgr, targets)
	case "logs":
		handleLogs(diag, targets, args)
	case "health":
		handleHealth(diag, targets)
	case "inspect":
		handleInspect(diag, targets)
	case "trace":
		handleTrace(tracer, mgr, targets, args)
	case "verify":
		handleVerify(verifier, targets, args)
	case "evidence":
		handleEvidence(verifier, targets, args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command '%s'. Run 'agent-ctl help' for usage.\n", cmd)
		os.Exit(1)
	}
}

func resolveTargets(cfg *config.Config, target string) []config.TargetInfo {
	if target == "all" {
		return []config.TargetInfo{cfg.Targets["cloud"], cfg.Targets["edge"], cfg.Targets["desktop"]}
	}
	if t, ok := cfg.Targets[target]; ok {
		return []config.TargetInfo{t}
	}
	// Support alias 'local' for 'edge'
	if target == "local" {
		return []config.TargetInfo{cfg.Targets["edge"]}
	}
	return nil
}

func handleStart(mgr *process.Manager, targets []config.TargetInfo, args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	timeout := fs.Duration("timeout", 15*time.Second, "Timeout for health check polling")
	_ = fs.Parse(args)

	for _, t := range targets {
		fmt.Printf("Starting %s (%s)...\n", t.DisplayName, t.Name)
		rec, err := mgr.Start(t.Name, *timeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [ERROR] %v\n", err)
			continue
		}
		fmt.Printf("  [OK] %s running with PID %d on port %d\n", t.DisplayName, rec.PID, rec.Port)
	}
}

func handleStop(mgr *process.Manager, targets []config.TargetInfo) {
	for _, t := range targets {
		fmt.Printf("Stopping %s (%s)...\n", t.DisplayName, t.Name)
		err := mgr.Stop(t.Name)
		if err != nil {
			fmt.Printf("  [WARN] %v\n", err)
		} else {
			fmt.Printf("  [OK] %s stopped successfully\n", t.DisplayName)
		}
	}
}

func handleStatus(mgr *process.Manager, targets []config.TargetInfo) {
	fmt.Println("=== Service Status ===")
	for _, t := range targets {
		st, err := mgr.Status(t.Name)
		if err != nil {
			fmt.Printf("- %-10s : Error: %v\n", t.Name, err)
			continue
		}
		running := st["running"].(bool)
		healthy := st["healthy"].(bool)

		runStr := "STOPPED"
		if running {
			runStr = fmt.Sprintf("RUNNING (PID %v)", st["pid"])
		}

		healthStr := "UNHEALTHY"
		if healthy {
			healthStr = fmt.Sprintf("HEALTHY (HTTP %v)", st["http_status"])
		}

		fmt.Printf("- %-10s : %-20s | Port %d | %s\n", t.Name, runStr, t.Port, healthStr)
	}
}

func handleLogs(diag *diagnostic.Diagnostic, targets []config.TargetInfo, args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	tail := fs.Int("tail", 40, "Number of tail lines to display")
	errorOnly := fs.Bool("error-only", false, "Filter only lines containing error/warn/panic")
	_ = fs.Parse(args)

	for _, t := range targets {
		fmt.Printf("=== Logs for %s (%s) ===\n", t.DisplayName, t.LogPath)
		lines, err := diag.Logs(t, *tail, *errorOnly)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading logs: %v\n", err)
			continue
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		fmt.Println()
	}
}

func handleHealth(diag *diagnostic.Diagnostic, targets []config.TargetInfo) {
	fmt.Println("=== Deep Health & Readiness Probe ===")
	for _, t := range targets {
		res, err := diag.Health(t)
		if err != nil {
			fmt.Printf("Error probing %s: %v\n", t.Name, err)
			continue
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
	}
}

func handleInspect(diag *diagnostic.Diagnostic, targets []config.TargetInfo) {
	fmt.Println("=== Target Environment Inspection ===")
	for _, t := range targets {
		res, err := diag.Inspect(t)
		if err != nil {
			fmt.Printf("Error inspecting %s: %v\n", t.Name, err)
			continue
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
	}
}

func handleTrace(tracer *trace.Tracer, mgr *process.Manager, targets []config.TargetInfo, args []string) {
	mode := "metrics"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		mode = args[0]
		args = args[1:]
	}

	fs := flag.NewFlagSet("trace", flag.ExitOnError)
	profType := fs.String("type", "heap", "pprof profile type: heap, profile, goroutine, block")
	duration := fs.Duration("duration", 5*time.Second, "pprof sampling duration")
	_ = fs.Parse(args)

	for _, t := range targets {
		st, _ := mgr.Status(t.Name)
		pid := 0
		if st != nil && st["running"] == true {
			if p, ok := st["pid"].(int); ok {
				pid = p
			}
		}

		if mode == "pprof" {
			fmt.Printf("Collecting pprof (%s) from %s...\n", *profType, t.DisplayName)
			savedPath, err := tracer.Profile(t, *profType, *duration, "")
			if err != nil {
				fmt.Fprintf(os.Stderr, "  [ERROR] %v\n", err)
			} else {
				fmt.Printf("  [OK] Profile saved to: %s\n", savedPath)
			}
		} else {
			// Metrics mode
			m, err := tracer.Metrics(t, pid)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading metrics: %v\n", err)
				continue
			}
			b, _ := json.MarshalIndent(m, "", "  ")
			fmt.Println(string(b))
		}
	}
}

func handleVerify(verifier *verify.Verifier, targets []config.TargetInfo, args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	noTests := fs.Bool("no-tests", false, "Skip automated unit tests")
	noSmoke := fs.Bool("no-smoke", false, "Skip HTTP smoke checks")
	_ = fs.Parse(args)

	allPassed := true
	for _, t := range targets {
		fmt.Printf("Running verification on %s (%s)...\n", t.DisplayName, t.Name)
		rep, err := verifier.CollectEvidence(t, !*noTests, !*noSmoke)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [ERROR] %v\n", err)
			allPassed = false
			continue
		}
		if rep.OverallPass {
			fmt.Printf("  [PASS] %s\n", rep.Summary)
		} else {
			fmt.Printf("  [FAIL] %s\n", rep.Summary)
			allPassed = false
		}
		if rep.Tests != nil && !rep.Tests.Passed {
			fmt.Printf("  Test failures:\n%s\n", rep.Tests.Output)
		}
	}

	if !allPassed {
		os.Exit(1)
	}
}

func handleEvidence(verifier *verify.Verifier, targets []config.TargetInfo, args []string) {
	fs := flag.NewFlagSet("evidence", flag.ExitOnError)
	format := fs.String("format", "markdown", "Output format: markdown, json")
	_ = fs.Parse(args)

	for _, t := range targets {
		rep, err := verifier.CollectEvidence(t, true, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error collecting evidence: %v\n", err)
			continue
		}

		if *format == "json" {
			jsonStr, _ := rep.ToJSON()
			fmt.Println(jsonStr)
		} else {
			fmt.Println(rep.ToMarkdown())
		}
	}
}
