package delivery

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func forbidden(from, dep string) string {
	uc := strings.Contains(from+"/", "/usecase/")
	if uc && (dep == "net/http" || strings.HasPrefix(dep, "github.com/gin-gonic/gin")) {
		return "usecase imports HTTP"
	}
	if !strings.HasPrefix(dep, "kicau.cloud/control/") {
		return ""
	}
	to := strings.TrimPrefix(dep, "kicau.cloud/control/")
	if uc && to != "internal/apperror" && to != "internal/infra/logger" && !strings.HasSuffix(to, "/model") && !strings.HasSuffix(to, "/contract") {
		return "usecase imports concrete dependency"
	}
	if strings.Contains(from+"/", "/repository/") && (strings.Contains(to+"/", "/repository/") || strings.Contains(to+"/", "/usecase/") || strings.Contains(to+"/", "/delivery/")) {
		return "repository imports upper layer/another repository"
	}
	if strings.Contains(from+"/", "/delivery/") && (strings.Contains(to+"/", "/usecase/") || strings.Contains(to+"/", "/repository/")) {
		return "handler imports concrete business/storage"
	}
	if strings.Contains(from+"/", "/service/") && (strings.Contains(to+"/", "/usecase/") || strings.Contains(to+"/", "/repository/") || strings.Contains(to+"/", "/delivery/")) {
		return "adapter imports business implementation"
	}
	if strings.Contains(from+"/", "/worker/") && (strings.Contains(to+"/", "/usecase/") || strings.Contains(to+"/", "/repository/")) {
		return "worker imports concrete business implementation"
	}
	return ""
}
func TestDependencyDirection(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		node, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		dir, _ := filepath.Rel(root, filepath.Dir(path))
		from := filepath.ToSlash(dir)
		if strings.Contains(from+"/", "/usecase/") {
			ast.Inspect(node, func(n ast.Node) bool {
				if _, ok := n.(*ast.GoStmt); ok {
					t.Errorf("goroutine belongs in worker/service: %s", path)
				}
				return true
			})
		}
		for _, i := range node.Imports {
			dep, err := strconv.Unquote(i.Path.Value)
			if err != nil {
				return err
			}
			if problem := forbidden(from, dep); problem != "" {
				t.Errorf("%s: %s -> %s", problem, from, dep)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
