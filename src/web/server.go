// Package web serves the public docs, the admin dashboard (QR login + API
// key management) and the authenticated /notify endpoint.
package web

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/yuin/goldmark"

	"github.com/local/whatsmeow-base/src/app"
)

//go:embed static
var staticFiles embed.FS

type server struct {
	app  *app.App
	auth *auth
	md   goldmark.Markdown
}

// Start runs the HTTP server until ctx is cancelled.
func Start(ctx context.Context, application *app.App) error {
	s := &server{app: application, auth: newAuth(), md: goldmark.New()}

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("GET /", s.handleApp)
	mux.HandleFunc("GET /admin", s.handleApp)
	mux.HandleFunc("GET /docs", s.handleApp)
	mux.HandleFunc("GET /api/docs", s.handleDocsList)
	mux.HandleFunc("GET /api/docs/{slug}", s.handleDoc)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("POST /api/logout-wa", s.auth.require(s.handleLogoutWhatsApp))
	mux.HandleFunc("GET /api/status", s.auth.require(s.handleStatus))
	mux.HandleFunc("GET /api/qr.png", s.auth.require(s.handleQR))
	mux.HandleFunc("GET /api/keys", s.auth.require(s.handleKeysList))
	mux.HandleFunc("POST /api/keys", s.auth.require(s.handleKeyCreate))
	mux.HandleFunc("DELETE /api/keys/{id}", s.auth.require(s.handleKeyDelete))
	mux.HandleFunc("POST /api/send", s.auth.require(s.handleSend))
	mux.HandleFunc("POST /notify", s.handleNotify)

	srv := &http.Server{Addr: Addr(), Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	log.Printf("web: listening on http://%s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Addr returns the bind address, overridable for deployment.
func Addr() string {
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		return v
	}
	return "127.0.0.1:18080"
}

// handleApp serves the single-page app (login-gated). Dashboard and Docs are
// views inside it, switched via the sidebar. /admin and /docs are aliases.
func (s *server) handleApp(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/", "/admin", "/docs":
		s.serveStatic(w, "app.html")
	default:
		http.NotFound(w, r)
	}
}

func (s *server) serveStatic(w http.ResponseWriter, name string) {
	b, err := staticFiles.ReadFile("static/" + name)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
