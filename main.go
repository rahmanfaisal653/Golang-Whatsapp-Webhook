package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"

	"github.com/local/whatsmeow-base/commands/owner"
	"github.com/local/whatsmeow-base/src/app"
	"github.com/local/whatsmeow-base/src/lib"
	"github.com/local/whatsmeow-base/src/web"
)

func main() {
	ctx := context.Background()
	client, cleanup := newClient(ctx)
	defer cleanup()

	application := app.New(client)

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

	// Clear the stale QR in the UI if WhatsApp logs this device out.
	client.AddEventHandler(func(evt any) {
		if _, ok := evt.(*events.LoggedOut); ok {
			application.SetQR("")
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
	<-stop
	client.Disconnect()
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
