package web

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
)

// repoRoot walks up from the test's working directory to the module root
// (the directory holding go.mod), matching the CWD the app runs from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestDocsRender(t *testing.T) {
	root := repoRoot(t)
	md := goldmark.New()
	for _, d := range docs {
		src, err := os.ReadFile(filepath.Join(root, d.File))
		if err != nil {
			t.Fatalf("doc %q file %q: %v", d.Slug, d.File, err)
		}
		var buf bytes.Buffer
		if err := md.Convert(src, &buf); err != nil {
			t.Fatalf("doc %q render: %v", d.Slug, err)
		}
		html := buf.String()
		if !strings.Contains(html, "<h1") {
			t.Fatalf("doc %q produced no <h1>", d.Slug)
		}
		t.Logf("doc %-12s file=%-28s bytes=%d ok", d.Slug, d.File, len(html))
	}
}
