// Package app holds the live WhatsApp client and the connection state that
// the web UI reads. It is the single source of truth for "is the bot
// connected" and "what QR should we show".
package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
)

// Connection states reported to the UI.
const (
	StateConnected  = "connected"
	StateWaitingQR  = "waiting_qr"
	StateConnecting = "connecting"
)

// App wraps the WhatsApp client with thread-safe, UI-facing state.
type App struct {
	Client  *whatsmeow.Client
	Started time.Time

	mu     sync.RWMutex
	qrCode string
}

// New returns an App around an already-created client.
func New(client *whatsmeow.Client) *App {
	return &App{Client: client, Started: time.Now()}
}

// SetQR stores the latest pairing QR payload (empty string clears it).
func (a *App) SetQR(code string) {
	a.mu.Lock()
	a.qrCode = code
	a.mu.Unlock()
}

// QR returns the latest pairing QR payload, or "" if none.
func (a *App) QR() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.qrCode
}

// Number returns the logged-in phone number, or "" if not paired.
func (a *App) Number() string {
	if a.Client.Store.ID == nil {
		return ""
	}
	return a.Client.Store.ID.User
}

// State returns the current connection state for the UI.
func (a *App) State() string {
	switch {
	case a.Client.Store.ID == nil:
		return StateWaitingQR
	case a.Client.IsConnected():
		return StateConnected
	default:
		return StateConnecting
	}
}

// Uptime returns how long the process has been running.
func (a *App) Uptime() time.Duration {
	return time.Since(a.Started)
}

// Connect pairs via QR when no session is saved, otherwise reconnects the
// saved session. It blocks until the outcome is known, so run it in a
// goroutine. Once connected, whatsmeow keeps the connection alive itself.
func (a *App) Connect(ctx context.Context) error {
	if a.Client.Store.ID != nil {
		return a.Client.Connect()
	}

	qr, err := a.Client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("open QR channel: %w", err)
	}
	if err := a.Client.Connect(); err != nil {
		return err
	}
	for ev := range qr {
		switch ev.Event {
		case "code":
			a.SetQR(ev.Code)
		case "success":
			a.SetQR("")
			return nil
		default:
			if ev.Error != nil {
				return ev.Error
			}
			return fmt.Errorf("WhatsApp login ended: %s", ev.Event)
		}
	}
	return fmt.Errorf("WhatsApp QR channel closed")
}
