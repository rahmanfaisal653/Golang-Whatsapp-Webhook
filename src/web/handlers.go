package web

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"time"

	"rsc.io/qr"

	"github.com/local/whatsmeow-base/src/notify"
)

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !s.auth.checkCredentials(body.Email, body.Password) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.auth.newSession(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.Header.Get("X-Forwarded-Proto") == "https",
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.dropSession(r)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"state":  s.app.State(),
		"number": s.app.Number(),
		"uptime": s.app.Uptime().Round(time.Second).String(),
		"qr":     s.app.QR() != "",
	})
}

func (s *server) handleQR(w http.ResponseWriter, r *http.Request) {
	code := s.app.QR()
	if code == "" {
		http.Error(w, "no qr", http.StatusNotFound)
		return
	}
	c, err := qr.Encode(code, qr.L)
	if err != nil {
		http.Error(w, "encode qr", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_ = png.Encode(w, renderQR(c, 8, 4))
}

// renderQR rasterises a QR code at the given module scale with a quiet zone.
// rsc.io/qr's own Image() ignores Scale and only paints one module-sized block
// in the corner, so we draw it ourselves to get a scannable image.
func renderQR(c *qr.Code, scale, quiet int) image.Image {
	dim := (c.Size + 2*quiet) * scale
	img := image.NewGray(image.Rect(0, 0, dim, dim))
	for y := range dim {
		for x := range dim {
			shade := uint8(0xFF)
			if c.Black(x/scale-quiet, y/scale-quiet) {
				shade = 0x00
			}
			img.SetGray(x, y, color.Gray{Y: shade})
		}
	}
	return img
}

func (s *server) handleKeysList(w http.ResponseWriter, r *http.Request) {
	keys, err := loadKeys()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type item struct {
		ID      string `json:"id"`
		Label   string `json:"label"`
		Created string `json:"created"`
	}
	out := make([]item, 0, len(keys))
	for _, k := range keys {
		out = append(out, item{ID: k.ID, Label: k.Label, Created: k.Created.Format(time.RFC3339)})
	}
	writeJSON(w, out)
}

func (s *server) handleKeyCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body)
	plain, k, err := createKey(strings.TrimSpace(body.Label))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"id": k.ID, "label": k.Label, "key": plain})
}

func (s *server) handleKeyDelete(w http.ResponseWriter, r *http.Request) {
	if err := deleteKey(r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleSend(w http.ResponseWriter, r *http.Request) {
	var req notify.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := notify.Send(r.Context(), s.app.Client, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]string{"status": "sent"})
}

func (s *server) handleNotify(w http.ResponseWriter, r *http.Request) {
	if !verifyKey(apiKeyFrom(r)) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req notify.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := notify.Send(r.Context(), s.app.Client, req); err != nil {
		http.Error(w, fmt.Sprintf("send whatsapp: %v", err), http.StatusBadGateway)
		return
	}
	fmt.Fprint(w, "notify applied")
}

// apiKeyFrom reads the caller key from X-API-Key or an Authorization Bearer.
func apiKeyFrom(r *http.Request) string {
	if k := r.Header.Get("X-API-Key"); k != "" {
		return k
	}
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		return strings.TrimPrefix(a, "Bearer ")
	}
	return ""
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
