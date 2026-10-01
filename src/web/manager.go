package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
	"rsc.io/qr"

	"github.com/local/whatsmeow-base/src/features"
	"github.com/local/whatsmeow-base/src/lib"
)

type LogEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`   // "NOTIFY", "DIRECT", "GROUP", "SYSTEM"
	Target    string `json:"target"`
	Message   string `json:"message"`
	Status    string `json:"status"` // "SUCCESS", "FAILED"
	Details   string `json:"details,omitempty"`
}

type GroupInfo struct {
	JID         string `json:"jid"`
	Name        string `json:"name"`
	Topic       string `json:"topic"`
	Owner       string `json:"owner"`
	MemberCount int    `json:"member_count"`
}

type WhatsAppManager struct {
	ctx          context.Context
	cancel       context.CancelFunc
	sessionDir   string
	store        *sqlstore.Container
	client       *whatsmeow.Client
	eventHandler func(interface{})
	featureMgr   *features.Manager
	mu           sync.RWMutex

	status       string // "DISCONNECTED", "CONNECTING", "WAITING_FOR_QR", "CONNECTED", "LOGGED_OUT", "ERROR"
	qrCode       string
	qrImage      []byte
	startTime    time.Time
	lastConnect  time.Time
	isConnecting bool

	logsMu sync.RWMutex
	logs   []LogEntry
}

func NewWhatsAppManager(ctx context.Context, sessionDir string) (*WhatsAppManager, error) {
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}

	dbPath := filepath.Join(sessionDir, "whatsmeow.db")
	uri := "file:" + filepath.ToSlash(dbPath) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"

	store, err := sqlstore.New(ctx, "sqlite", uri, waLog.Stdout("Database", "INFO", true))
	if err != nil {
		return nil, fmt.Errorf("open device store: %w", err)
	}

	device, err := store.GetFirstDevice(ctx)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("load device: %w", err)
	}

	mCtx, cancel := context.WithCancel(ctx)
	client := whatsmeow.NewClient(device, waLog.Stdout("WhatsApp", "INFO", true))

	mgr := &WhatsAppManager{
		ctx:        mCtx,
		cancel:     cancel,
		sessionDir: sessionDir,
		store:      store,
		client:     client,
		status:     "DISCONNECTED",
		startTime:  time.Now(),
		logs:       make([]LogEntry, 0, 50),
	}

	client.AddEventHandler(mgr.handleEvents)
	return mgr, nil
}

func (m *WhatsAppManager) SetExternalEventHandler(h func(interface{})) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventHandler = h
}

func (m *WhatsAppManager) SetFeatureManager(fm *features.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.featureMgr = fm
}

func (m *WhatsAppManager) GetClient() *whatsmeow.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

func (m *WhatsAppManager) handleEvents(evt interface{}) {
	switch v := evt.(type) {
	case *events.Connected:
		m.mu.Lock()
		m.status = "CONNECTED"
		m.qrCode = ""
		m.qrImage = nil
		m.lastConnect = time.Now()
		m.mu.Unlock()
		m.AddLog("SYSTEM", "WhatsApp", "Connected to WhatsApp successfully", "SUCCESS", "")
		fmt.Println("[WhatsAppManager] Event: Connected to WhatsApp!")

	case *events.LoggedOut:
		m.mu.Lock()
		m.status = "LOGGED_OUT"
		m.qrCode = ""
		m.qrImage = nil
		m.mu.Unlock()
		m.AddLog("SYSTEM", "WhatsApp", fmt.Sprintf("Logged out: %v", v.Reason), "FAILED", "")
		fmt.Printf("[WhatsAppManager] Event: Logged out (%v). Resetting session...\n", v.Reason)
		if m.featureMgr == nil || m.featureMgr.IsAutoReconnectEnabled() {
			go func() {
				time.Sleep(1 * time.Second)
				_ = m.StartConnect()
			}()
		}

	case *events.Disconnected:
		m.mu.Lock()
		if m.status == "CONNECTED" {
			m.status = "DISCONNECTED"
		}
		m.mu.Unlock()
		m.AddLog("SYSTEM", "WhatsApp", "Connection lost from WhatsApp server", "FAILED", "")
		fmt.Println("[WhatsAppManager] Event: Disconnected from WhatsApp server")
	}

	m.mu.RLock()
	extHandler := m.eventHandler
	m.mu.RUnlock()
	if extHandler != nil {
		extHandler(evt)
	}
}

func (m *WhatsAppManager) StartConnect() error {
	m.mu.Lock()
	if m.isConnecting {
		m.mu.Unlock()
		return nil
	}
	m.isConnecting = true
	client := m.client
	m.status = "CONNECTING"
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.isConnecting = false
		m.mu.Unlock()
	}()

	if client.Store.ID != nil {
		// Session exists, connect directly
		err := client.Connect()
		if err != nil {
			m.mu.Lock()
			m.status = "DISCONNECTED"
			m.mu.Unlock()
			m.AddLog("SYSTEM", "WhatsApp", fmt.Sprintf("Connect failed: %v", err), "FAILED", "")
			return err
		}
		m.mu.Lock()
		m.status = "CONNECTED"
		m.lastConnect = time.Now()
		m.mu.Unlock()
		return nil
	}

	// No session stored, listen for QR code
	qrChan, err := client.GetQRChannel(m.ctx)
	if err != nil {
		m.mu.Lock()
		m.status = "ERROR"
		m.mu.Unlock()
		m.AddLog("SYSTEM", "WhatsApp", fmt.Sprintf("Failed to get QR channel: %v", err), "FAILED", "")
		return fmt.Errorf("open QR channel: %w", err)
	}

	if err := client.Connect(); err != nil {
		m.mu.Lock()
		m.status = "ERROR"
		m.mu.Unlock()
		m.AddLog("SYSTEM", "WhatsApp", fmt.Sprintf("Client connect error: %v", err), "FAILED", "")
		return err
	}

	go func() {
		for event := range qrChan {
			switch event.Event {
			case "code":
				pngBytes := generateQRPNG(event.Code)
				m.mu.Lock()
				m.qrCode = event.Code
				m.qrImage = pngBytes
				m.status = "WAITING_FOR_QR"
				m.mu.Unlock()

				fmt.Println("[WhatsAppManager] QR Code generated. Scan in WhatsApp or visit Web UI:")
				qrterminal.GenerateHalfBlock(event.Code, qrterminal.L, os.Stdout)

			case "success":
				m.mu.Lock()
				m.status = "CONNECTED"
				m.qrCode = ""
				m.qrImage = nil
				m.lastConnect = time.Now()
				m.mu.Unlock()
				m.AddLog("SYSTEM", "WhatsApp", "QR Pair successful! WhatsApp session active.", "SUCCESS", "")
				fmt.Println("[WhatsAppManager] Pair successful! Connected to WhatsApp.")

			case "timeout":
				m.mu.Lock()
				m.status = "WAITING_FOR_QR"
				m.mu.Unlock()
				fmt.Println("[WhatsAppManager] QR code expired, waiting for next...")

			default:
				if event.Error != nil {
					m.mu.Lock()
					m.status = "ERROR: " + event.Error.Error()
					m.mu.Unlock()
					fmt.Printf("[WhatsAppManager] QR channel event error: %v\n", event.Error)
				}
			}
		}
	}()

	return nil
}

func (m *WhatsAppManager) Logout(ctx context.Context) error {
	m.mu.Lock()
	client := m.client
	store := m.store
	m.mu.Unlock()

	if client == nil {
		return errors.New("client not initialized")
	}

	// Unlink/Logout from WhatsApp
	_ = client.Logout(ctx)
	client.Disconnect()
	if client.Store != nil {
		_ = client.Store.Delete(ctx)
	}

	// Create fresh device store for new session
	newDevice, err := store.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("create new device: %w", err)
	}

	newClient := whatsmeow.NewClient(newDevice, waLog.Stdout("WhatsApp", "INFO", true))
	newClient.AddEventHandler(m.handleEvents)

	m.mu.Lock()
	m.client = newClient
	m.status = "DISCONNECTED"
	m.qrCode = ""
	m.qrImage = nil
	m.mu.Unlock()

	m.AddLog("SYSTEM", "WhatsApp", "Session cleared. Disconnected for new pairing.", "SUCCESS", "")
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = m.StartConnect()
	}()

	return nil
}

func (m *WhatsAppManager) Reconnect() error {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()

	if client == nil {
		return errors.New("client not initialized")
	}

	client.Disconnect()
	return m.StartConnect()
}

func (m *WhatsAppManager) SendMessage(ctx context.Context, target string, text string) error {
	return m.SendMessageWithType(ctx, "DIRECT", target, text)
}

func (m *WhatsAppManager) SendNotify(ctx context.Context, target string, text string) error {
	return m.SendMessageWithType(ctx, "NOTIFY", target, text)
}

func (m *WhatsAppManager) SendMessageWithType(ctx context.Context, logType, target, text string) error {
	m.mu.RLock()
	client := m.client
	status := m.status
	m.mu.RUnlock()

	if client == nil || status != "CONNECTED" {
		return fmt.Errorf("WhatsApp is not connected (current status: %s)", status)
	}

	cleanTarget := strings.TrimSpace(target)
	cleanText := strings.TrimSpace(text)
	if cleanTarget == "" || cleanText == "" {
		return errors.New("recipient and message must not be empty")
	}

	jid, err := lib.ParseRecipientJID(cleanTarget)
	if err != nil {
		m.AddLog(logType, cleanTarget, cleanText, "FAILED", err.Error())
		return err
	}

	msg := &waE2E.Message{
		Conversation: proto.String(cleanText),
	}

	_, sendErr := client.SendMessage(ctx, jid.ToNonAD(), msg)
	if sendErr != nil {
		m.AddLog(logType, jid.String(), cleanText, "FAILED", sendErr.Error())
		return fmt.Errorf("send whatsapp (%s): %w", jid.String(), sendErr)
	}

	finalLogType := logType
	if jid.Server == "g.us" {
		finalLogType = "GROUP"
	}
	m.AddLog(finalLogType, jid.String(), cleanText, "SUCCESS", "")
	return nil
}

func (m *WhatsAppManager) SendComplexMessage(ctx context.Context, p SendPayload) error {
	m.mu.RLock()
	client := m.client
	status := m.status
	m.mu.RUnlock()

	if client == nil || status != "CONNECTED" {
		return fmt.Errorf("WhatsApp is not connected (current status: %s)", status)
	}

	target := strings.TrimSpace(p.To)
	msgType := strings.ToLower(strings.TrimSpace(p.Type))
	if msgType == "" {
		msgType = "text"
	}
	if msgType == "status" {
		target = "status@broadcast"
	}
	if target == "" {
		return errors.New("recipient target cannot be empty")
	}

	jid, err := lib.ParseRecipientJID(target)
	if err != nil {
		m.AddLog(strings.ToUpper(msgType), target, p.Message, "FAILED", err.Error())
		return err
	}

	msg, err := m.BuildMessage(ctx, p)
	if err != nil {
		m.AddLog(strings.ToUpper(msgType), target, p.Message, "FAILED", err.Error())
		return err
	}

	_, sendErr := client.SendMessage(ctx, jid.ToNonAD(), msg)
	if sendErr != nil {
		m.AddLog(strings.ToUpper(msgType), jid.String(), p.Message, "FAILED", sendErr.Error())
		return fmt.Errorf("send whatsapp (%s): %w", jid.String(), sendErr)
	}

	finalLogType := strings.ToUpper(msgType)
	if jid.Server == "g.us" {
		finalLogType = "GROUP_" + finalLogType
	} else if jid.Server == "broadcast" {
		finalLogType = "STATUS"
	}
	m.AddLog(finalLogType, jid.String(), p.Message, "SUCCESS", "")
	return nil
}

func (m *WhatsAppManager) GetJoinedGroups(ctx context.Context) ([]GroupInfo, error) {
	m.mu.RLock()
	client := m.client
	status := m.status
	m.mu.RUnlock()

	if client == nil || status != "CONNECTED" {
		return nil, fmt.Errorf("WhatsApp is not connected (status: %s)", status)
	}

	rawGroups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		return nil, err
	}

	groups := make([]GroupInfo, 0, len(rawGroups))
	for _, g := range rawGroups {
		topic := g.Topic
		owner := g.OwnerJID.String()
		groups = append(groups, GroupInfo{
			JID:         g.JID.String(),
			Name:        g.Name,
			Topic:       topic,
			Owner:       owner,
			MemberCount: len(g.Participants),
		})
	}
	return groups, nil
}

func (m *WhatsAppManager) GetQRImage() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.qrImage
}

func (m *WhatsAppManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connected := m.status == "CONNECTED"
	phone := ""
	deviceID := ""
	pushName := ""

	if m.client != nil && m.client.Store != nil && m.client.Store.ID != nil {
		phone = m.client.Store.ID.User
		deviceID = m.client.Store.ID.String()
		pushName = m.client.Store.PushName
	}

	uptime := time.Since(m.startTime).Round(time.Second)
	uptimeStr := formatDuration(uptime)

	m.logsMu.RLock()
	logsCount := len(m.logs)
	m.logsMu.RUnlock()

	return map[string]interface{}{
		"connected":      connected,
		"status":         m.status,
		"phone":          phone,
		"push_name":      pushName,
		"device_id":      deviceID,
		"platform":       "minibox2",
		"uptime":         uptimeStr,
		"uptime_seconds": int64(uptime.Seconds()),
		"qr_available":   len(m.qrImage) > 0 && m.status == "WAITING_FOR_QR",
		"logs_count":     logsCount,
	}
}

func (m *WhatsAppManager) AddLog(entryType, target, message, status, details string) {
	m.logsMu.Lock()
	defer m.logsMu.Unlock()

	preview := message
	if len(preview) > 120 {
		preview = preview[:117] + "..."
	}

	entry := LogEntry{
		ID:        fmt.Sprintf("log_%d", time.Now().UnixNano()),
		Timestamp: time.Now().Format("15:04:05"),
		Type:      entryType,
		Target:    target,
		Message:   preview,
		Status:    status,
		Details:   details,
	}

	// Prepend to show latest first, capped at 50
	m.logs = append([]LogEntry{entry}, m.logs...)
	if len(m.logs) > 50 {
		m.logs = m.logs[:50]
	}
}

func (m *WhatsAppManager) GetLogs() []LogEntry {
	m.logsMu.RLock()
	defer m.logsMu.RUnlock()
	result := make([]LogEntry, len(m.logs))
	copy(result, m.logs)
	return result
}

func (m *WhatsAppManager) Close() {
	m.cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil {
		m.client.Disconnect()
	}
	if m.store != nil {
		m.store.Close()
	}
}

func generateQRPNG(text string) []byte {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return nil
	}
	return code.PNG()
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
