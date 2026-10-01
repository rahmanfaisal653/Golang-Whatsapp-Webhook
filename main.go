package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"github.com/local/whatsmeow-base/commands/owner"
	"github.com/local/whatsmeow-base/src/features"
	"github.com/local/whatsmeow-base/src/lib"
	"github.com/local/whatsmeow-base/src/web"
)

func sqliteURI(path string) string {
	return "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessionDir := filepath.Join("src", "session")

	// 1. Initialize persistent features manager
	featuresPath := filepath.Join(sessionDir, "features.json")
	featMgr, err := features.NewManager(featuresPath)
	if err != nil {
		fatal("initialize feature manager", err)
	}

	// 2. Initialize WhatsApp manager
	mgr, err := web.NewWhatsAppManager(ctx, sessionDir)
	if err != nil {
		fatal("initialize WhatsApp manager", err)
	}
	defer mgr.Close()
	mgr.SetFeatureManager(featMgr)

	client := mgr.GetClient()

	if len(os.Args) > 1 && os.Args[1] == "--list-groups" {
		if err := mgr.StartConnect(); err != nil {
			fatal("connect", err)
		}
		defer client.Disconnect()
		if err := listGroups(ctx, client); err != nil {
			fatal("list groups", err)
		}
		return
	}

	// 3. Start unified Web UI, API, & Notifier Server
	_ = web.StartServer(ctx, mgr, featMgr, "")

	// 4. Wire up command handler with dynamic features
	ownerJID := parseOwnerJID()
	handler := lib.NewCommandHandler(ctx, client, ownerJID)
	handler.SetClientProvider(mgr.GetClient)
	handler.SetLogger(mgr.AddLog)
	handler.SetFeatureManager(featMgr)

	menuCmd := owner.MenuCommand{
		MenuProvider: featMgr.FormatMenuText,
	}

	for _, cmd := range []owner.Command{
		owner.PingCommand{},
		owner.AboutCommand{},
		menuCmd,
		owner.UptimeCommand{},
		owner.IDCommand{},
		owner.GroupsCommand{},
		owner.TestCommand{},
	} {
		handler.Register(cmd)
	}
	handler.LogSummary()
	mgr.SetExternalEventHandler(handler.Handle)

	// 5. Connect / Listen for QR code
	if err := mgr.StartConnect(); err != nil {
		fmt.Printf("[main] Initial connect error (waiting for Web UI or retry): %v\n", err)
	}

	fmt.Println("WhatsApp Webhook, Features Gateway & Web UI running. Press CTRL+C to stop.")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
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
