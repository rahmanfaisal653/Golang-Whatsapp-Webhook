package web

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
)

// TestDocsRender checks every configured doc exists and renders to HTML.
// Tests run in src/web, so paths are resolved from the module root two levels up.
func TestDocsRender(t *testing.T) {
	md := goldmark.New()
	for _, d := range docs {
		src, err := os.ReadFile(filepath.Join("..", "..", d.File))
		if err != nil {
			t.Fatalf("doc %q: %v", d.Slug, err)
		}
		var buf bytes.Buffer
		if err := md.Convert(src, &buf); err != nil {
			t.Fatalf("doc %q render: %v", d.Slug, err)
		}
		if !strings.Contains(buf.String(), "<h1") {
			t.Fatalf("doc %q produced no <h1>", d.Slug)
		}
	}
}
