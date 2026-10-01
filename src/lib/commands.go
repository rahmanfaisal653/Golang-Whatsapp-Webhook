package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	"github.com/local/whatsmeow-base/src/features"
)

const groupCacheTTL = 5 * time.Minute

type groupEntry struct {
	info    *types.GroupInfo
	expires time.Time
}

// CommandHandler routes incoming messages to registered commands.
type CommandHandler struct {
	ctx            context.Context
	client         *whatsmeow.Client
	clientProvider func() *whatsmeow.Client
	logger         func(entryType, target, message, status, details string)
	ownerJID       types.JID
	commands       map[string]owner.Command
	featureMgr     *features.Manager

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

// SetClientProvider assigns a dynamic provider for the active whatsmeow client.
func (h *CommandHandler) SetClientProvider(provider func() *whatsmeow.Client) {
	h.clientProvider = provider
}

// SetLogger sets an optional activity logger callback (e.g. WhatsAppManager.AddLog).
func (h *CommandHandler) SetLogger(logger func(entryType, target, message, status, details string)) {
	h.logger = logger
}

// GetClient returns the current active whatsmeow client from provider if available, or static client.
func (h *CommandHandler) GetClient() *whatsmeow.Client {
	if h.clientProvider != nil {
		if c := h.clientProvider(); c != nil {
			return c
		}
	}
	return h.client
}

// SetFeatureManager assigns a dynamic feature manager to the command handler.
func (h *CommandHandler) SetFeatureManager(fm *features.Manager) {
	h.featureMgr = fm
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
	cli := h.GetClient()
	if cli == nil {
		return false
	}
	info, err := cli.GetGroupInfo(h.ctx, msg.Info.Chat)
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

	cleanText := strings.TrimSpace(text)
	lowerText := strings.ToLower(cleanText)

	// 1. Check Feature Manager global switches
	if h.featureMgr != nil {
		cfg := h.featureMgr.GetConfig()
		if !cfg.Settings.BotEnabled {
			return
		}
		if msg.Info.IsGroup && !cfg.Settings.GroupResponse {
			return
		}
		if !msg.Info.IsGroup && !cfg.Settings.PrivateResponse {
			return
		}
	}

	// 2. Check built-in commands
	if cmd, ok := h.commands[lowerText]; ok {
		if h.featureMgr != nil && !h.featureMgr.IsCommandEnabled(cmd.Name()) {
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

		cli := h.GetClient()
		if cli == nil {
			fmt.Fprintf(os.Stderr, "command %s: client is not ready\n", cmd.Name())
			return
		}

		fmt.Printf("[%s] %s: %s\n", msg.Info.Chat, SenderJID(msg.Info), text)
		if err := cmd.Execute(h.ctx, cli, msg); err != nil {
			fmt.Fprintf(os.Stderr, "command %s: %v\n", cmd.Name(), err)
			if h.logger != nil {
				h.logger("COMMAND", msg.Info.Chat.String(), cmd.Name(), "FAILED", err.Error())
			}
		} else {
			if h.logger != nil {
				h.logger("COMMAND", msg.Info.Chat.String(), cmd.Name(), "SUCCESS", "")
			}
		}
		return
	}

	// 3. Check custom auto-responders
	if h.featureMgr != nil {
		if matched := h.featureMgr.MatchCustom(cleanText, msg.Info.IsGroup); matched != nil {
			sender := SenderJID(msg.Info).User
			replyText := h.featureMgr.InterpolateResponse(matched.Response, sender, msg.Info.Chat.String())
			h.reply(msg, replyText)
			fmt.Printf("[AutoReply] Triggered '%s' for %s (%s)\n", matched.Trigger, sender, msg.Info.Chat)
			return
		}
	}

	// 4. Outgoing Webhook Event Forwarding
	if h.featureMgr != nil {
		cfg := h.featureMgr.GetConfig()
		if cfg.Settings.WebhookEnabled && cfg.Settings.OutgoingWebhookURL != "" {
			go h.forwardToWebhook(cfg.Settings.OutgoingWebhookURL, msg, text)
		}
	}
}

func (h *CommandHandler) forwardToWebhook(webhookURL string, msg *events.Message, text string) {
	sender := SenderJID(msg.Info)
	payload := map[string]interface{}{
		"event":      "message",
		"sender":     sender.User,
		"sender_jid": sender.String(),
		"chat":       msg.Info.Chat.String(),
		"is_group":   msg.Info.IsGroup,
		"push_name":  msg.Info.PushName,
		"message":    text,
		"timestamp":  time.Now().Unix(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WebhookForwarder] Error sending to %s: %v\n", webhookURL, err)
		if h.logger != nil {
			h.logger("WEBHOOK", webhookURL, text, "FAILED", err.Error())
		}
		return
	}
	defer resp.Body.Close()

	if h.logger != nil {
		h.logger("WEBHOOK", webhookURL, text, "SUCCESS", fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
}

func (h *CommandHandler) reply(msg *events.Message, text string) {
	cli := h.GetClient()
	if cli == nil {
		fmt.Fprintf(os.Stderr, "reply: client is not ready\n")
		return
	}
	_, err := cli.SendMessage(h.ctx, msg.Info.Chat, &waE2E.Message{Conversation: proto.String(text)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "reply: %v\n", err)
		if h.logger != nil {
			h.logger("BOT", msg.Info.Chat.String(), text, "FAILED", err.Error())
		}
	} else {
		if h.logger != nil {
			h.logger("BOT", msg.Info.Chat.String(), text, "SUCCESS", "")
		}
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
