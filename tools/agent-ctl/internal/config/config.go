package config

import (
	"os"
	"path/filepath"
)

type TargetInfo struct {
	Name        string
	DisplayName string
	Dir         string
	Port        int
	HealthURL   string
	ReadyURL    string
	LogPath     string
	RunCmd      []string
	TestCmd     []string
}

type Config struct {
	RootDir  string
	StateDir string
	Targets  map[string]TargetInfo
}

func Detect() (*Config, error) {
	// Look for project root starting from current directory up
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	root := findRoot(cwd)
	stateDir := filepath.Join(root, ".agent-ctl")
	_ = os.MkdirAll(stateDir, 0755)

	// Desktop directory detection
	desktopDir := os.Getenv("DESKTOP_DIR")
	if desktopDir == "" {
		// Try standard sibling directory: D:\Workdir\wails\kicau-mania-desktop
		cand := filepath.Join(root, "..", "..", "wails", "kicau-mania-desktop")
		if abs, err := filepath.Abs(cand); err == nil && dirExists(abs) {
			desktopDir = abs
		} else if dirExists("D:\\Workdir\\wails\\kicau-mania-desktop") {
			desktopDir = "D:\\Workdir\\wails\\kicau-mania-desktop"
		} else {
			desktopDir = filepath.Join(root, "kicau-mania-desktop")
		}
	}

	cloudDir := filepath.Join(root, "cloud-app")
	edgeDir := filepath.Join(root, "local-app")

	cfg := &Config{
		RootDir:  root,
		StateDir: stateDir,
		Targets: map[string]TargetInfo{
			"cloud": {
				Name:        "cloud",
				DisplayName: "Cloud App API",
				Dir:         cloudDir,
				Port:        8080,
				HealthURL:   "http://localhost:8080/healthz",
				ReadyURL:    "http://localhost:8080/readyz",
				LogPath:     filepath.Join(cloudDir, "logs", "app.log"),
				RunCmd:      []string{"go", "run", "./cmd/api"},
				TestCmd:     []string{"go", "test", "./..."},
			},
			"edge": {
				Name:        "edge",
				DisplayName: "Local Edge Engine",
				Dir:         edgeDir,
				Port:        8081,
				HealthURL:   "http://localhost:8081/healthz",
				ReadyURL:    "http://localhost:8081/readyz",
				LogPath:     filepath.Join(edgeDir, "logs", "app.log"),
				RunCmd:      []string{"go", "run", "./cmd/api"},
				TestCmd:     []string{"go", "test", "./..."},
			},
			"desktop": {
				Name:        "desktop",
				DisplayName: "Desktop Operator HUD",
				Dir:         desktopDir,
				Port:        5173,
				HealthURL:   "http://localhost:5173/",
				ReadyURL:    "http://localhost:5173/",
				LogPath:     filepath.Join(desktopDir, "logs", "desktop.log"),
				RunCmd:      []string{"wails", "dev"},
				TestCmd:     []string{"go", "test", "./..."},
			},
		},
	}

	return cfg, nil
}

func findRoot(dir string) string {
	curr := dir
	for i := 0; i < 6; i++ {
		if fileExists(filepath.Join(curr, "cloud-app", "go.mod")) {
			return curr
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return dir
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
