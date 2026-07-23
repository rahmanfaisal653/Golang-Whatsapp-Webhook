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
	_ "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"

	"github.com/local/whatsmeow-base/commands/owner"
	"github.com/local/whatsmeow-base/src/lib"
	"github.com/local/whatsmeow-base/src/notify"
)

func main() {
	ctx := context.Background()
	client, cleanup := newClient(ctx)
	defer cleanup()

	if len(os.Args) > 1 && os.Args[1] == "--list-groups" {
		if err := connect(ctx, client); err != nil {
			fatal("connect", err)
		}
		defer client.Disconnect()
		if err := listGroups(ctx, client); err != nil {
			fatal("list groups", err)
		}
		return
	}

	go notify.Start(ctx, client)

	ownerJID := parseOwnerJID()
	handler := lib.NewCommandHandler(ctx, client, ownerJID)
	for _, cmd := range []owner.Command{
		owner.PingCommand{},
		owner.AboutCommand{},
		owner.MenuCommand{},
		owner.UptimeCommand{},
		owner.IDCommand{},
		owner.GroupsCommand{},
		owner.TestCommand{},
	} {
		handler.Register(cmd)
	}
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

func newClient(ctx context.Context) (*whatsmeow.Client, func()) {
	if err := os.MkdirAll(filepath.Join("src", "session"), 0o700); err != nil {
		fatal("create data directory", err)
	}
	store, err := sqlstore.New(ctx, "sqlite", sqliteURI(filepath.Join("src", "session", "whatsmeow.db")), waLog.Stdout("Database", "INFO", true))
	if err != nil {
		fatal("open device store", err)
	}
	device, err := store.GetFirstDevice(ctx)
	if err != nil {
		store.Close()
		fatal("load device", err)
	}
	return whatsmeow.NewClient(device, waLog.Stdout("WhatsApp", "INFO", true)), func() { store.Close() }
}

func listGroups(ctx context.Context, client *whatsmeow.Client) error {
	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		return err
	}
	for _, group := range groups {
		fmt.Printf("%s\n%s\n\n", group.Name, group.JID)
	}
	return nil
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
