package webui_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"naosemeta.com/backend/internal/adapters/webui"
)

func TestFrontendRoutesAndAssets(t *testing.T) {
	files := fstest.MapFS{"index.html": {Data: []byte("<h1>Não se Meta</h1>")}, "assets/app-abcd.js": {Data: []byte("console.log('ok')")}, ".env": {Data: []byte("secret")}}
	h, err := webui.New(files)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path     string
		status   int
		contains string
	}{{"/", 200, "Não se Meta"}, {"/vincular", 200, "Não se Meta"}, {"/assets/app-abcd.js", 200, "console.log"}, {"/assets/missing.js", 404, ""}, {"/.env", 404, ""}, {"/assets/", 404, ""}, {"/v1/unknown", 404, ""}} {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "secret") {
				t.Fatal("exposed hidden file")
			}
			if w.Code == 200 && w.Header().Get("Content-Security-Policy") == "" {
				t.Fatal("missing CSP")
			}
		})
	}
}

func TestRootFilesystemDoesNotExposeEscapingSymlink(t *testing.T) {
	directory := t.TempDir()
	assets := filepath.Join(directory, "assets")
	outside := filepath.Join(t.TempDir(), "private.txt")
	if err := os.Mkdir(assets, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("frontend"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("PRIVATE_TEST_SECRET"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(assets, "leak.js")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	h, err := webui.New(root.FS())
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/assets/leak.js", nil))
	if w.Code != 404 || strings.Contains(w.Body.String(), "PRIVATE_TEST_SECRET") {
		t.Fatal("escaping symlink exposed file")
	}
}
