package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"
)

// newTestApp builds an App around a throwaway in-memory-ish device store.
func newTestApp(t *testing.T) *App {
	t.Helper()
	db := filepath.Join(t.TempDir(), "t.db")
	c, err := sqlstore.New(context.Background(), "sqlite", "file:"+db+"?_pragma=foreign_keys(1)", waLog.Noop)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := c.GetFirstDevice(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return New(whatsmeow.NewClient(dev, waLog.Noop))
}

// A fresh, unpaired app reports waiting_qr (not connected).
func TestStateFreshIsWaitingQR(t *testing.T) {
	a := newTestApp(t)
	if got := a.State(); got != StateWaitingQR {
		t.Fatalf("fresh state = %q, want %q", got, StateWaitingQR)
	}
	if a.QR() != "" {
		t.Fatalf("fresh QR should be empty")
	}
}

// After a logout the app reports logged_out and signals a restart exactly once.
func TestMarkLoggedOut(t *testing.T) {
	a := newTestApp(t)
	a.MarkLoggedOut()

	if got := a.State(); got != StateLoggedOut {
		t.Fatalf("state = %q, want %q", got, StateLoggedOut)
	}
	if !a.LoggedOut() {
		t.Fatalf("LoggedOut() should be true")
	}
	select {
	case <-a.Restart():
	case <-time.After(time.Second):
		t.Fatalf("Restart() channel was not signalled")
	}
	// A second MarkLoggedOut must not block (non-blocking send).
	a.MarkLoggedOut()
}

// A paired-looking device (ID set) that is not connected reports connecting.
func TestStatePairedNotConnectedIsConnecting(t *testing.T) {
	a := newTestApp(t)
	jid := types.NewJID("628111222333", types.DefaultUserServer)
	a.Client.Store.ID = &jid
	if got := a.State(); got != StateConnecting {
		t.Fatalf("state = %q, want %q", got, StateConnecting)
	}
	if a.Number() != "628111222333" {
		t.Fatalf("number = %q", a.Number())
	}
}
