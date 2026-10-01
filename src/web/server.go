package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/local/whatsmeow-base/src/features"
)

//go:embed ui.html
var uiHTML []byte

type SendRequest struct {
	To      string `json:"to"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

type Server struct {
	addr       string
	manager    *WhatsAppManager
	featureMgr *features.Manager
	httpSrv    *http.Server
}

func StartServer(ctx context.Context, mgr *WhatsAppManager, featMgr *features.Manager, addr string) *Server {
	if addr == "" {
		addr = getListenAddr()
	}

	mux := http.NewServeMux()
	srv := &Server{
		addr:       addr,
		manager:    mgr,
		featureMgr: featMgr,
	}

	// 1. Web UI Dashboard
	mux.HandleFunc("/", srv.handleRoot)

	// 2. Status & QR endpoints
	mux.HandleFunc("/api/status", srv.handleStatus)
	mux.HandleFunc("/api/qr.png", srv.handleQRImage)
	mux.HandleFunc("/api/logout", srv.handleLogout)
	mux.HandleFunc("/api/reconnect", srv.handleReconnect)

	// 3. Messenger & Tools
	mux.HandleFunc("/api/send", srv.handleSend)
	mux.HandleFunc("/api/groups", srv.handleGroups)
	mux.HandleFunc("/api/logs", srv.handleLogs)

	// 4. Backward-compatible /notify Webhook Endpoint
	mux.HandleFunc("/notify", srv.handleNotify)

	// 5. Feature Management Endpoints
	mux.HandleFunc("/api/features", srv.handleFeatures)
	mux.HandleFunc("/api/features/settings", srv.handleFeaturesSettings)
	mux.HandleFunc("/api/features/builtins/toggle", srv.handleFeaturesBuiltinToggle)
	mux.HandleFunc("/api/features/customs", srv.handleFeaturesCustoms)
	mux.HandleFunc("/api/features/customs/toggle", srv.handleFeaturesCustomToggle)
	mux.HandleFunc("/api/features/customs/delete", srv.handleFeaturesCustomDelete)
	mux.HandleFunc("/api/features/customs/update", srv.handleFeaturesCustomUpdate)

	// 6. Webhook Integrations Management Endpoints
	mux.HandleFunc("/api/webhooks", srv.handleWebhooks)
	mux.HandleFunc("/api/webhooks/toggle", srv.handleWebhooksToggle)
	mux.HandleFunc("/api/webhooks/delete", srv.handleWebhooksDelete)
	mux.HandleFunc("/api/webhooks/update", srv.handleWebhooksUpdate)
	mux.HandleFunc("/api/webhooks/test", srv.handleWebhooksTest)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	srv.httpSrv = httpServer

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	go func() {
		log.Printf("[WebServer] WhatsApp Web UI & API listening on http://%s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[WebServer] Error: %v", err)
		}
	}()

	return srv
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(uiHTML)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := s.manager.GetStatus()
	status["server_addr"] = s.addr
	status["platform"] = "minibox2"
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleQRImage(w http.ResponseWriter, r *http.Request) {
	qrData := s.manager.GetQRImage()
	if len(qrData) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "QR code is not currently active or device is already connected",
			"status":  s.manager.status,
			"success": false,
		})
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(qrData)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := s.manager.Logout(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "logged out and session reset"})
}

func (s *Server) handleReconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := s.manager.Reconnect()
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "reconnecting"})
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "invalid json"})
		return
	}

	err := s.manager.SendMessage(r.Context(), req.To, req.Message)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Message sent to %s", req.To),
	})
}

func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.manager.GetJoinedGroups(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(groups)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	logs := s.manager.GetLogs()
	_ = json.NewEncoder(w).Encode(logs)
}

func (s *Server) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.featureMgr != nil && !s.featureMgr.IsWebhookEnabled() {
		s.manager.AddLog("NOTIFY", "-", "Request rejected: Webhook is disabled", "FAILED", "Webhook disabled in settings")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Webhook notifier is disabled in feature settings",
		})
		return
	}

	var req SendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "invalid json: " + err.Error(),
		})
		return
	}

	req.To = strings.TrimSpace(req.To)
	req.Message = strings.TrimSpace(req.Message)
	if req.To == "" || req.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "recipient 'to' and 'message' are required and cannot be empty",
		})
		return
	}

	if err := s.manager.SendNotify(r.Context(), req.To, req.Message); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if s.featureMgr != nil {
		s.featureMgr.TouchWebhook(req.To, req.Source)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  "DELIVERED",
		"message": "notify applied",
		"to":      req.To,
		"time":    time.Now().Format("15:04:05"),
	})
}

// 5. Feature Management Handlers

func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}
	_ = json.NewEncoder(w).Encode(s.featureMgr.GetConfig())
}

func (s *Server) handleFeaturesSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	var req features.GlobalSettings
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json: " + err.Error()})
		return
	}

	if err := s.featureMgr.UpdateSettings(req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	s.manager.AddLog("SYSTEM", "Features", "Global settings updated", "SUCCESS", "")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "settings": req})
}

func (s *Server) handleFeaturesBuiltinToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json"})
		return
	}

	if err := s.featureMgr.ToggleBuiltin(req.ID, req.Enabled); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	s.manager.AddLog("SYSTEM", "Features", fmt.Sprintf("Builtin '%s' set to enabled=%v", req.ID, req.Enabled), "SUCCESS", "")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": req.ID, "enabled": req.Enabled})
}

func (s *Server) handleFeaturesCustoms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	if r.Method == http.MethodPost {
		var req features.CustomFeature
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json: " + err.Error()})
			return
		}
		created, err := s.featureMgr.AddCustom(req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		s.manager.AddLog("SYSTEM", "Features", fmt.Sprintf("Created custom feature '%s' (%s)", created.Trigger, created.ID), "SUCCESS", "")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "feature": created})
		return
	}

	_ = json.NewEncoder(w).Encode(s.featureMgr.GetConfig().Customs)
}

func (s *Server) handleFeaturesCustomToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json"})
		return
	}

	if err := s.featureMgr.ToggleCustom(req.ID, req.Enabled); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	s.manager.AddLog("SYSTEM", "Features", fmt.Sprintf("Custom feature '%s' set to enabled=%v", req.ID, req.Enabled), "SUCCESS", "")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": req.ID, "enabled": req.Enabled})
}

func (s *Server) handleFeaturesCustomDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json"})
		return
	}

	if err := s.featureMgr.DeleteCustom(req.ID); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	s.manager.AddLog("SYSTEM", "Features", fmt.Sprintf("Deleted custom feature '%s'", req.ID), "SUCCESS", "")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": req.ID})
}

func (s *Server) handleFeaturesCustomUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	var req features.CustomFeature
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json"})
		return
	}

	if err := s.featureMgr.UpdateCustom(req.ID, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	s.manager.AddLog("SYSTEM", "Features", fmt.Sprintf("Updated custom feature '%s' (%s)", req.Trigger, req.ID), "SUCCESS", "")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "feature": req})
}

// 6. Webhook Integrations Handlers

func (s *Server) handleWebhooks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	if r.Method == http.MethodPost {
		var req features.WebhookIntegration
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json: " + err.Error()})
			return
		}
		created, err := s.featureMgr.AddWebhook(req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		s.manager.AddLog("SYSTEM", "Webhook", fmt.Sprintf("Added webhook integration '%s' (%s)", created.Name, created.Target), "SUCCESS", "")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "webhook": created})
		return
	}

	_ = json.NewEncoder(w).Encode(s.featureMgr.GetWebhooks())
}

func (s *Server) handleWebhooksToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json"})
		return
	}

	if err := s.featureMgr.ToggleWebhook(req.ID, req.Enabled); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	s.manager.AddLog("SYSTEM", "Webhook", fmt.Sprintf("Webhook '%s' set to enabled=%v", req.ID, req.Enabled), "SUCCESS", "")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": req.ID, "enabled": req.Enabled})
}

func (s *Server) handleWebhooksDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json"})
		return
	}

	if err := s.featureMgr.DeleteWebhook(req.ID); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	s.manager.AddLog("SYSTEM", "Webhook", fmt.Sprintf("Deleted webhook integration '%s'", req.ID), "SUCCESS", "")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": req.ID})
}

func (s *Server) handleWebhooksUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.featureMgr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "feature manager not available"})
		return
	}

	var req features.WebhookIntegration
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json"})
		return
	}

	if err := s.featureMgr.UpdateWebhook(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	s.manager.AddLog("SYSTEM", "Webhook", fmt.Sprintf("Updated webhook '%s' (%s)", req.Name, req.ID), "SUCCESS", "")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "webhook": req})
}

func (s *Server) handleWebhooksTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Target string `json:"target"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid json: " + err.Error()})
		return
	}

	if req.Target == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "target recipient is required"})
		return
	}

	if req.Name == "" {
		req.Name = "Webhook Integration Test"
	}

	testMsg := fmt.Sprintf("⚡ *[UJI COBA WEBHOOK — BOTVISTA]* ⚡\n\n📌 *Layanan:* %s\n📡 *Sumber:* %s\n🎯 *Target:* %s\n⏱️ *Waktu:* %s WIB\n\n✅ Jalur integrasi webhook WhatsApp Gateway berhasil terhubung dan aktif!",
		req.Name, req.Source, req.Target, time.Now().Format("2006-01-02 15:04:05"))

	if err := s.manager.SendNotify(r.Context(), req.Target, testMsg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	if s.featureMgr != nil {
		s.featureMgr.TouchWebhook(req.Target, req.Name)
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Test ping webhook berhasil dikirim ke %s", req.Target),
		"time":    time.Now().Format("15:04:05"),
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getListenAddr() string {
	if addr := os.Getenv("ADDR"); addr != "" {
		return addr
	}
	if port := os.Getenv("PORT"); port != "" {
		if !strings.Contains(port, ":") {
			return "0.0.0.0:" + port
		}
		return port
	}
	return "0.0.0.0:18080"
}
