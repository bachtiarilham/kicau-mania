package delivery

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "kicau.local/edge/"

func isAdapter(pkg string) bool {
	for _, prefix := range []string{"internal/infra/ffmpeg", "internal/infra/mediamtx", "internal/streaming/service", "internal/cloud/service", "internal/infra/storage/", "internal/infra/runtime/", "internal/infra/network/", "internal/cloud/client"} {
		if strings.HasPrefix(pkg, prefix) {
			return true
		}
	}
	return false
}

func domainOf(pkg string) string {
	parts := strings.Split(pkg, "/")
	if len(parts) < 3 || parts[0] != "internal" {
		return ""
	}
	if parts[1] == "cloud" || parts[1] == "camera" || parts[1] == "recording" || parts[1] == "streaming" {
		return parts[1]
	}
	return ""
}

// Contracts and models may be shared; concrete implementations are wired by app.
func dependencyViolation(from, imported string) string {
	if strings.HasPrefix(imported, modulePath+"internal/core/arena") || imported == "modernc.org/sqlite" || imported == "github.com/mattn/go-sqlite3" {
		return "removed local business/SQLite dependency"
	}
	if strings.HasPrefix(imported, modulePath+"pkg/") {
		return "legacy pkg import"
	}
	usecase := strings.Contains(from+"/", "/usecase/")
	if usecase && (imported == "net/http" || strings.HasPrefix(imported, "github.com/gin-gonic/gin") || strings.HasPrefix(imported, "github.com/gorilla/websocket")) {
		return "usecase imports HTTP transport"
	}
	if !strings.HasPrefix(imported, modulePath) {
		return ""
	}
	to := strings.TrimPrefix(imported, modulePath)
	for _, old := range []string{"internal/mediaengine/video", "internal/delivery/rest/video", "internal/mediaengine/audio/pubsub", "internal/mediaengine/audio/metricsclient"} {
		if to == old || strings.HasPrefix(to, old+"/") {
			return "removed video/audio package grouping"
		}
	}
	if to == "internal/edgetocloud" || strings.HasPrefix(to, "internal/edgetocloud/") {
		return "legacy edgetocloud import; use internal/cloud"
	}
	if to == "internal/delivery/http" || strings.HasPrefix(to, "internal/delivery/http/") {
		return "legacy HTTP grouping; use shared delivery or delivery/rest"
	}
	for _, old := range []string{"internal/mediaengine/video/delivery", "internal/mediaengine/saving/delivery", "internal/mediaengine/streaming/delivery", "internal/edgetocloud/synchronization/delivery"} {
		if to == old || strings.HasPrefix(to, old+"/") {
			return "legacy REST handler location"
		}
	}
	if to == "internal/mediaengine/audio/delivery" || strings.HasPrefix(to, "internal/mediaengine/audio/delivery/") {
		return "legacy audio delivery import; use delivery/websocket"
	}
	if from == "internal/delivery/websocket" && strings.Contains(to+"/", "/usecase/") {
		return "protocol delivery depends on concrete usecase instead of an injected interface"
	}
	if to == "internal/edge" || strings.HasPrefix(to, "internal/edge/") {
		return "legacy edge grouping import"
	}
	for _, old := range []string{"arena", "video", "media", "synchronization", "exchange", "operation", "network", "storage", "runtime", "cloudmedia", "mediaengine", "setup"} {
		if to == "internal/"+old || strings.HasPrefix(to, "internal/"+old+"/") {
			return "legacy ungrouped internal import"
		}
	}
	if to == "internal/app" || strings.HasPrefix(to, "internal/app/") || to == "internal/server" || strings.HasPrefix(to, "internal/server/") {
		if from != "internal/app" && from != "internal/server" && !strings.HasPrefix(from, "cmd/") {
			return "dependency points back to application wiring"
		}
	}
	if usecase && to != "internal/apperror" && to != "internal/infra/logger" && to != "internal/core/identifier" && to != "internal/infra/media" && !strings.HasSuffix(to, "/model") && !strings.HasSuffix(to, "/contract") {
		return "usecase imports implementation instead of contract/model"
	}
	if from == "config" && to != "internal/core/identifier" {
		return "configuration depends on application implementation"
	}
	if isAdapter(from) && (strings.Contains(to+"/", "/repository/") || strings.Contains(to+"/", "/usecase/") || strings.Contains(to+"/", "/delivery/") || strings.Contains(to+"/", "/worker/") || to == "config") {
		return "adapter depends on business implementation, transport, or global config"
	}
	if strings.Contains(from+"/", "/repository/") && (strings.Contains(to+"/", "/repository/") || strings.Contains(to+"/", "/usecase/") || strings.Contains(to+"/", "/delivery/") || strings.Contains(to+"/", "/worker/")) {
		return "repository depends on another repository or upper layer"
	}
	if strings.Contains(from+"/", "/worker/") && (strings.Contains(to+"/", "/repository/") || strings.Contains(to+"/", "/usecase/") || strings.Contains(to+"/", "/delivery/") || isAdapter(to)) && to != "internal/infra/runtime/jobgroup" {
		return "worker depends on concrete business/media implementation"
	}
	if strings.Contains(from+"/", "/delivery/") && (strings.Contains(to+"/", "/repository/") || strings.Contains(to+"/", "/worker/") || isAdapter(to)) {
		return "delivery imports repository, worker, or adapter"
	}
	if fromDomain, toDomain := domainOf(from), domainOf(to); fromDomain != "" && toDomain != "" && fromDomain != toDomain {
		if (!usecase && !isAdapter(from)) || (!strings.HasSuffix(to, "/model") && !strings.HasSuffix(to, "/contract")) {
			return "cross-domain access outside usecase contracts/models"
		}
	}
	if from == "internal/infra/media" || from == "internal/apperror" || from == "internal/core/identifier" || strings.HasSuffix(from, "/model") {
		return "shared contract/model depends on implementation"
	}
	return ""
}

func TestDomainDependencyBoundaries(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	for _, old := range []string{"internal/edgetocloud", "internal/delivery/http", "internal/mediaengine/audio/delivery", "internal/mediaengine/video", "internal/delivery/rest/video", "internal/mediaengine/audio/pubsub", "internal/mediaengine/audio/metricsclient", "internal/mediaengine/saving/delivery", "internal/mediaengine/streaming/delivery", "internal/mediaengine/channel", "internal/mediaengine/zoom", "internal/delivery/rest/channel", "internal/delivery/rest/zoom", "internal/mediaengine/saving", "internal/mediaengine/session", "internal/delivery/rest/saving", "internal/delivery/rest/session", "internal/mediaengine/streaming", "internal/mediaengine/audio", "internal/delivery/rest/streaming", "internal/delivery/websocket", "internal/cloudmedia", "internal/mediaengine", "internal/setup", "internal/infra/network/onvif"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(old))); !os.IsNotExist(err) {
			t.Fatalf("legacy protocol delivery location still exists: %s", old)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "pkg")); !os.IsNotExist(err) {
		t.Fatal("legacy pkg directory still exists")
	}
	for _, dir := range []string{"config", "internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			from := filepath.ToSlash(rel)
			testFile := strings.HasSuffix(path, "_test.go")
			if !testFile && strings.Contains(from+"/", "/usecase/") {
				ast.Inspect(parsed, func(node ast.Node) bool {
					if _, ok := node.(*ast.GoStmt); ok {
						t.Errorf("usecase delegates goroutines to workers: %s", path)
					}
					return true
				})
			}
			for _, imp := range parsed.Imports {
				name, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				// Tests may assemble concrete fixtures, but cannot revive pkg.
				if testFile && !strings.HasPrefix(name, modulePath+"pkg/") {
					continue
				}
				if reason := dependencyViolation(from, name); reason != "" {
					t.Errorf("%s: %s -> %s", reason, path, name)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDependencyRules(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		allowed  bool
	}{
		{"internal/streaming/usecase", "internal/camera/model", true},
		{"internal/recording/usecase", "internal/infra/media", true},
		{"internal/app", "internal/camera/repository", true},
		{"internal/app", "internal/infra/ffmpeg", true},
		{"internal/infra/ffmpeg", "internal/infra/runtime/subprocess", true},
		{"internal/streaming/worker", "internal/infra/runtime/jobgroup", true},
		{"internal/recording/repository", "internal/infra/storage/filesystem", true},
		{"config", "internal/core/identifier", true},
		{"internal/streaming/usecase", "internal/streaming/service", false},
		{"internal/streaming/usecase", "internal/infra/ffmpeg", false},
		{"internal/recording/usecase", "internal/infra/runtime/subprocess", false},
		{"internal/camera/usecase", "internal/camera/repository", false},
		{"internal/camera/repository", "internal/recording/repository", false},
		{"internal/infra/mediamtx", "internal/camera/repository", false},
		{"internal/streaming/service", "internal/streaming/usecase", false},
		{"internal/streaming/service", "config", false},
		{"internal/app", "internal/streaming/service", true},
		{"internal/app", "internal/mediaengine/video/usecase", false},
		{"internal/app", "internal/mediaengine/audio/pubsub", false},
		{"internal/app", "internal/mediaengine/audio/metricsclient", false},
		{"internal/delivery/websocket", "internal/app", false},
		{"internal/app", "internal/mediaengine/audio/delivery", false},
		{"internal/app", "internal/delivery/http", false},
		{"internal/app", "internal/mediaengine/video/delivery", false},
		{"internal/delivery", "internal/streaming/delivery", true},
		{"internal/cloud/usecase", "internal/cloud/client", false},
		{"internal/cloud/usecase", "internal/cloud/model", true},
		{"internal/cloud/usecase", "internal/recording/model", true},
		{"internal/cloud/client", "internal/cloud/usecase", false},
		{"internal/app", "internal/edgetocloud/synchronization/usecase", false},
		{"internal/delivery/websocket", "internal/streaming/usecase", false},
		{"internal/delivery", "internal/infra/storage/s3", false},
		{"internal/streaming/worker", "internal/mediaengine/video/model", false},
		{"internal/infra/media", "internal/infra/runtime/subprocess", false},
		{"internal/streaming/usecase", "pkg/nvr", false},
		{"internal/app", "internal/core/arena/repository", false},
	} {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			reason := dependencyViolation(tc.from, modulePath+tc.to)
			if (reason == "") != tc.allowed {
				t.Fatalf("allowed=%v, violation=%q", tc.allowed, reason)
			}
		})
	}
	if dependencyViolation("internal/streaming/usecase", "net/http") == "" {
		t.Fatal("usecase can import HTTP")
	}
}
