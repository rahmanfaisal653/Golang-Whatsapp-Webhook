package web

import (
	"bytes"
	"net/http"
	"os"
)

// docs is the single page shown in the docs view, loaded from the repo.
var docs = []struct {
	Slug string
	File string
}{
	{"usage", "docs/usage.md"},
}

func (s *server) handleDocsList(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Slug string `json:"slug"`
	}
	out := make([]item, 0, len(docs))
	for _, d := range docs {
		out = append(out, item{Slug: d.Slug})
	}
	writeJSON(w, out)
}

// docFile returns the file path for a slug, or "" if unknown.
func docFile(slug string) string {
	for _, d := range docs {
		if d.Slug == slug {
			return d.File
		}
	}
	return ""
}

// handleDocRaw returns the raw markdown of a doc, for the "Copy docs" button.
func (s *server) handleDocRaw(w http.ResponseWriter, r *http.Request) {
	file := docFile(r.PathValue("slug"))
	if file == "" {
		http.NotFound(w, r)
		return
	}
	src, err := os.ReadFile(file)
	if err != nil {
		http.Error(w, "doc not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = w.Write(src)
}

func (s *server) handleDoc(w http.ResponseWriter, r *http.Request) {
	file := docFile(r.PathValue("slug"))
	if file == "" {
		http.NotFound(w, r)
		return
	}
	src, err := os.ReadFile(file)
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
}
