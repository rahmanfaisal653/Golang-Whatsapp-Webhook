package owner

import (
	"context"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/local/whatsmeow-base/src/notify"
)

// TestCommand sends a "notify applied" message to the current chat to prove
// the send path works end to end.
type TestCommand struct{ publicCommand }

func (TestCommand) Name() string { return ".test" }

func (TestCommand) Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error {
	return notify.Send(ctx, client, notify.Request{
		To:      chatJID(msg.Info).String(),
		Message: "notify applied",
	})
}
