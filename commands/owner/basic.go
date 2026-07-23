package owner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var startedAt = time.Now()

type publicCommand struct{}

func (publicCommand) OwnerOnly() bool      { return false }
func (publicCommand) AdminOnly() bool      { return false }
func (publicCommand) OwnerOnlyMsg() string { return "" }
func (publicCommand) AdminOnlyMsg() string { return "" }

type AboutCommand struct{ publicCommand }
type MenuCommand struct{ publicCommand }
type UptimeCommand struct{ publicCommand }
type IDCommand struct{ publicCommand }

func (AboutCommand) Name() string  { return ".about" }
func (MenuCommand) Name() string   { return ".menu" }
func (UptimeCommand) Name() string { return ".uptime" }
func (IDCommand) Name() string     { return ".id" }

func (AboutCommand) Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error {
	return reply(ctx, client, msg, "GoWA\nWhatsApp webhook notification gateway.")
}

func (MenuCommand) Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error {
	return reply(ctx, client, msg, "Commands:\n.ping - check bot\n.about - bot info\n.uptime - bot runtime\n.id - show sender/chat JID\n.groups - list joined group JIDs\n.test - call localhost notifier")
}

func (UptimeCommand) Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error {
	return reply(ctx, client, msg, "Uptime: "+formatDuration(time.Since(startedAt)))
}

func (IDCommand) Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error {
	return reply(ctx, client, msg, fmt.Sprintf("Sender: %s\nChat: %s\nFromMe: %t\nIsGroup: %t", senderJID(msg.Info), chatJID(msg.Info), msg.Info.IsFromMe, msg.Info.IsGroup))
}

func reply(ctx context.Context, client *whatsmeow.Client, msg *events.Message, text string) error {
	_, err := client.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{Conversation: proto.String(text)})
	return err
}

func senderJID(info types.MessageInfo) types.JID {
	if info.Sender.Server == "lid" && !info.SenderAlt.IsEmpty() {
		return info.SenderAlt.ToNonAD()
	}
	return info.Sender.ToNonAD()
}

func chatJID(info types.MessageInfo) types.JID {
	if info.Chat.Server == "lid" && !info.RecipientAlt.IsEmpty() {
		return info.RecipientAlt.ToNonAD()
	}
	return info.Chat.ToNonAD()
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	parts := []string{}
	if h := int(d / time.Hour); h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
		d -= time.Duration(h) * time.Hour
	}
	if m := int(d / time.Minute); m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
		d -= time.Duration(m) * time.Minute
	}
	parts = append(parts, fmt.Sprintf("%ds", int(d/time.Second)))
	return strings.Join(parts, " ")
}
