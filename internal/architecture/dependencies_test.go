package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDependenciesPointInward(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			value, _ := strconv.Unquote(imp.Path.Value)
			thirdParty := strings.Contains(strings.Split(value, "/")[0], ".")
			if strings.HasPrefix(path, "../domain/") && thirdParty {
				t.Errorf("domain depends on infrastructure: %s -> %s", path, value)
			}
			if strings.HasPrefix(path, "../application/") && thirdParty && value != "naosemeta.com/backend/internal/domain" {
				t.Errorf("application depends on infrastructure: %s -> %s", path, value)
			}
			if strings.HasPrefix(path, "../adapters/") && strings.HasPrefix(value, "naosemeta.com/backend/internal/adapters/") {
				parts := strings.Split(filepath.ToSlash(path), "/")
				own := "naosemeta.com/backend/internal/adapters/" + parts[2]
				if value != own && !strings.HasPrefix(value, own+"/") {
					t.Errorf("adapter depends on another adapter: %s -> %s", path, value)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
