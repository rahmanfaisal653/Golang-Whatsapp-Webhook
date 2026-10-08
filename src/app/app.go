// Package app holds the live WhatsApp client and the connection state that
// the web UI reads. It is the single source of truth for "is the bot
// connected" and "what QR should we show".
package app

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.mau.fi/whatsmeow"
)

// Connection states reported to the UI.
const (
	StateConnected  = "connected"
	StateWaitingQR  = "waiting_qr"
	StateConnecting = "connecting"
	StateLoggedOut  = "logged_out"
)

// App wraps the WhatsApp client with thread-safe, UI-facing state.
type App struct {
	Client  *whatsmeow.Client
	Started time.Time

	mu     sync.RWMutex
	qrCode string

	loggedOut atomic.Bool
	restart   chan struct{}
	logs      *LogBuffer
}

// New returns an App around an already-created client. logs receives the
// application's own events (and is shared with the whatsmeow logger).
func New(client *whatsmeow.Client, logs *LogBuffer) *App {
	return &App{Client: client, Started: time.Now(), restart: make(chan struct{}, 1), logs: logs}
}

// Logs returns the shared log buffer for the UI.
func (a *App) Logs() *LogBuffer {
	return a.logs
}

// logf records an application-level log line (visible on the Logs page).
func (a *App) logf(level, msg string, args ...any) {
	if a.logs != nil {
		a.logs.Add(level, "App", fmt.Sprintf(msg, args...))
	}
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
	case a.loggedOut.Load():
		return StateLoggedOut
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

// LoggedOut reports whether WhatsApp has unlinked this device.
func (a *App) LoggedOut() bool {
	return a.loggedOut.Load()
}

// Restart returns a channel that receives once the process should restart.
// After a logout the current client is dead (whatsmeow deleted its store), so a
// fresh process — hence a fresh client and device — is the only way to pair
// again. The supervisor (PM2) restarts the process on exit.
func (a *App) Restart() <-chan struct{} {
	return a.restart
}

// MarkLoggedOut records that the device was unlinked and asks main to restart.
func (a *App) MarkLoggedOut() {
	a.SetQR("")
	a.loggedOut.Store(true)
	a.logf("WARN", "WhatsApp device unlinked — restarting to pair a new device")
	select {
	case a.restart <- struct{}{}:
	default: // already signalled
	}
}

// Logout unlinks the device from WhatsApp (user-initiated) and requests a
// restart so the bot returns ready to pair a new device.
func (a *App) Logout(ctx context.Context) error {
	a.logf("INFO", "unlink requested from dashboard")
	if err := a.Client.Logout(ctx); err != nil {
		a.logf("ERROR", "logout failed: %v", err)
		return err
	}
	a.MarkLoggedOut()
	return nil
}

// Connect pairs via QR when no session is saved, otherwise reconnects the
// saved session. When pairing, a QR code only lives a couple of minutes; if the
// scan does not happen in time whatsmeow ends the channel. Instead of giving up
// (which leaves a stale, unscannable QR on the dashboard) we request a fresh QR
// and keep going until pairing succeeds or the context is cancelled.
func (a *App) Connect(ctx context.Context) error {
	if a.Client.Store.ID != nil {
		a.logf("INFO", "reconnecting saved session")
		err := a.Client.Connect()
		if err != nil {
			a.logf("ERROR", "reconnect failed: %v", err)
		}
		return err
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := a.pairOnce(ctx); err != nil {
			a.logf("WARN", "pairing attempt ended (%v) — requesting a new QR", err)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		return nil
	}
}

// pairOnce opens one QR channel and blocks until pairing succeeds or the
// channel ends. Each time whatsmeow rotates the code a new QR is published.
func (a *App) pairOnce(ctx context.Context) error {
	a.logf("INFO", "no saved session — waiting for QR scan")
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
			a.logf("INFO", "device paired successfully")
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
