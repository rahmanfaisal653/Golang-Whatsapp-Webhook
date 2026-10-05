package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	_ "modernc.org/sqlite"

	"github.com/local/whatsmeow-base/commands/owner"
	"github.com/local/whatsmeow-base/src/app"
	"github.com/local/whatsmeow-base/src/lib"
	"github.com/local/whatsmeow-base/src/web"
)

func main() {
	ctx := context.Background()
	logs := app.NewLogBuffer(300)
	client, cleanup := newClient(ctx, logs)
	defer cleanup()

	application := app.New(client, logs)

	if len(os.Args) > 1 && os.Args[1] == "--list-groups" {
		if err := application.Connect(ctx); err != nil {
			fatal("connect", err)
		}
		defer client.Disconnect()
		if err := listGroups(ctx, client); err != nil {
			fatal("list groups", err)
		}
		return
	}

	// If WhatsApp unlinks this device (from the phone or via the dashboard),
	// the current client is dead: whatsmeow deletes its store and refuses to
	// reconnect. Ask main to exit so the supervisor (PM2) restarts a fresh
	// process, which creates a new device and shows a new QR to pair again.
	client.AddEventHandler(func(evt any) {
		switch e := evt.(type) {
		case *events.LoggedOut:
			application.MarkLoggedOut()
		case *events.Connected:
			logs.Add("INFO", "App", "connected to WhatsApp")
		case *events.Disconnected:
			logs.Add("WARN", "App", "disconnected from WhatsApp — reconnecting")
		case *events.ConnectFailure:
			logs.Add("ERROR", "App", fmt.Sprintf("connect failed: %v", e.Reason))
		}
	})

	handler := lib.NewCommandHandler(ctx, client, parseOwnerJID())
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

	// Pair (first run) or reconnect (saved session) in the background so the
	// web server starts immediately and can show the QR when needed.
	go func() {
		if err := application.Connect(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		}
	}()
	go func() {
		if err := web.Start(ctx, application); err != nil {
			fmt.Fprintf(os.Stderr, "web: %v\n", err)
		}
	}()

	fmt.Println("Running. Admin dashboard: http://" + web.Addr() + "/admin  (CTRL+C to stop)")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-stop:
		client.Disconnect()
	case <-application.Restart():
		// Device was unlinked. Exit cleanly so PM2 restarts a fresh process
		// that can pair a new device (the current client can no longer connect).
		fmt.Println("WhatsApp device unlinked — restarting to allow re-pairing.")
		client.Disconnect()
		// Give the triggering HTTP response a moment to reach the browser
		// before the process exits.
		time.Sleep(time.Second)
	}
}

func newClient(ctx context.Context, logs *app.LogBuffer) (*whatsmeow.Client, func()) {
	if err := os.MkdirAll(filepath.Join("src", "session"), 0o700); err != nil {
		fatal("create data directory", err)
	}
	store, err := sqlstore.New(ctx, "sqlite", sqliteURI(filepath.Join("src", "session", "whatsmeow.db")), logs.Logger("Database", "INFO", true))
	if err != nil {
		fatal("open device store", err)
	}
	device, err := store.GetFirstDevice(ctx)
	if err != nil {
		store.Close()
		fatal("load device", err)
	}
	return whatsmeow.NewClient(device, logs.Logger("WhatsApp", "INFO", true)), func() { store.Close() }
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
