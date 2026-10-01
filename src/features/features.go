package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type GlobalSettings struct {
	BotEnabled         bool   `json:"bot_enabled"`
	WebhookEnabled     bool   `json:"webhook_enabled"`
	OutgoingWebhookURL string `json:"outgoing_webhook_url"`
	GroupResponse      bool   `json:"group_response"`
	PrivateResponse    bool   `json:"private_response"`
	AutoReconnect      bool   `json:"auto_reconnect"`
}

type BuiltinCommand struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	OwnerOnly   bool   `json:"owner_only"`
	AdminOnly   bool   `json:"admin_only"`
}

type CustomFeature struct {
	ID          string `json:"id"`
	Trigger     string `json:"trigger"`
	MatchType   string `json:"match_type"` // "exact", "prefix", "contains"
	Response    string `json:"response"`
	Description string `json:"description"`
	Target      string `json:"target"` // "all", "private_only", "group_only"
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
}

type WebhookIntegration struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	TargetName  string `json:"target_name"`
	Endpoint    string `json:"endpoint"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	LastUsed    string `json:"last_used"`
}

type Config struct {
	Settings GlobalSettings       `json:"settings"`
	Builtins []BuiltinCommand     `json:"builtins"`
	Customs  []CustomFeature      `json:"customs"`
	Webhooks []WebhookIntegration `json:"webhooks"`
}

type Manager struct {
	filePath string
	mu       sync.RWMutex
	config   Config
}

func DefaultConfig() Config {
	return Config{
		Settings: GlobalSettings{
			BotEnabled:      true,
			WebhookEnabled:  true,
			GroupResponse:   true,
			PrivateResponse: true,
			AutoReconnect:   true,
		},
		Builtins: []BuiltinCommand{
			{ID: "ping", Name: ".ping", Description: "Cek responsivitas bot WhatsApp (pong)", Enabled: true},
			{ID: "about", Name: ".about", Description: "Informasi bot gateway WhatsApp", Enabled: true},
			{ID: "menu", Name: ".menu", Description: "Menampilkan daftar perintah aktif dinamis", Enabled: true},
			{ID: "uptime", Name: ".uptime", Description: "Menampilkan durasi bot aktif berjalan", Enabled: true},
			{ID: "id", Name: ".id", Description: "Menampilkan Sender & Chat JID pengirim", Enabled: true},
			{ID: "groups", Name: ".groups", Description: "Menampilkan daftar grup WhatsApp yang diikuti", Enabled: true},
			{ID: "test", Name: ".test", Description: "Menguji endpoint notifier lokal", Enabled: true},
		},
		Customs: []CustomFeature{
			{
				ID:          "feat_welcome",
				Trigger:     "halo",
				MatchType:   "exact",
				Response:    "Halo {sender}! 👋 Selamat datang di WhatsApp Gateway Nouvem.\nKetik *.menu* untuk melihat fitur yang tersedia.",
				Description: "Auto-reply sapaan awal",
				Target:      "all",
				Enabled:     true,
				CreatedAt:   time.Now().Format("2006-01-02 15:04"),
			},
		},
		Webhooks: []WebhookIntegration{
			{
				ID:          "wh_kp_guard_nas",
				Name:        "KP-Guard NAS (Thermal & Load Governor)",
				Description: "Pemantau suhu CPU (≥88°C, ≥92°C), fisik disk (>60°C), dan lonjakan beban Synology DS923+",
				Source:      "Synology NAS DS923+ (kp-load-guard.service)",
				Target:      "120363404628369352@g.us",
				TargetName:  "Grup KroomBox",
				Endpoint:    "/notify",
				Enabled:     true,
				CreatedAt:   "2026-10-01 14:45",
				LastUsed:    time.Now().Format("2006-01-02 15:04:05"),
			},
			{
				ID:          "wh_kolabpanel_deploy",
				Name:        "KolabPanel Deployment & Build Notifier",
				Description: "Notifikasi otomatis saat proses deploy, SSL renew, atau build situs di server panel",
				Source:      "KolabPanel Hosting Core",
				Target:      "120363404628369352@g.us",
				TargetName:  "Grup KroomBox",
				Endpoint:    "/notify",
				Enabled:     true,
				CreatedAt:   "2026-10-01 14:00",
				LastUsed:    "-",
			},
		},
	}
}

func NewManager(filePath string) (*Manager, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create features dir: %w", err)
	}

	mgr := &Manager{
		filePath: filePath,
		config:   DefaultConfig(),
	}

	if err := mgr.load(); err != nil {
		if os.IsNotExist(err) {
			if saveErr := mgr.save(); saveErr != nil {
				return nil, fmt.Errorf("initial save features config: %w", saveErr)
			}
		} else {
			return nil, fmt.Errorf("load features config: %w", err)
		}
	}

	return mgr, nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Merge default builtins if any are missing from saved config
	defaultBuiltins := DefaultConfig().Builtins
	existingMap := make(map[string]bool)
	for _, b := range loaded.Builtins {
		existingMap[b.ID] = true
	}
	for _, defB := range defaultBuiltins {
		if !existingMap[defB.ID] {
			loaded.Builtins = append(loaded.Builtins, defB)
		}
	}

	if loaded.Customs == nil {
		loaded.Customs = make([]CustomFeature, 0)
	}

	// Merge default webhooks if missing or empty
	defaultWebhooks := DefaultConfig().Webhooks
	if len(loaded.Webhooks) == 0 {
		loaded.Webhooks = defaultWebhooks
	} else {
		whMap := make(map[string]bool)
		for _, w := range loaded.Webhooks {
			whMap[w.ID] = true
		}
		for _, defW := range defaultWebhooks {
			if !whMap[defW.ID] {
				loaded.Webhooks = append(loaded.Webhooks, defW)
			}
		}
	}

	m.config = loaded
	return nil
}

func (m *Manager) save() error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.config, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	tmpFile := m.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o666); err != nil {
		return fmt.Errorf("write tmp features file: %w", err)
	}
	return os.Rename(tmpFile, m.filePath)
}

func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return deep copy
	copied := m.config
	copied.Builtins = make([]BuiltinCommand, len(m.config.Builtins))
	copy(copied.Builtins, m.config.Builtins)
	copied.Customs = make([]CustomFeature, len(m.config.Customs))
	copy(copied.Customs, m.config.Customs)
	copied.Webhooks = make([]WebhookIntegration, len(m.config.Webhooks))
	copy(copied.Webhooks, m.config.Webhooks)
	return copied
}

func (m *Manager) UpdateSettings(s GlobalSettings) error {
	m.mu.Lock()
	m.config.Settings = s
	m.mu.Unlock()
	return m.save()
}

func (m *Manager) ToggleBuiltin(id string, enabled bool) error {
	m.mu.Lock()
	found := false
	for i := range m.config.Builtins {
		if m.config.Builtins[i].ID == id {
			m.config.Builtins[i].Enabled = enabled
			found = true
			break
		}
	}
	m.mu.Unlock()

	if !found {
		return fmt.Errorf("builtin command '%s' not found", id)
	}
	return m.save()
}

func (m *Manager) IsCommandEnabled(cmdName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clean := strings.ToLower(strings.TrimSpace(cmdName))
	for _, b := range m.config.Builtins {
		if strings.ToLower(b.Name) == clean || strings.ToLower(b.ID) == strings.TrimPrefix(clean, ".") {
			return b.Enabled
		}
	}
	return true
}

func (m *Manager) IsBotEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Settings.BotEnabled
}

func (m *Manager) IsWebhookEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Settings.WebhookEnabled
}

func (m *Manager) IsAutoReconnectEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Settings.AutoReconnect
}

func (m *Manager) AddCustom(f CustomFeature) (CustomFeature, error) {
	if strings.TrimSpace(f.Trigger) == "" {
		return f, fmt.Errorf("kata kunci (trigger) tidak boleh kosong")
	}
	if strings.TrimSpace(f.Response) == "" {
		return f, fmt.Errorf("pesan balasan (response) tidak boleh kosong")
	}

	m.mu.Lock()
	f.ID = fmt.Sprintf("feat_%d", time.Now().UnixNano())
	f.CreatedAt = time.Now().Format("2006-01-02 15:04")
	if f.MatchType == "" {
		f.MatchType = "exact"
	}
	if f.Target == "" {
		f.Target = "all"
	}
	m.config.Customs = append(m.config.Customs, f)
	m.mu.Unlock()

	if err := m.save(); err != nil {
		return f, err
	}
	return f, nil
}

func (m *Manager) UpdateCustom(id string, updated CustomFeature) error {
	if strings.TrimSpace(updated.Trigger) == "" {
		return fmt.Errorf("kata kunci (trigger) tidak boleh kosong")
	}
	if strings.TrimSpace(updated.Response) == "" {
		return fmt.Errorf("pesan balasan (response) tidak boleh kosong")
	}

	m.mu.Lock()
	found := false
	for i := range m.config.Customs {
		if m.config.Customs[i].ID == id {
			updated.ID = id
			if updated.CreatedAt == "" {
				updated.CreatedAt = m.config.Customs[i].CreatedAt
			}
			m.config.Customs[i] = updated
			found = true
			break
		}
	}
	m.mu.Unlock()

	if !found {
		return fmt.Errorf("custom feature '%s' not found", id)
	}
	return m.save()
}

func (m *Manager) DeleteCustom(id string) error {
	m.mu.Lock()
	newCustoms := make([]CustomFeature, 0, len(m.config.Customs))
	found := false
	for _, c := range m.config.Customs {
		if c.ID == id {
			found = true
			continue
		}
		newCustoms = append(newCustoms, c)
	}
	if found {
		m.config.Customs = newCustoms
	}
	m.mu.Unlock()

	if !found {
		return fmt.Errorf("custom feature '%s' not found", id)
	}
	return m.save()
}

func (m *Manager) ToggleCustom(id string, enabled bool) error {
	m.mu.Lock()
	found := false
	for i := range m.config.Customs {
		if m.config.Customs[i].ID == id {
			m.config.Customs[i].Enabled = enabled
			found = true
			break
		}
	}
	m.mu.Unlock()

	if !found {
		return fmt.Errorf("custom feature '%s' not found", id)
	}
	return m.save()
}

func (m *Manager) MatchCustom(text string, isGroup bool) *CustomFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cleanText := strings.TrimSpace(text)
	lowerText := strings.ToLower(cleanText)

	for _, c := range m.config.Customs {
		if !c.Enabled {
			continue
		}
		// Check target
		if c.Target == "group_only" && !isGroup {
			continue
		}
		if c.Target == "private_only" && isGroup {
			continue
		}

		trigLower := strings.ToLower(strings.TrimSpace(c.Trigger))
		matched := false
		switch c.MatchType {
		case "exact":
			matched = lowerText == trigLower
		case "prefix":
			matched = strings.HasPrefix(lowerText, trigLower)
		case "contains":
			matched = strings.Contains(lowerText, trigLower)
		default:
			matched = lowerText == trigLower
		}

		if matched {
			res := c
			return &res
		}
	}
	return nil
}

func (m *Manager) InterpolateResponse(tpl string, sender string, chat string) string {
	now := time.Now()
	res := tpl
	res = strings.ReplaceAll(res, "{sender}", sender)
	res = strings.ReplaceAll(res, "{chat}", chat)
	res = strings.ReplaceAll(res, "{time}", now.Format("15:04:05"))
	res = strings.ReplaceAll(res, "{date}", now.Format("2006-01-02"))
	return res
}

func (m *Manager) FormatMenuText() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("🌟 *DAFTAR PERINTAH & FITUR AKTIF* 🌟\n\n")

	sb.WriteString("📌 *Perintah Utama:*\n")
	hasBuiltin := false
	for _, b := range m.config.Builtins {
		if b.Enabled {
			hasBuiltin = true
			sb.WriteString(fmt.Sprintf("• *%s* — %s\n", b.Name, b.Description))
		}
	}
	if !hasBuiltin {
		sb.WriteString("_(Semua perintah bawaan sedang dinonaktifkan)_\n")
	}

	hasCustom := false
	for _, c := range m.config.Customs {
		if c.Enabled {
			if !hasCustom {
				sb.WriteString("\n⚡ *Auto-Responder & Respon Otomatis:*\n")
				hasCustom = true
			}
			matchLabel := ""
			switch c.MatchType {
			case "exact":
				matchLabel = "tepat"
			case "prefix":
				matchLabel = "awalan"
			case "contains":
				matchLabel = "mengandung"
			}
			desc := c.Description
			if desc == "" {
				desc = "Auto reply"
			}
			sb.WriteString(fmt.Sprintf("• *\"%s\"* (%s) — %s\n", c.Trigger, matchLabel, desc))
		}
	}

	sb.WriteString("\n_Ketik salah satu perintah atau kata kunci di atas untuk berinteraksi._")
	return sb.String()
}

// ─── WEBHOOK INTEGRATIONS MANAGEMENT ──────────────────────────────────────────

func (m *Manager) GetWebhooks() []WebhookIntegration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copied := make([]WebhookIntegration, len(m.config.Webhooks))
	copy(copied, m.config.Webhooks)
	return copied
}

func (m *Manager) AddWebhook(w WebhookIntegration) (WebhookIntegration, error) {
	if strings.TrimSpace(w.Name) == "" {
		return w, fmt.Errorf("nama webhook tidak boleh kosong")
	}
	if strings.TrimSpace(w.Target) == "" {
		return w, fmt.Errorf("target WhatsApp tidak boleh kosong")
	}

	m.mu.Lock()
	if w.ID == "" {
		w.ID = fmt.Sprintf("wh_%d", time.Now().UnixNano())
	}
	if w.CreatedAt == "" {
		w.CreatedAt = time.Now().Format("2006-01-02 15:04")
	}
	if w.Endpoint == "" {
		w.Endpoint = "/notify"
	}
	if w.LastUsed == "" {
		w.LastUsed = "-"
	}

	m.config.Webhooks = append(m.config.Webhooks, w)
	m.mu.Unlock()

	return w, m.save()
}

func (m *Manager) ToggleWebhook(id string, enabled bool) error {
	m.mu.Lock()
	found := false
	for i := range m.config.Webhooks {
		if m.config.Webhooks[i].ID == id {
			m.config.Webhooks[i].Enabled = enabled
			found = true
			break
		}
	}
	m.mu.Unlock()

	if !found {
		return fmt.Errorf("webhook integration '%s' not found", id)
	}
	return m.save()
}

func (m *Manager) UpdateWebhook(w WebhookIntegration) error {
	m.mu.Lock()
	found := false
	for i := range m.config.Webhooks {
		if m.config.Webhooks[i].ID == w.ID {
			m.config.Webhooks[i].Name = w.Name
			m.config.Webhooks[i].Description = w.Description
			m.config.Webhooks[i].Source = w.Source
			m.config.Webhooks[i].Target = w.Target
			m.config.Webhooks[i].TargetName = w.TargetName
			m.config.Webhooks[i].Endpoint = w.Endpoint
			m.config.Webhooks[i].Enabled = w.Enabled
			found = true
			break
		}
	}
	m.mu.Unlock()

	if !found {
		return fmt.Errorf("webhook integration '%s' not found", w.ID)
	}
	return m.save()
}

func (m *Manager) DeleteWebhook(id string) error {
	m.mu.Lock()
	idx := -1
	for i := range m.config.Webhooks {
		if m.config.Webhooks[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		m.mu.Unlock()
		return fmt.Errorf("webhook integration '%s' not found", id)
	}
	m.config.Webhooks = append(m.config.Webhooks[:idx], m.config.Webhooks[idx+1:]...)
	m.mu.Unlock()
	return m.save()
}

func (m *Manager) TouchWebhook(target string, source string) {
	m.mu.Lock()
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	changed := false
	for i := range m.config.Webhooks {
		w := &m.config.Webhooks[i]
		cleanTarget := strings.TrimSpace(target)
		cleanSource := strings.TrimSpace(source)

		match := false
		if cleanSource != "" {
			if strings.EqualFold(w.ID, cleanSource) ||
				strings.Contains(strings.ToLower(w.Name), strings.ToLower(cleanSource)) ||
				strings.Contains(strings.ToLower(w.Source), strings.ToLower(cleanSource)) {
				match = true
			}
		}
		if !match && cleanTarget != "" {
			if strings.EqualFold(strings.TrimSpace(w.Target), cleanTarget) ||
				strings.HasPrefix(cleanTarget, strings.TrimSuffix(strings.TrimSpace(w.Target), "@s.whatsapp.net")) {
				match = true
			}
		}

		if match {
			w.LastUsed = nowStr
			changed = true
		}
	}
	m.mu.Unlock()

	if changed {
		_ = m.save()
	}
}
