package web

import (
	"bytes"
	"net/http"
	"os"
)

// docs maps the pages shown in the public docs UI to files in the repo.
var docs = []struct {
	Slug  string
	Title string
	File  string
}{
	{"overview", "Overview", "docs/getting-started.md"},
	{"integration", "Integration Guide", "docs/integration.md"},
	{"notifier", "API Reference", "docs/notifier.md"},
}

func (s *server) handleDocsList(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
	}
	out := make([]item, 0, len(docs))
	for _, d := range docs {
		out = append(out, item{Slug: d.Slug, Title: d.Title})
	}
	writeJSON(w, out)
}

func (s *server) handleDoc(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	for _, d := range docs {
		if d.Slug != slug {
			continue
		}
		src, err := os.ReadFile(d.File)
		if err != nil {
			http.Error(w, "doc not found", http.StatusNotFound)
			return
		}
		var buf bytes.Buffer
		if err := s.md.Convert(src, &buf); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(buf.Bytes())
		return
	}
	http.NotFound(w, r)
}
