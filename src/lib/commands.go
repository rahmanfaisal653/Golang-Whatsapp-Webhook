package lib

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/local/whatsmeow-base/commands/owner"
)

const groupCacheTTL = 5 * time.Minute

type groupEntry struct {
	info    *types.GroupInfo
	expires time.Time
}

// CommandHandler routes incoming messages to registered commands.
type CommandHandler struct {
	ctx      context.Context
	client   *whatsmeow.Client
	ownerJID types.JID
	commands map[string]owner.Command

	cacheMu  sync.RWMutex
	groupMap map[string]groupEntry // group JID string → cached info
}

// NewCommandHandler creates a handler and wires it as the event callback.
func NewCommandHandler(ctx context.Context, client *whatsmeow.Client, ownerJID types.JID) *CommandHandler {
	return &CommandHandler{
		ctx:      ctx,
		client:   client,
		ownerJID: ownerJID,
		commands: make(map[string]owner.Command),
		groupMap: make(map[string]groupEntry),
	}
}

// Register adds a command to the handler and logs it to stdout.
func (h *CommandHandler) Register(cmd owner.Command) {
	h.commands[cmd.Name()] = cmd
	fmt.Printf("  %s loaded\n", cmd.Name())
}

// LogSummary prints the total number of registered commands.
func (h *CommandHandler) LogSummary() {
	fmt.Printf("\nCommands Loaded (%d total)\n", len(h.commands))
}

// IsOwner returns true if the message sender is the configured owner.
func (h *CommandHandler) IsOwner(msg *events.Message) bool {
	if h.ownerJID.IsEmpty() {
		return false
	}
	return SenderJID(msg.Info) == h.ownerJID
}

// IsAdmin returns true if the sender is an admin in the group chat.
// Non-group messages always return false.
func (h *CommandHandler) IsAdmin(msg *events.Message) bool {
	if !msg.Info.IsGroup {
		return false
	}

	chatStr := msg.Info.Chat.String()

	// Check cache first.
	h.cacheMu.RLock()
	entry, ok := h.groupMap[chatStr]
	h.cacheMu.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return participantIsAdmin(entry.info, SenderJID(msg.Info))
	}

	// Fetch fresh group info.
	info, err := h.client.GetGroupInfo(h.ctx, msg.Info.Chat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get group info %s: %v\n", chatStr, err)
		return false
	}

	h.cacheMu.Lock()
	h.groupMap[chatStr] = groupEntry{info: info, expires: time.Now().Add(groupCacheTTL)}
	h.cacheMu.Unlock()

	return participantIsAdmin(info, SenderJID(msg.Info))
}

func participantIsAdmin(info *types.GroupInfo, jid types.JID) bool {
	for _, p := range info.Participants {
		if p.JID.User == jid.User && (p.IsAdmin || p.IsSuperAdmin) {
			return true
		}
	}
	return false
}

// Handle is the event callback matching whatsmeow.EventHandler (func(evt interface{})).
func (h *CommandHandler) Handle(event any) {
	msg, ok := event.(*events.Message)
	if !ok || msg.Info.IsFromMe || msg.Message == nil {
		return
	}

	text := messageText(msg.Message)
	if text == "" {
		return
	}

	cmd, ok := h.commands[strings.TrimSpace(strings.ToLower(text))]
	if !ok {
		return
	}

	// Guard checks: owner bypasses all, then check individual gates.
	isOwner := h.IsOwner(msg)
	if cmd.OwnerOnly() && !isOwner {
		h.reply(msg, guardMsg(cmd.OwnerOnlyMsg(), "❌ This command is for the owner only."))
		return
	}
	if cmd.AdminOnly() && !isOwner && !h.IsAdmin(msg) {
		h.reply(msg, guardMsg(cmd.AdminOnlyMsg(), "❌ This command is for group admins only."))
		return
	}

	fmt.Printf("[%s] %s: %s\n", msg.Info.Chat, SenderJID(msg.Info), text)
	if err := cmd.Execute(h.ctx, h.client, msg); err != nil {
		fmt.Fprintf(os.Stderr, "command %s: %v\n", cmd.Name(), err)
	}
}

func (h *CommandHandler) reply(msg *events.Message, text string) {
	_, err := h.client.SendMessage(h.ctx, msg.Info.Chat, &waE2E.Message{Conversation: proto.String(text)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "reply: %v\n", err)
	}
}

func messageText(message *waE2E.Message) string {
	if text := message.GetConversation(); text != "" {
		return text
	}
	return message.GetExtendedTextMessage().GetText()
}

func guardMsg(custom, fallback string) string {
	if custom != "" {
		return custom
	}
	return fallback
}
