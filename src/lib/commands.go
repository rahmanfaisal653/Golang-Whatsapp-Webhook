package lib

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/local/whatsmeow-base/commands/owner"
)

// CommandHandler routes incoming messages to registered commands.
type CommandHandler struct {
	ctx      context.Context
	client   *whatsmeow.Client
	commands map[string]owner.Command
}

// NewCommandHandler creates a handler and wires it as the event callback.
func NewCommandHandler(ctx context.Context, client *whatsmeow.Client) *CommandHandler {
	return &CommandHandler{
		ctx:      ctx,
		client:   client,
		commands: make(map[string]owner.Command),
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

// Handle is the event callback matching whatsmeow.EventHandler (func(evt interface{})).
func (h *CommandHandler) Handle(event any) {
	msg, ok := event.(*events.Message)
	if !ok || msg.Info.IsFromMe || msg.Message == nil {
		return
	}

	text := msg.Message.GetConversation()
	if text == "" {
		return
	}

	cmd, ok := h.commands[strings.TrimSpace(strings.ToLower(text))]
	if !ok {
		return
	}
	fmt.Printf("[%s] %s: %s\n", msg.Info.Chat, SenderJID(msg.Info), text)
	if err := cmd.Execute(h.ctx, h.client, msg); err != nil {
		fmt.Fprintf(os.Stderr, "command %s: %v\n", cmd.Name(), err)
	}
}

