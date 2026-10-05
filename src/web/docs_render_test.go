package web

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocsRender checks every configured doc exists and renders to HTML.
// Tests run in src/web, so paths are resolved from the module root two levels up.
func TestDocsRender(t *testing.T) {
	md := newMarkdown()
	for _, d := range docs {
		src, err := os.ReadFile(filepath.Join("..", "..", d.File))
		if err != nil {
			t.Fatalf("doc %q: %v", d.Slug, err)
		}
		var buf bytes.Buffer
		if err := md.Convert(src, &buf); err != nil {
			t.Fatalf("doc %q render: %v", d.Slug, err)
		}
		if strings.TrimSpace(buf.String()) == "" {
			t.Fatalf("doc %q rendered empty", d.Slug)
		}
	}
}

// TestDocsTablesRender guards against the table extension being dropped: with
// it enabled, GFM tables become <table> and no raw "|" pipes leak into the HTML.
func TestDocsTablesRender(t *testing.T) {
	md := newMarkdown()
	var buf bytes.Buffer
	src := []byte("| a | b |\n|---|---|\n| 1 | 2 |\n")
	if err := md.Convert(src, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "<td>1</td>") {
		t.Fatalf("table not rendered as HTML:\n%s", html)
	}
	if strings.Contains(html, "|") {
		t.Fatalf("raw pipe leaked into rendered HTML:\n%s", html)
	}
}
