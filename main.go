package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	_ "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"

	"github.com/local/whatsmeow-base/commands/owner"
	"github.com/local/whatsmeow-base/src/lib"
)

func main() {
	ctx := context.Background()
	if err := os.MkdirAll(filepath.Join("src", "session"), 0o700); err != nil {
		fatal("create data directory", err)
	}

	store, err := sqlstore.New(ctx, "sqlite", sqliteURI(filepath.Join("src", "session", "whatsmeow.db")), waLog.Stdout("Database", "INFO", true))
	if err != nil {
		fatal("open device store", err)
	}
	defer store.Close()

	device, err := store.GetFirstDevice(ctx)
	if err != nil {
		fatal("load device", err)
	}

	client := whatsmeow.NewClient(device, waLog.Stdout("WhatsApp", "INFO", true))

	ownerJID := parseOwnerJID()
	handler := lib.NewCommandHandler(ctx, client, ownerJID)
	handler.Register(owner.PingCommand{})
	handler.LogSummary()
	client.AddEventHandler(handler.Handle)

	if err := connect(ctx, client); err != nil {
		fatal("connect", err)
	}
	defer client.Disconnect()

	fmt.Println("Connected. Press CTRL+C to stop.")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}

func connect(ctx context.Context, client *whatsmeow.Client) error {
	if client.Store.ID != nil {
		return client.Connect()
	}

	qr, err := client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("open QR channel: %w", err)
	}
	if err := client.Connect(); err != nil {
		return err
	}

	for event := range qr {
		switch event.Event {
		case "code":
			fmt.Println("Scan this QR in WhatsApp > Linked devices:")
			qrterminal.GenerateHalfBlock(event.Code, qrterminal.L, os.Stdout)
		case "success":
			return nil
		default:
			if event.Error != nil {
				return event.Error
			}
			return fmt.Errorf("WhatsApp login ended: %s", event.Event)
		}
	}
	return errors.New("WhatsApp QR channel closed")
}

func sqliteURI(path string) string {
	return "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}

func parseOwnerJID() types.JID {
	raw := os.Getenv("OWNER_JID")
	if raw == "" {
		return types.JID{}
	}
	jid, err := types.ParseJID(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid OWNER_JID %q: %v\n", raw, err)
		return types.JID{}
	}
	return jid
}
